// Package lower emits a translation unit as a VIR module.
package lower

import (
	"fmt"
	"math/big"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/constexpr"
	"github.com/vertex-language/vcx/mangle"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// Options holds configuration for lowering a translation unit.
type Options struct {
	// Name is the module name (stem of primary source file or "a").
	Name string

	// Target supplies the module layout and data layout.
	Target ir.Target

	// Model is the target data model used during semantic analysis.
	Model types.Model

	// SymbolPrefix is prepended to symbol names (e.g. "_" on Mach-O).
	SymbolPrefix string

	// ABI is the name-mangling scheme (MSVC or Itanium).
	ABI mangle.ABI
}

// Diagnostic is one complaint from lowering.
type Diagnostic struct {
	Severity token.Severity
	Pos      ast.Tok
	Message  string
}

// Lower lowers an analyzed AST and its semantic info to an IR module.
func Lower(file *ast.File, res *sema.Result, opt Options) (*ir.Module, []Diagnostic) {
	u := &unit{
		file:  file,
		res:   res,
		opt:   opt,
		unit:  file.Unit,
		model: opt.Model,
	}
	if u.opt.Name == "" {
		u.opt.Name = "a"
	}
	u.mod = ir.NewModule(u.opt.Name, opt.Target)

	u.declare()
	u.define()

	if err := u.mod.Err(); err != nil {
		u.errorf(ast.NoTok, "internal: the IR builder rejected this unit: %v", err)
	}
	return u.mod, u.diags
}

// unit is one translation unit's lowering state.
type unit struct {
	file  *ast.File
	res   *sema.Result
	opt   Options
	unit  ast.Unit
	model types.Model

	mod   *ir.Module
	diags []Diagnostic

	// funcs maps declared functions to their IR functions.
	funcs map[*sema.FuncSymbol]*ir.Func

	// globals maps namespace-scope variables to their IR globals, or to
	// the imports that stand for ones another unit defines.
	globals map[*sema.VarSymbol]ir.Symbol

	// pendingFuncs tracks inline functions awaiting body generation.
	pendingFuncs []*sema.FuncSymbol

	// imports tracks imported functions by symbol and by name.
	imports       map[*sema.FuncSymbol]ir.Callee
	importsByName map[string]ir.Callee

	// vtables maps polymorphic records to their vtables; thunks maps adjustor thunks.
	vtables map[*types.Record][]vtable
	thunks  map[thunkKey]ir.Callee

	// purecallFn is the runtime's _purecall, once imported.
	purecallFn ir.Callee

	// typeInfos are the Itanium type_info objects emitted so far, and
	// cxxabiTables the runtime's tables their pointers aim into.
	typeInfos    map[*types.Record]ir.Symbol
	cxxabiTables map[string]ir.Symbol

	// invokers maps a closure to the function it converts to.
	invokers map[*types.Record]ir.Callee

	// implicitCopies maps a class to its synthesized copy constructor and
	// copy assignment (see implicitCopy).
	implicitCopies map[copyKey]ir.Callee

	// implicitDtors maps a class with no destructor of its own to the one
	// synthesized for it (see implicitDestructor).
	implicitDtors map[*types.Record]*sema.FuncSymbol

	// deletingDtors maps a class to its vector deleting destructor;
	// deletingDtorSig is the signature an indirect call to one carries.
	deletingDtors   map[*types.Record]ir.Callee
	deletingDtorSig *ir.Type

	// recordTypes caches struct types for class records; srets tracks hidden return pointers.
	recordTypes map[*types.Record]*ir.Type
	ntypes      int
	srets       map[*sema.FuncSymbol]ir.Ptr

	// dynamicInits tracks global variables requiring dynamic initialization.
	dynamicInits []dynamicInit
	declsOf      map[sema.Symbol]*ast.InitDeclarator

	// strings caches string literal globals; opNew/opDelete track allocation functions.
	strings       map[string]*ir.Global
	nstrings      int
	opNew         ir.Callee
	opDelete      ir.Callee
	opNewArray    ir.Callee
	opDeleteArray ir.Callee

	// Aligned allocation forms taking std::align_val_t for over-aligned types.
	opNewAligned         ir.Callee
	opDeleteAligned      ir.Callee
	opNewArrayAligned    ir.Callee
	opDeleteArrayAligned ir.Callee
	funcTypes            map[*sema.FuncSymbol]*ir.Type
	nsigs                int

	// params caches lowered parameter values for each function.
	params map[*sema.FuncSymbol][]ir.Value
}

func (u *unit) errorf(pos ast.Tok, format string, args ...any) {
	u.diags = append(u.diags, Diagnostic{
		Severity: token.Error,
		Pos:      pos,
		Message:  fmt.Sprintf(format, args...),
	})
}

// symbolName is what a mangled name becomes in the object file: the
// same, with the container's prefix.
func (u *unit) symbolName(name string) string {
	return u.opt.SymbolPrefix + name
}

// linkSymbol is fn's symbol in the object file: an __asm label as written,
// or the mangled name with the container's prefix.
func (u *unit) linkSymbol(fn *sema.FuncSymbol) string {
	if fn.AsmLabel != "" {
		return fn.AsmLabel
	}
	return u.symbolName(u.funcSymbol(fn))
}

// funcSymbol returns the mangled object-file symbol name for fn.
func (u *unit) funcSymbol(fn *sema.FuncSymbol) string {
	name, err := mangle.FunctionName(u.opt.ABI, mangle.Describe(fn))
	if err != nil {
		u.errorf(fn.SymPos, "%v", err)
		return "_unnameable_" + fn.SymName
	}
	return name
}

// funcTypeKey returns a unique identifier for fn's signature type.
func (u *unit) funcTypeKey(fn *sema.FuncSymbol) string {
	u.nsigs++
	return fmt.Sprintf("sig_%s_%d", identOf(fn.SymName), u.nsigs)
}

// identOf strips a name to what a VIR identifier admits.
func identOf(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '_', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9' && i > 0:
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return "fn"
	}
	return string(out)
}

