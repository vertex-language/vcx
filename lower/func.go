package lower

import (
	"fmt"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// fn tracks the lowering state of one function.
type fn struct {
	u   *unit
	sym *sema.FuncSymbol
	f   *ir.Func

	blk *ir.Block // where instructions are being appended, nil after a terminator

	this    ir.Ptr
	hasThis bool

	sret    ir.Ptr
	hasSRet bool

	resultInto ir.Ptr

	// entry holds frame slot allocations and jumps to the first body block.
	entry *ir.Block

	// slots maps a declaration to the frame slot holding it.
	slots map[sema.Symbol]ir.Ptr

	nblocks int

	// nstatics counts the function's block-scope statics, which number
	// their symbols.
	nstatics int

	// breakTo and continueTo target blocks for loop control flow.
	breakTo    *ir.Block
	continueTo *ir.Block

	// scopes tracks scopes with pending destructors; loopDepth tracks unwind depth.
	scopes    []*scope
	loopDepth int

	// temps are the current full-expression's temporaries (see temporary).
	temps []localObj

	// bitFields notes which addresses are bit-fields' storage units, for
	// load and store to shift and mask (see bitfield.go).
	bitFields map[ir.Ptr]bitField
}

func (u *unit) defineFunc(sym *sema.FuncSymbol, f *ir.Func) {
	if u.mod.Err() != nil {
		// The builder has already failed once and every call from here on
		// is a no-op that hands back placeholders; walking the body would
		return
	}
	fl := &fn{u: u, sym: sym, f: f, slots: map[sema.Symbol]ir.Ptr{}}
	fl.entry = f.Entry()
	fl.blk = f.Block("body")
	if p, has := u.srets[sym]; has {
		fl.sret, fl.hasSRet = p, true
	}

	// Spill incoming parameters to stack slots.
	args := u.params[sym]
	if sym.InClass != nil && !sym.Static && len(args) > 0 {
		// Keep `this` in register.
		if p, isPtr := args[0].(ir.Ptr); isPtr {
			fl.this, fl.hasThis = p, true
			// Adjust `this` from subobject to complete object if offset is non-zero.
			if off := u.thisOffset(sym); off != 0 {
				fl.this = fl.blk.Ptr.Add(p, fl.blk.I64.Const(-off))
			}
		}
		args = args[1:]
	}
	// Parameter scope encloses the function body, so parameters with destructors
	// are destroyed after locals. Under MSVC the callee destroys by-value class
	// parameters; under Itanium the caller destroys them after return.
	fl.pushScope()
	for i, p := range sym.Params {
		if i >= len(args) {
			break
		}
		byValueClass := !isReference(p.SymType) && classOf(p.SymType) != nil
		if p.SymName == "" {
			if u.model.ABI.CalleeDestroysParameters() && byValueClass {
				if addr, isPtr := args[i].(ir.Ptr); isPtr {
					fl.track(addr, p.SymType)
				}
			}
			continue // unnamed: passed, never read
		}
		fl.spillParam(p, args[i])
		if u.model.ABI.CalleeDestroysParameters() && byValueClass {
			fl.track(fl.slots[p], p.SymType)
		}
	}

	// Construct bases, install vptr, then run member initializers.
	if sym.Decl != nil && sym.InClass != nil && sym.SymName == sym.InClass.Name {
		fl.constructBases(sym)
		if fl.hasThis {
			fl.installVPtr(fl.this, sym.InClass)
		}
		fl.memInits(sym)
		if fl.hasThis {
			fl.memberDefaultsAfter(sym)
		}
	}

	fl.stmt(sym.Body)

	// Destroy members and bases in reverse order after a destructor's body.
	if sym.InClass != nil && sym.SymName == "~"+sym.InClass.Name {
		fl.destroyMembersAndBases(sym)
	}

	// The allocations are complete, so the entry block can be closed.
	fl.entry.Br(fl.f.Blocks()[1].To())

	// Implicit return at end of function or main.
	if fl.blk != nil {
		fl.returnDefault()
	}
	fl.scopes = fl.scopes[:len(fl.scopes)-1]
	u.structorVariant(sym, f)
}

func (fl *fn) spillParam(p *sema.VarSymbol, arg ir.Value) {
	// Class parameter already has caller-allocated storage.
	if !isReference(p.SymType) && classOf(p.SymType) != nil {
		if addr, isPtr := arg.(ir.Ptr); isPtr {
			fl.slots[p] = addr
			return
		}
	}
	slot := fl.alloc(p.SymType, p.SymName+"_addr")
	fl.slots[p] = slot
	fl.store(slot, arg, p.SymType)
}

// returnDefault ends a block that ran out of statements.
func (fl *fn) returnDefault() {
	fl.destroyFrom(0)
	if fl.u.ctorReturnsThis(fl.sym) {
		fl.blk.Return(fl.this)
		fl.blk = nil
		return
	}
	ret := fl.sym.FuncType.Ret
	if ret == nil || types.IsVoid(types.Unqualify(ret)) || fl.hasSRet {
		fl.blk.Return()
		fl.blk = nil
		return
	}
	// Implicit return 0 for main, or zero-value return.
	fl.blk.Return(fl.zeroOf(ret))
	fl.blk = nil
}

func (fl *fn) zeroOf(t types.Type) ir.Value {
	switch fl.u.regType(t) {
	case ir.TypeI64:
		return fl.blk.I64.Const(0)
	case ir.TypeF32:
		return fl.blk.F32.Const(0)
	case ir.TypeF64:
		return fl.blk.F64.Const(0)
	case ir.TypePtr:
		return fl.blk.Ptr.Const()
	default:
		return fl.blk.I32.Const(0)
	}
}

// constructBases runs base subobject constructors in declaration order.
func (fl *fn) constructBases(sym *sema.FuncSymbol) {
	named := map[string]*ast.MemInit{}
	for _, mi := range sym.Decl.Inits {
		if mi.Name != nil {
			named[sema.NameString(mi.Name, fl.u.unit)] = mi
		}
	}
	baseOffs := make([]int64, len(sym.InClass.Bases))
	fl.u.model.LayoutWithBases(sym.InClass, make([]int64, len(sym.InClass.Fields)), baseOffs)
	for i, b := range sym.InClass.Bases {
		br, isRec := types.Unqualify(b.Type).(*types.Record)
		if !isRec || b.Virtual {
			continue
		}
		var args []ast.Expr
		if mi, ok := named[br.Name]; ok {
			args = mi.Args
		}
		fl.constructBase(br, baseOffs[i], args)
	}
}

// memInits runs the member part of a constructor's mem-initializer-list.
func (fl *fn) memInits(sym *sema.FuncSymbol) {
	for _, mi := range sym.Decl.Inits {
		if mi.Name == nil {
			continue
		}
		name := sema.NameString(mi.Name, fl.u.unit)
		off, t, ok := fl.u.fieldOffset(sym.InClass, name)
		if !ok {
			// Not a member, so a base -- already handled above.
			continue
		}
		var initExpr ast.Expr
		switch {
		case len(mi.Args) == 1:
			initExpr = mi.Args[0]
		case mi.Braced != nil && len(mi.Braced.Items) == 1:
			initExpr, _ = mi.Braced.Items[0].(ast.Expr)
		}
		dst := fl.this
		if off != 0 {
			dst = fl.blk.Ptr.Add(dst, fl.blk.I64.Const(off))
		}
		if isReference(t) {
			// A reference member binds to the initializer address.
			if initExpr != nil {
				if addr, ok := fl.bind(initExpr, t); ok {
					fl.blk.Ptr.Store(addr, dst)
				}
			}
			continue
		}
		if rec := classOf(t); rec != nil {
			// A class member: built in place from a prvalue, copied
			// from an lvalue, by the member's own constructor where the
			// list names one.
			if ctor := fl.u.res.Info.MemInits[mi]; ctor != nil {
				fl.constructWith(dst, ctor, mi.Args, mi.Pos())
			} else if initExpr != nil {
				fl.exprInto(dst, initExpr, rec)
			} else if mi.Braced != nil {
				fl.initList(dst, t, mi.Braced)
			}
			continue
		}
		if _, isArr := types.Unqualify(t).(*types.Array); isArr && mi.Braced != nil {
			// An array member from a braced list.
			fl.initList(dst, t, mi.Braced)
			continue
		}
		if initExpr == nil {
			continue
		}
		v := fl.expr(initExpr)
		if v == nil {
			return
		}
		fl.store(dst, fl.convert(v, fl.typeOf(initExpr), t), t)
	}
}

// constructBase runs a base subobject's constructor on its part of this.
//
// The base with no user constructor has a trivial one and nothing runs;
// its table pointer, if it has one, is installed by the derived class's own
// constructor overwriting it a moment later, so nothing is needed here
// either. A base with a user constructor is called on `this + offset`, with
// the arguments the mem-initializer supplied or none.
func (fl *fn) constructBase(base *types.Record, off int64, argExprs []ast.Expr) {
	obj := fl.this
	if off != 0 {
		obj = fl.blk.Ptr.Add(obj, fl.blk.I64.Const(off))
	}
	target := fl.baseConstructor(base, argExprs)
	if target == nil {
		if len(argExprs) == 0 {
			// No constructor of its own: use the implicit one.
			fl.defaultConstruct(obj, base, fl.sym.SymPos)
		}
		return
	}
	args := []ir.Value{obj}
	for _, a := range argExprs {
		v := fl.expr(a)
		if v == nil {
			return
		}
		args = append(args, v)
	}
	fl.blk.Call(target, args...)
}

// alloc reserves a frame slot in the entry block.
func (fl *fn) alloc(t types.Type, name string) ir.Ptr {
	size, align := fl.u.sizeAlign(t)
	return fl.entry.Ptr.Alloc(uint64(size), uint64(align)).Named(name)
}

// block starts a fresh block and makes it current.
func (fl *fn) block(name string) *ir.Block {
	fl.nblocks++
	return fl.f.Block(fmt.Sprintf("%s_%d", name, fl.nblocks))
}

// sizeAlign is a type's storage size and alignment, with a floor of one byte
// so a slot is never zero-sized.
func (u *unit) sizeAlign(t types.Type) (int64, int64) {
	size, ok := u.model.Sizeof(t)
	if !ok || size <= 0 {
		size = 1
	}
	align, ok := u.model.Alignof(t)
	if !ok || align <= 0 {
		align = 1
	}
	return size, align
}

// store writes a value into a slot at the type's memory width.
func (fl *fn) store(dst ir.Ptr, v ir.Value, t types.Type) {
	if bf, isBit := fl.bitFields[dst]; isBit {
		fl.bitFieldStore(dst, bf, v)
		return
	}
	b := fl.blk
	switch val := v.(type) {
	case ir.I32:
		switch fl.storeBytes(t) {
		case 1:
			b.I32.Store8(val, dst)
		case 2:
			b.I32.Store16(val, dst)
		default:
			b.I32.Store(val, dst)
		}
	case ir.I64:
		b.I64.Store(val, dst)
	case ir.F32:
		b.F32.Store(val, dst)
	case ir.F64:
		b.F64.Store(val, dst)
	case ir.Ptr:
		b.Ptr.Store(val, dst)
	case ir.I1:
		b.I32.Store8(b.I32.ZExtI1(val), dst)
	}
}

// load reads a slot value, promoting narrow integers as needed.
func (fl *fn) load(src ir.Ptr, t types.Type) ir.Value {
	if bf, isBit := fl.bitFields[src]; isBit {
		return fl.bitFieldLoad(src, bf, t)
	}
	if classOf(t) != nil || types.IsFunc(t) || types.IsArray(t) {
		return src
	}
	b := fl.blk
	switch fl.u.regType(t) {
	case ir.TypeI64:
		return b.I64.Load(src)
	case ir.TypeF32:
		return b.F32.Load(src)
	case ir.TypeF64:
		return b.F64.Load(src)
	case ir.TypePtr:
		return b.Ptr.Load(src)
	default:
		switch fl.storeBytes(t) {
		case 1:
			if isSigned(t) {
				return b.I32.SLoad8(src)
			}
			return b.I32.ULoad8(src)
		case 2:
			if isSigned(t) {
				return b.I32.SLoad16(src)
			}
			return b.I32.ULoad16(src)
		default:
			return b.I32.Load(src)
		}
	}
}

// storeBytes is how wide a scalar is in memory.
func (fl *fn) storeBytes(t types.Type) int64 {
	size, ok := fl.u.model.Sizeof(t)
	if !ok {
		return 4
	}
	return size
}

// typeOf returns the type recorded by sema for an expression.
func (fl *fn) typeOf(e ast.Expr) types.Type {
	if fl.u.res.Info == nil {
		return nil
	}
	return fl.u.res.Info.Types[e]
}

// baseConstructor resolves the constructor for a base subobject.
func (fl *fn) baseConstructor(base *types.Record, argExprs []ast.Expr) ir.Callee {
	for _, m := range base.Methods {
		if m.Name != base.Name || m.Defaulted || len(m.Func.Params) != len(argExprs) {
			continue
		}
		return fl.u.method(base, m.Name, m.Func)
	}
	return nil
}