// declare gives every function a symbol before any body is walked, so that a
// call can name a function defined later in the file -- or not defined here
// at all.
func (u *unit) declare() {
	u.funcs = map[*sema.FuncSymbol]*ir.Func{}
	u.params = map[*sema.FuncSymbol][]ir.Value{}
	u.globals = map[*sema.VarSymbol]ir.Symbol{}
	u.funcTypes = map[*sema.FuncSymbol]*ir.Type{}
	u.imports = map[*sema.FuncSymbol]ir.Callee{}
	u.importsByName = map[string]ir.Callee{}
	u.strings = map[string]*ir.Global{}
	u.recordTypes = map[*types.Record]*ir.Type{}
	u.srets = map[*sema.FuncSymbol]ir.Ptr{}

	u.declareGlobals()
	for _, fn := range u.res.Functions {
		if fn.Body == nil {
			continue
		}
		// Inline functions are emitted only when odr-used.
		if fn.Inline {
			continue
		}
		u.funcs[fn] = u.declareFunc(fn)
	}
}

// callee is the symbol a call to fn reaches: the definition when this unit
// holds one, and otherwise an import with the same signature -- a function
// declared here and defined in another object, which the linker resolves.
//
// The analysis hands lowering two kinds of symbol for one function: the
// declared one, and one it made on the spot from a class's method list
// while resolving a constructor or a member call. The second is matched to
// the first by class, name and signature, so that every path to a function
// ends at the one symbol the object file has for it.
func (u *unit) callee(fn *sema.FuncSymbol) ir.Callee {
	if fn == nil {
		return nil
	}
	if f, defined := u.funcs[fn]; defined {
		return f
	}
	if imp, done := u.imports[fn]; done {
		return imp
	}
	if fn.Defaulted && (fn.SymName == "operator==" || fn.SymName == "operator<=>") && (fn.InClass != nil || defaultedFriendClass(fn) != nil) {
		// Defaulted comparison operators get compiler-generated bodies.
		return u.defaultedComparison(fn)
	}
	if fn.Body != nil {
		// An inline function reached for the first time: defined in this
		// object after all, once the function that reached it is done.
		// It is looked at before the declared-symbol match below, since
		// a member template's instance matches its template by
		// signature and is not it.
		f := u.declareFunc(fn)
		u.funcs[fn] = f
		u.pendingFuncs = append(u.pendingFuncs, fn)
		return f
	}
	if declared := u.declaredFor(fn); declared != nil && declared != fn {
		c := u.callee(declared)
		u.imports[fn] = c
		return c
	}
	name := u.linkSymbol(fn)
	if existing, dup := u.importsByName[name]; dup {
		u.imports[fn] = existing
		return existing
	}
	imp := u.mod.ImportFunc(name, u.signature(fn))
	u.imports[fn] = imp
	u.importsByName[name] = imp
	return imp
}

// declaredFor finds the unit's own symbol for a function the analysis
// described by class, name and signature.
func (u *unit) declaredFor(fn *sema.FuncSymbol) *sema.FuncSymbol {
	for _, d := range u.res.Declared {
		if d.InClass == fn.InClass && d.SymName == fn.SymName && types.SameSignature(d.FuncType, fn.FuncType) {
			return d
		}
	}
	for _, d := range u.res.Functions {
		if d.InClass == fn.InClass && d.SymName == fn.SymName && types.SameSignature(d.FuncType, fn.FuncType) {
			return d
		}
	}
	return nil
}

// method is the unit's symbol for a member of a class by name and
// signature, or nil where the class has no such member.
func (u *unit) method(rec *types.Record, name string, sig *types.Func) ir.Callee {
	probe := &sema.FuncSymbol{SymName: name, FuncType: sig, InClass: rec}
	declared := u.declaredFor(probe)
	if declared == nil {
		return nil
	}
	return u.callee(declared)
}

// signature is a function's IR signature: the shape declareFunc gives a
// definition, for an import or an indirect call.
func (u *unit) signature(fn *sema.FuncSymbol) *ir.Sig {
	sig := ir.NewSig()
	retRec := classOf(fn.FuncType.Ret)
	hiddenAfterThis := retRec != nil && !u.plainForReturn(retRec) && u.model.ABI.ResultAfterThis() && fn.InClass != nil && !fn.Static
	if retRec != nil && !hiddenAfterThis {
		if u.plainForReturn(retRec) {
			sig = sig.Param(ir.TypePtr, ir.SRet(u.recordType(retRec)))
		} else {
			sig = sig.Param(ir.TypePtr)
		}
	}
	if fn.InClass != nil && !fn.Static {
		sig = sig.Param(ir.TypePtr)
	}
	if hiddenAfterThis {
		sig = sig.Param(ir.TypePtr)
	}
	for _, p := range fn.FuncType.Params {
		if rec := classOf(p.Type); rec != nil && u.plainForCalls(rec) {
			sig = sig.Param(ir.TypePtr, ir.ByVal(u.recordType(rec)))
			continue
		}
		sig = sig.Param(u.regTypeOrPtr(p.Type))
	}
	if ret := fn.FuncType.Ret; retRec == nil && ret != nil && !types.IsVoid(types.Unqualify(ret)) {
		sig = sig.Ret(u.regTypeOrPtr(ret))
	}
	if u.ctorReturnsThis(fn) {
		sig = sig.Ret(ir.TypePtr)
	}
	if fn.FuncType.Variadic {
		sig = sig.Variadic()
	}
	return sig
}

// ctorReturnsThis is the Microsoft convention that a constructor returns
// the object it constructed, in RAX: cl's `new T(args)` takes the pointer
// from the constructor's return, not from the allocation it made, so a
// constructor of vcx's that a cl caller reaches -- a COMDAT copy of an
// inline one that the linker chose -- has to hand it back. Generic Itanium
// constructors return nothing; ARM's, and so Apple arm64's, return the
// object from destructors as well (see types.CXXABI).
func (u *unit) ctorReturnsThis(fn *sema.FuncSymbol) bool {
	if fn.InClass == nil || fn.Static {
		return false
	}
	if fn.SymName == fn.InClass.Name {
		return u.model.ABI.ConstructorsReturnThis()
	}
	return fn.SymName == "~"+fn.InClass.Name && u.model.ABI.DestructorsReturnThis()
}

func (u *unit) declareFunc(fn *sema.FuncSymbol) *ir.Func {
	f := u.mod.Func(u.linkSymbol(fn)).Export()

	// Inline functions use COMDAT linkage to allow duplicate definitions across units.
	if fn.Inline {
		f.Comdat()
	}
	if fn.InClass != nil && isClosure(fn.InClass) {
		// Closure types have internal linkage.
		f.Internal()
	}

	// A class returned by value comes back through storage the caller
	// supplies. Where the class is plain the parameter says `sret` and the
	// backend decides whether that means a register or a hidden pointer;
	// where it is not, the pointer is an ordinary parameter and the callee
	// constructs into it. The two ABIs put it in different places: first
	// under Itanium, but after `this` under Microsoft, where RCX is always
	// the object and the result pointer takes RDX.
	retRec := classOf(fn.FuncType.Ret)
	hiddenAfterThis := retRec != nil && !u.plainForReturn(retRec) && u.model.ABI.ResultAfterThis() && fn.InClass != nil && !fn.Static
	if retRec != nil && !hiddenAfterThis {
		u.srets[fn] = u.declareSRet(f, retRec)
	}

	// Non-static member functions take this as their first parameter.
	if fn.InClass != nil && !fn.Static {
		u.params[fn] = append(u.params[fn], f.ParamPtr("this"))
	}
	if hiddenAfterThis {
		u.srets[fn] = f.ParamPtr("__ret")
	}

	for _, p := range fn.Params {
		u.params[fn] = append(u.params[fn], u.declareParam(f, p))
	}
	if retRec == nil {
		u.declareResult(f, fn.FuncType.Ret)
	}
	if u.ctorReturnsThis(fn) {
		f.ReturnsPtr()
	}
	return f
}

// declareSRet is the hidden result parameter for a class returned by
// value: `sret` for a plain class, a bare pointer otherwise.
func (u *unit) declareSRet(f *ir.Func, rec *types.Record) ir.Ptr {
	if u.plainForReturn(rec) {
		return f.ParamPtr("__ret", ir.SRet(u.recordType(rec)))
	}
	return f.ParamPtr("__ret")
}

func (u *unit) declareParam(f *ir.Func, p *sema.VarSymbol) ir.Value {
	name := p.SymName
	if name == "" {
		name = "unnamed"
	}
	// A class parameter is a copy: the ABI's own for a plain class, which
	// `byval` asks the backend for, and one the caller constructed for
	// any other, which arrives as a pointer.
	if rec := classOf(p.SymType); rec != nil {
		if u.plainForCalls(rec) {
			return f.ParamPtr(name, ir.ByVal(u.recordType(rec)))
		}
		return f.ParamPtr(name)
	}
	switch u.regType(p.SymType) {
	case ir.TypeI1:
		return f.ParamI1(name)
	case ir.TypeI64:
		return f.ParamI64(name)
	case ir.TypeF32:
		return f.ParamF32(name)
	case ir.TypeF64:
		return f.ParamF64(name)
	case ir.TypePtr:
		return f.ParamPtr(name)
	default:
		return f.ParamI32(name)
	}
}

func (u *unit) declareResult(f *ir.Func, ret types.Type) {
	if ret == nil || types.IsVoid(types.Unqualify(ret)) {
		return
	}
	switch u.regType(ret) {
	case ir.TypeI1:
		f.ReturnsI1()
	case ir.TypeI32:
		f.ReturnsI32()
	case ir.TypeI64:
		f.ReturnsI64()
	case ir.TypeF32:
		f.ReturnsF32()
	case ir.TypeF64:
		f.ReturnsF64()
	default:
		f.ReturnsPtr()
	}
}

// define walks each function body.
func (u *unit) define() {
	// The tables come after every function has a symbol -- their entries
	// point at those symbols -- and before any body is walked, because a
	// constructor body stores a table's address.
	u.declareVTables()

	for _, fn := range u.res.Functions {
		f, ok := u.funcs[fn]
		if !ok || fn.Inline {
			continue
		}
		u.defineFunc(fn, f)
	}
	u.defineInitializer()
	// The inline functions the bodies above reached, and the ones those
	// reach in turn, until nothing new is reached.
	for len(u.pendingFuncs) > 0 {
		fn := u.pendingFuncs[0]
		u.pendingFuncs = u.pendingFuncs[1:]
		u.defineFunc(fn, u.funcs[fn])
	}
}

// declareGlobals gives every namespace-scope object a symbol.
//
// Globals with static storage duration are zero-initialized before code runs.
// Constant initializers provide initial global values.
func (u *unit) declareGlobals() {
	if u.res.GlobalScope == nil {
		return
	}
	u.declareGlobalsIn(u.res.GlobalScope, map[*sema.Scope]bool{})
	for _, v := range sema.StaticMembers(u.res) {
		u.globals[v] = u.declareGlobal(v)
	}
}

// declareGlobalsIn declares the objects of a namespace and, through the
// namespace symbols it holds, of every namespace inside it: `std::_FNV_prime`
// is as much an object as one at the top.
func (u *unit) declareGlobalsIn(scope *sema.Scope, seen map[*sema.Scope]bool) {
	if scope == nil || seen[scope] {
		return
	}
	seen[scope] = true
	for _, syms := range scope.Symbols {
		for _, sym := range syms {
			switch s := sym.(type) {
			case *sema.NamespaceSymbol:
				u.declareGlobalsIn(s.InnerScope, seen)
			case *sema.VarSymbol:
				if s.SymName == "" {
					continue
				}
				// A variable template is not an object; its instances
				// are, and those are folded to their values where they
				// are used (see templateId), so none is emitted yet.
				if s.Template != nil {
					continue
				}
				if !s.Defined {
					// External declarations without definitions are imported on use.
					continue
				}
				u.globals[s] = u.declareGlobal(s)
			}
		}
	}
	for _, ns := range scope.UsingNamespaces {
		u.declareGlobalsIn(ns, seen)
	}
}

// globalFor is the symbol for an object with static storage duration: the
// one declareGlobals made, or an import made now for one declared here and
// defined elsewhere.
func (u *unit) globalFor(v *sema.VarSymbol) (ir.Symbol, bool) {
	if g, known := u.globals[v]; known {
		return g, true
	}
	// Only an extern object or a static data member can be defined in
	// another unit; anything else undefined here is not an object at all.
	if v.Defined || v.Template != nil || v.Storage != sema.StorageExtern && v.InClass == nil {
		return nil, false
	}
	g := u.importGlobal(v)
	u.globals[v] = g
	return g, true
}

// importGlobal is the reference to an object declared here and defined
// elsewhere.
func (u *unit) importGlobal(v *sema.VarSymbol) ir.Symbol {
	name, err := mangle.VariableName(u.opt.ABI, mangle.DescribeVariable(v, nil))
	if err != nil {
		u.errorf(v.SymPos, "%v", err)
		name = "_unnameable_" + v.SymName
	}
	return u.mod.ImportGlobal(u.symbolName(name), u.storageType(v.SymType))
}

func (u *unit) declareGlobal(v *sema.VarSymbol) *ir.Global {
	_, align := u.sizeAlign(v.SymType)
	name, err := mangle.VariableName(u.opt.ABI, mangle.DescribeVariable(v, nil))
	if err != nil {
		u.errorf(v.SymPos, "%v", err)
		name = "_unnameable_" + v.SymName
	}
	g := u.mod.Global(u.symbolName(name), ir.RW, u.storageType(v.SymType))
	if v.Inline {
		// Inline variables use COMDAT linkage.
		g.Comdat()
	}
	if sema.HasExternalLinkage(v) {
		g.Export()
	} else {
		// Internal linkage: unit-local, invisible to linker.
		g.Internal()
	}
	g.Align(uint64(align))

	if v.Init != nil && classOf(v.SymType) == nil {
		if n, err := u.evalInt(v.Init); err == nil {
			g.Init(ir.Lit(ir.Int(n)))
		}
	}
	u.noteDynamicInit(v, g)
	return g
}

// ftype is the storage shape a global declares.
//
// A scalar names its own store-type; anything else is a blob of the right
// size, which is all a container needs to reserve the space. Naming the
// record's real shape arrives with the field access that needs it.
func (u *unit) ftype(t types.Type, size int64) ir.FType {
	switch u.regType(t) {
	case ir.TypeI64:
		return ir.StoreI64.FType()
	case ir.TypeF32:
		return ir.StoreF32.FType()
	case ir.TypeF64:
		return ir.StoreF64.FType()
	case ir.TypePtr:
		return ir.StorePtr.FType()
	}
	switch size {
	case 1:
		return ir.StoreI8.FType()
	case 2:
		return ir.StoreI16.FType()
	case 8:
		return ir.StoreI64.FType()
	default:
		return ir.StoreI32.FType()
	}
}

// evalInt asks the constant evaluator for a literal's value.
func (u *unit) evalInt(e ast.Expr) (int64, error) {
	if u.res.Info != nil {
		if n, ok := u.res.Info.Consts[e]; ok {
			return n, nil
		}
	}
	ctx := constexpr.NewContext(u.unit, u.model)
	return ctx.EvalInt(e)
}

// evalFloat asks the constant evaluator for a floating literal's value.
func (u *unit) evalFloat(e ast.Expr) (float64, error) {
	ctx := constexpr.NewContext(u.unit, u.model)
	v, err := ctx.Eval(e)
	if err != nil {
		return 0, err
	}
	switch n := v.(type) {
	case constexpr.FloatValue:
		return n.Val, nil
	case constexpr.IntValue:
		// Convert integer literal value to float.
		f, _ := new(big.Float).SetInt(n.Val).Float64()
		return f, nil
	}
	return 0, fmt.Errorf("the literal is not a number")
}

// fieldOffset is where one member sits in its class, and what type it is.
//
// It walks the bases first, because an inherited member is reached at the
// base subobject's offset plus its own -- and because the same walk is what
// makes a name declared in a base findable at all.
func (u *unit) fieldOffset(r *types.Record, name string) (int64, types.Type, bool) {
	if r == nil {
		return 0, nil, false
	}

	baseOffs := make([]int64, len(r.Bases))
	offs := make([]int64, len(r.Fields))
	u.model.LayoutWithBases(r, offs, baseOffs)

	for i, b := range r.Bases {
		br, isRec := types.Unqualify(b.Type).(*types.Record)
		if !isRec || b.Virtual {
			continue
		}
		if off, t, ok := u.fieldOffset(br, name); ok {
			return baseOffs[i] + off, t, true
		}
	}

	for i, f := range r.Fields {
		if f.Name == name {
			return offs[i], f.Type, true
		}
		// An anonymous union's members sit inside its unnamed field.
		if f.Name == "" {
			if inner := types.AsRecord(types.Unqualify(f.Type)); inner != nil {
				if off, t, ok := u.fieldOffset(inner, name); ok {
					return offs[i] + off, t, true
				}
			}
		}
	}
	return 0, nil, false
}

// thisOffset is where a member function expects `this` to point, relative
// to the object: zero for everything but an override of a virtual that a
// secondary base introduced (types.ThisOffset).
func (u *unit) thisOffset(fn *sema.FuncSymbol) int64 {
	if fn.InClass == nil || !fn.Virtual {
		return 0
	}
	return u.model.ThisOffset(fn.InClass, fn.SymName, fn.FuncType)
}

// typeOfTypeId is the type a type-id names, as the analysis recorded it.
func (u *unit) typeOfTypeId(id *ast.TypeId) types.Type {
	if id == nil || u.res.Info == nil {
		return nil
	}
	return u.res.Info.TypeIds[id]
}
