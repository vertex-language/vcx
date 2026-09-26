package lower

import (
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/literal"
	"github.com/vertex-language/vcx/objcrt"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// Objective-C++'s lowering: what a message, a class reference, an
// instance variable and the @-statements become. The metadata the runtime
// reads -- classes, categories, protocols, the lists it walks -- is in
// objcmeta.go. The rules (symbol names, sections, layouts, encodings) are
// objcrt's; this file only asks it.
//
// A send is an ordinary indirect call. objc_msgSend forwards whatever it
// was handed, so the call is typed as the method -- receiver, selector,
// then the method's parameters -- and made through the trampoline's
// address, with the same calling convention any C++ call of that type
// gets. That is what clang does too, casting objc_msgSend at each site.

// objcState is an Objective-C++ unit's lowering state, nil for a unit
// without Objective-C.
type objcState struct {
	info *sema.ObjCInfo
	abi  objcrt.ABI
	arch objcrt.Arch

	// The references the runtime rewrites at load, one per selector,
	// class, and super target.
	selRefs   map[string]ir.Symbol
	classRefs map[string]ir.Symbol
	superRefs map[string]ir.Symbol

	// classSyms are class and metaclass objects, defined here for a class
	// this unit implements and imported otherwise; ivarSyms the offset
	// variables, and protoSyms the protocol objects.
	classSyms map[string]ir.Symbol
	ivarSyms  map[string]ir.Symbol
	protoSyms map[string]ir.Symbol

	// strs interns the metadata's C strings and @"…" objects.
	strs map[string]ir.Symbol
	anon int

	// methods is what each method body compiled here is: its class,
	// category, and whether it is a class method.
	methods map[*sema.FuncSymbol]*objcMethodFn

	// blocks are the block literals by invoke function; blockOwner the
	// method a block is written in, whose self and super its body means.
	blocks       map[*sema.FuncSymbol]*sema.BlockInfo
	blockOwner   map[*sema.FuncSymbol]*objcMethodFn
	blockLayouts map[*sema.BlockInfo]*blockLayout
	byrefLayouts map[*sema.VarSymbol]*byrefLayout

	// The lists the image info points the runtime at.
	classList, categoryList               []ir.Symbol
	nonLazyClassList, nonLazyCategoryList []ir.Symbol
}

// objcMethodFn is one method body.
type objcMethodFn struct {
	class    *types.ObjCInterface
	category string
	method   *types.ObjCMethod
	sym      *sema.FuncSymbol
	isClass  bool
}

// initObjC sets up the state of an Objective-C++ unit, and defines the
// symbols a body may name before the metadata is written: the class
// objects and instance variable offsets of what the unit implements.
func (u *unit) initObjC() {
	info := u.res.Info.ObjC
	if info == nil {
		if !u.model.ObjC {
			return
		}
		// A .mm unit with no message in it is still compiled with ARC,
		// and its @-statements are still the runtime's.
		info = sema.NewObjCInfo()
	}
	arch := objcrt.ARM64
	if use := u.opt.Target.Use(); strings.Contains(use, "amd64") || strings.Contains(use, "x86_64") {
		arch = objcrt.AMD64
	}
	u.objc = &objcState{
		info:      info,
		abi:       objcrt.Darwin64(),
		arch:      arch,
		selRefs:   map[string]ir.Symbol{},
		classRefs: map[string]ir.Symbol{},
		superRefs: map[string]ir.Symbol{},
		classSyms: map[string]ir.Symbol{},
		ivarSyms:  map[string]ir.Symbol{},
		protoSyms: map[string]ir.Symbol{},
		strs:      map[string]ir.Symbol{},
		methods:   map[*sema.FuncSymbol]*objcMethodFn{},

		blocks:       map[*sema.FuncSymbol]*sema.BlockInfo{},
		blockOwner:   map[*sema.FuncSymbol]*objcMethodFn{},
		blockLayouts: map[*sema.BlockInfo]*blockLayout{},
		byrefLayouts: map[*sema.VarSymbol]*byrefLayout{},
	}
	for _, b := range u.res.Info.Blocks {
		u.objc.blocks[b.Invoke] = b
	}
	for _, impl := range info.Impls {
		for _, m := range impl.Methods {
			u.objc.methods[m.Func] = &objcMethodFn{
				class:    impl.Class,
				category: impl.Category,
				method:   m.Method,
				sym:      m.Func,
				isClass:  m.Method.Class,
			}
		}
		if impl.Category == "" {
			u.declareObjCClass(impl.Class)
			u.declareObjCIvars(impl.Class)
		}
	}
}

// objcName is a name for something the source did not name.
func (u *unit) objcName(prefix string) string {
	u.objc.anon++
	return u.symbolName(fmt.Sprintf("%s%d", prefix, u.objc.anon))
}

// objcImport imports a runtime function once, sharing the import with a
// declaration the unit's headers made of the same name.
func (u *unit) objcImport(name string, sig *ir.Sig) ir.Callee {
	sym := u.symbolName(name)
	if f, ok := u.defsByName[sym]; ok {
		return f
	}
	if f, ok := u.importsByName[sym]; ok {
		return f
	}
	f := u.mod.ImportFunc(sym, sig)
	u.importsByName[sym] = f
	return f
}

// ---- expressions ----

// objcExpr lowers an Objective-C expression, and the C++ ones that mean
// a message here: a property, an object subscript, and an assignment to
// either. It reports false for anything else.
func (fl *fn) objcExpr(e ast.Expr) (ir.Value, bool) {
	o := fl.u.objc
	switch e := e.(type) {
	case *ast.ObjCMessageExpr:
		return fl.objcMessage(e), true
	case *ast.ObjCStringLit:
		sym := fl.u.objcConstantString(e)
		if sym == nil {
			return nil, true
		}
		return fl.blk.Ptr.GetAddr(sym), true
	case *ast.ObjCSelectorExpr:
		return fl.objcSelector(e.Name), true
	case *ast.ObjCProtocolExpr:
		if e.Name == nil {
			return fl.blk.Ptr.Const(), true
		}
		p := fl.u.objcProtocolNamed(e.Name.Text(fl.u.unit))
		if p == nil {
			return fl.blk.Ptr.Const(), true
		}
		return fl.blk.Ptr.GetAddr(fl.u.objcProtocol(p)), true
	case *ast.ObjCEncodeExpr:
		t := fl.u.res.Info.TypeIds[e.Type]
		s := o.abi.Encode(t, fl.u.model)
		return fl.blk.Ptr.GetAddr(fl.u.objcCString(s, objcrt.SecCString, objcrt.CStringLabel)), true
	case *ast.ObjCBoolLit:
		v := int64(0)
		if e.Value {
			v = 1
		}
		return fl.u.constOf(fl.blk, fl.typeOf(e), v), true
	case *ast.ObjCAvailableExpr:
		// Everything this compiler targets is at least what the SDK
		// requires, so an availability check is always true.
		return fl.u.constOf(fl.blk, fl.typeOf(e), 1), true
	case *ast.ObjCBoxedExpr:
		return fl.objcBoxed(e), true
	case *ast.ObjCArrayLit:
		return fl.objcArrayLit(e), true
	case *ast.ObjCDictLit:
		return fl.objcDictLit(e), true
	case *ast.BlockExpr:
		return fl.blockExpr(e), true
	case *ast.ObjCBridgeCast:
		v := fl.expr(e.X)
		if v == nil {
			return nil, true
		}
		v = fl.convert(v, fl.typeOf(e.X), fl.typeOf(e))
		switch e.Kind {
		case "__bridge_retained":
			// Out of ARC at +1: CoreFoundation's to release.
			if !fl.objcTake(v) {
				v = fl.objcRetain(v, types.ObjCId)
			}
		case "__bridge_transfer":
			// Into ARC at +1: this expression owns it.
			fl.objcOwn(v)
		}
		return v, true
	case *ast.MemberExpr:
		if p := o.info.Props[e]; p != nil {
			return fl.objcPropGet(e, p), true
		}
	case *ast.IndexExpr:
		if s := o.info.Subs[e]; s != nil {
			return fl.objcSubscriptGet(e, s), true
		}
	case *ast.IncDecExpr:
		if m, ok := unparen(e.X).(*ast.MemberExpr); ok && o.info.Props[m] != nil {
			return fl.objcPropIncDec(m, o.info.Props[m], e.Op, false), true
		}
	case *ast.UnaryExpr:
		if e.Op == token.INC || e.Op == token.DEC {
			if m, ok := unparen(e.X).(*ast.MemberExpr); ok && o.info.Props[m] != nil {
				return fl.objcPropIncDec(m, o.info.Props[m], e.Op, true), true
			}
		}
	case *ast.AssignExpr:
		switch lhs := unparen(e.Lhs).(type) {
		case *ast.MemberExpr:
			if p := o.info.Props[lhs]; p != nil {
				return fl.objcPropSet(e, lhs, p), true
			}
		case *ast.IndexExpr:
			if s := o.info.Subs[lhs]; s != nil {
				return fl.objcSubscriptSet(e, lhs, s), true
			}
		}
	}
	return nil, false
}

// constOf is an integer constant in t's register.
func (u *unit) constOf(b *ir.Block, t types.Type, v int64) ir.Value {
	switch u.regType(t) {
	case ir.TypeI1:
		return b.I1.Const(v != 0)
	case ir.TypeI64:
		return b.I64.Const(v)
	}
	return b.I32.Const(v)
}

// objcMessage lowers [recv sel:args…].
func (fl *fn) objcMessage(e *ast.ObjCMessageExpr) ir.Value {
	send := fl.u.objc.info.Sends[e]
	if send == nil || send.Method == nil {
		fl.u.errorf(e.Pos(), "lowering found no method for this message")
		return nil
	}
	recv, ok := fl.objcReceiver(e.Recv, send.Super)
	if !ok {
		return nil
	}
	if send.Method.Family == "init" && !send.Method.Class && !send.ClassRecv {
		// An init method consumes its receiver: one this expression made,
		// [Box alloc], is handed over, and any other -- a variable, or
		// super, which is self -- is retained for it.
		if send.Super {
			self, _ := fl.objcSelf()
			fl.objcRetain(self, types.ObjCId)
		} else if !fl.objcTake(recv) {
			recv = fl.objcRetain(recv, types.ObjCId)
		}
	}
	var args []ast.Expr
	for _, p := range e.Parts {
		args = append(args, p.Args...)
	}
	vals, ok := fl.objcArgs(args, send.ParamTypes)
	if !ok {
		return nil
	}
	v := fl.objcSend(recv, send.Super, send.Selector, vals, send.ParamTypes, send.Method.Variadic, send.Result, e.Pos())
	if len(fl.writebacks) > 0 {
		fl.objcWriteBack()
	}
	if send.Method.ReturnsRetained && retainable(send.Result) {
		// alloc, copy, init, mutableCopy and new hand back an object the
		// caller owns.
		fl.objcOwn(v)
	}
	return v
}

// objcReceiver lowers what a message goes to: a class object for a class
// name, the two-word objc_super for super, and otherwise the value.
func (fl *fn) objcReceiver(x ast.Expr, super bool) (ir.Value, bool) {
	switch r := x.(type) {
	case *ast.ObjCSuperExpr:
		m := fl.objcContext()
		self, ok := fl.objcSelf()
		if m == nil || !ok {
			fl.u.errorf(x.Pos(), "super outside a method")
			return nil, false
		}
		// objc_msgSendSuper2 takes { self, the class the method is in }
		// and starts its search one above that class -- the metaclass,
		// for a class method.
		st := fl.entry.Ptr.Alloc(16, 8)
		fl.blk.Ptr.Store(self, st)
		fl.blk.Ptr.Store(fl.objcSuperRef(m.class.Name, m.isClass), fl.blk.Ptr.Add(st, fl.blk.I64.Const(8)))
		return st, true
	case *ast.ObjCClassRecv:
		name := r.Name.Text(fl.u.unit)
		if c := fl.u.objc.info; c != nil {
			if send := fl.classRecvClass(name); send != nil {
				return fl.objcClassRef(send.Name), true
			}
		}
		fl.u.errorf(x.Pos(), "lowering: %q is not a class", name)
		return nil, false
	}
	v := fl.expr(x)
	if v == nil {
		return nil, false
	}
	return v, true
}

// classRecvClass is the class a class-name receiver names.
func (fl *fn) classRecvClass(name string) *types.ObjCInterface {
	if fl.u.res.GlobalScope == nil || fl.u.res.GlobalScope.ObjC == nil {
		return nil
	}
	return fl.u.res.GlobalScope.ObjC.Classes[name]
}

// objcContext is the method this code is in: the function's own, or for a
// block's body the method the block is written in.
func (fl *fn) objcContext() *objcMethodFn {
	if m := fl.u.objc.methods[fl.sym]; m != nil {
		return m
	}
	return fl.u.objc.blockOwner[fl.sym]
}

// objcSelf is the method's self: its first parameter, or in a block the
// self the block captured.
func (fl *fn) objcSelf() (ir.Ptr, bool) {
	selfSym := (*sema.VarSymbol)(nil)
	if b := fl.u.blockOf(fl.sym); b != nil {
		selfSym = b.Self
	} else if len(fl.sym.Params) > 0 {
		selfSym = fl.sym.Params[0]
	}
	if selfSym == nil {
		return ir.Ptr{}, false
	}
	slot, ok := fl.slots[selfSym]
	if !ok {
		return ir.Ptr{}, false
	}
	p, ok := fl.load(slot, selfSym.SymType).(ir.Ptr)
	return p, ok
}

// objcArgs lowers a message's arguments, converted to the method's
// parameters, and past them promoted as a variadic call's are.
func (fl *fn) objcArgs(args []ast.Expr, params []types.Type) ([]ir.Value, bool) {
	out := make([]ir.Value, 0, len(args))
	for i, a := range args {
		var want types.Type
		if i < len(params) {
			want = params[i]
		}
		if rec := classOf(want); rec != nil {
			addr, ok := fl.objectOf(a)
			if !ok {
				return nil, false
			}
			if !fl.u.plainForCalls(rec) || fl.u.paramDestroyedInCallee(rec) {
				if fl.u.paramDestroyedInCallee(rec) && fl.takeTemporary(addr) {
					out = append(out, addr)
					continue
				}
				tmp := fl.alloc(rec, "")
				if !fl.copyObject(tmp, addr, rec, nil) {
					return nil, false
				}
				if !fl.u.paramDestroyedInCallee(rec) {
					fl.temporary(tmp, rec)
				}
				addr = tmp
			}
			out = append(out, addr)
			continue
		}
		if isReference(want) {
			addr, ok := fl.bind(a, want)
			if !ok {
				return nil, false
			}
			out = append(out, addr)
			continue
		}
		if wb, ok := fl.objcOutArg(a, want); ok {
			out = append(out, wb)
			continue
		}
		if rec := classOf(fl.typeOf(a)); want == nil && rec != nil {
			if addr, ok := fl.objectOf(a); ok {
				if words, ok := fl.variadicClass(addr, rec); ok {
					out = append(out, words...)
					continue
				}
			}
		}
		v, _, converted := fl.convertedScalar(a)
		if !converted {
			v = fl.expr(a)
		}
		if v == nil {
			return nil, false
		}
		from := fl.typeOf(a)
		if want == nil {
			want = fl.promoted(from)
		}
		out = append(out, fl.convert(v, from, want))
	}
	return out, true
}

// promoted is a variadic argument's type after [expr.call]/12's
// promotions: a float travels as a double, a narrow integer as an int.
func (fl *fn) promoted(t types.Type) types.Type {
	from := types.Unqualify(types.RemoveReference(t))
	switch {
	case types.IsFloat(from) && fl.u.regType(from) == ir.TypeF32:
		return types.Typ(types.Double)
	case types.IsInteger(from) || types.IsEnum(from) || types.IsBool(from):
		if size, _ := fl.u.sizeAlign(from); size < 4 {
			return types.Typ(types.Int)
		}
	}
	return nil
}

// objcSend emits one message: the call every construct that means a
// message goes through. recv is the object, or objc_super's address for
// a super send; args are lowered already.
func (fl *fn) objcSend(recv ir.Value, super bool, sel string, args []ir.Value, params []types.Type, variadic bool, ret types.Type, at ast.Tok) ir.Value {
	if fl.blk == nil {
		return nil
	}
	o := fl.u.objc
	// The call's type: receiver, selector, the method's parameters, and
	// the arguments past them as the variadic tail.
	ft := &types.Func{Ret: ret, Variadic: variadic}
	ft.Params = append(ft.Params, types.Param{Type: types.ObjCId}, types.Param{Type: &types.Pointer{Elem: types.Typ(types.Void)}})
	for _, p := range params {
		ft.Params = append(ft.Params, types.Param{Type: p})
	}
	callee := &sema.FuncSymbol{SymName: "objc_msgSend", FuncType: ft}
	fnType := fl.u.funcType(callee)

	all := []ir.Value{recv, fl.objcSelector(sel)}
	var result ir.Ptr
	retRec := classOf(ret)
	if retRec != nil {
		result = fl.alloc(retRec, "")
		if fl.resultInto != (ir.Ptr{}) {
			result = fl.resultInto
			fl.resultInto = ir.Ptr{}
		}
		// A message to nil returns zero. A result the runtime hands back
		// in registers it clears itself; one returned in memory it never
		// touches, so the memory is cleared first.
		if size, _ := fl.u.sizeAlign(retRec); size > 16 && !super {
			fl.blk.MemSet(result, fl.blk.I32.Const(0), fl.blk.I64.Const(size))
		}
		all = append([]ir.Value{result}, all...)
	}
	all = append(all, args...)

	name := o.abi.Send(o.arch, ret, fl.u.model, super)
	imp := fl.u.objcImport(name, ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr).Variadic().Ret(ir.TypePtr))
	fp := fl.blk.Ptr.GetAddr(imp)
	res := fl.emitCallInd(fp, fnType, all...)
	if retRec != nil {
		return result
	}
	if len(res) == 0 {
		return nil
	}
	return res[0]
}

// objcSendTo is a message to a class by name, which a literal means.
func (fl *fn) objcSendTo(class, sel string, args []ir.Value, params []types.Type, ret types.Type, at ast.Tok) ir.Value {
	return fl.objcSend(fl.objcClassRef(class), false, sel, args, params, false, ret, at)
}

// ---- the references the runtime rewrites ----

// objcSelector loads a selector's SEL. The reference is a global the
// runtime rewrites at load: the compiler writes the name's address, and
// the runtime the unique selector.
func (fl *fn) objcSelector(sel string) ir.Value {
	o := fl.u.objc
	sym, ok := o.selRefs[sel]
	if !ok {
		sym = fl.u.mod.Global(fl.u.objcName(objcrt.SelectorRefPrefix), ir.RW, ir.StorePtr.FType()).
			Internal().
			Section(o.abi.Name(objcrt.SecSelectorRefs)).
			Align(8).
			Init(ir.RelocInit(fl.u.objcMethodName(sel)))
		o.selRefs[sel] = sym
	}
	return fl.blk.Ptr.Load(fl.blk.Ptr.GetAddr(sym))
}

// objcClassRef loads a class object.
func (fl *fn) objcClassRef(class string) ir.Ptr {
	o := fl.u.objc
	sym, ok := o.classRefs[class]
	if !ok {
		sym = fl.u.mod.Global(fl.u.objcName(objcrt.ClassRefPrefix), ir.RW, ir.StorePtr.FType()).
			Internal().
			Section(o.abi.Name(objcrt.SecClassRefs)).
			Align(8).
			Init(ir.RelocInit(fl.u.objcClassSymbol(objcrt.ClassSymbol(class))))
		o.classRefs[class] = sym
	}
	return fl.blk.Ptr.Load(fl.blk.Ptr.GetAddr(sym))
}

// objcSuperRef loads what a super send starts above: the class for an
// instance method and the metaclass for a class method.
func (fl *fn) objcSuperRef(class string, meta bool) ir.Ptr {
	o := fl.u.objc
	symbol, key := objcrt.ClassSymbol(class), class
	if meta {
		symbol, key = objcrt.MetaclassSymbol(class), "+"+class
	}
	sym, ok := o.superRefs[key]
	if !ok {
		sym = fl.u.mod.Global(fl.u.objcName(objcrt.SuperRefPrefix), ir.RW, ir.StorePtr.FType()).
			Internal().
			Section(o.abi.Name(objcrt.SecSuperRefs)).
			Align(8).
			Init(ir.RelocInit(fl.u.objcClassSymbol(symbol)))
		o.superRefs[key] = sym
	}
	return fl.blk.Ptr.Load(fl.blk.Ptr.GetAddr(sym))
}

// objcClassSymbol is a class or metaclass object, imported where this
// unit does not define it.
func (u *unit) objcClassSymbol(symbol string) ir.Symbol {
	if s, ok := u.objc.classSyms[symbol]; ok {
		return s
	}
	s := ir.Symbol(u.mod.ImportGlobal(u.symbolName(symbol), ir.StorePtr.FType()))
	u.objc.classSyms[symbol] = s
	return s
}

// objcMethodName interns a selector's characters.
func (u *unit) objcMethodName(sel string) ir.Symbol {
	return u.objcCString(sel, objcrt.SecMethodNames, objcrt.MethodNameLabel)
}

// objcClassName interns a class, category or protocol name.
func (u *unit) objcClassName(name string) ir.Symbol {
	return u.objcCString(name, objcrt.SecClassNames, objcrt.ClassNameLabel)
}

// objcMethodType interns a type encoding.
func (u *unit) objcMethodType(s string) ir.Symbol {
	return u.objcCString(s, objcrt.SecMethodTypes, objcrt.MethodTypeLabel)
}

// objcCString interns one NUL-terminated string in a section.
func (u *unit) objcCString(s string, sec objcrt.Section, label string) ir.Symbol {
	key := label + "\x00" + s
	if sym, ok := u.objc.strs[key]; ok {
		return sym
	}
	g := u.mod.Global(u.objcName(label), ir.RO, ir.Array(uint64(len(s)+1), ir.StoreI8.FType())).
		Internal().
		Section(u.objc.abi.Name(sec)).
		Init(ir.Str(s + "\x00"))
	u.objc.strs[key] = g
	return g
}

// objcConstantString is the object @"…" is: a constant CFString whose
// characters are 8-bit where they are ASCII and UTF-16 where they are not.
func (u *unit) objcConstantString(e *ast.ObjCStringLit) ir.Symbol {
	s, err := literal.Decode(u.unit, e.Str)
	if err != nil {
		u.errorf(e.Pos(), "%v", err)
		return nil
	}
	raw := s.Bytes()
	ascii := true
	for _, c := range raw {
		if c > 0x7F {
			ascii = false
			break
		}
	}
	key := "cfstr\x00" + string(raw)
	if sym, ok := u.objc.strs[key]; ok {
		return sym
	}
	var data ir.Symbol
	flags, n := objcrt.CFStringASCII, len(raw)
	if ascii {
		data = u.objcCString(string(raw), objcrt.SecCString, objcrt.CStringLabel)
	} else {
		var runes []rune
		for b := raw; len(b) > 0; {
			r, size := utf8.DecodeRune(b)
			runes = append(runes, r)
			b = b[size:]
		}
		w := utf16.Encode(runes)
		items := make([]ir.Init, 0, len(w)+1)
		for _, c := range w {
			items = append(items, ir.Lit(ir.Int(int64(c))))
		}
		items = append(items, ir.Lit(ir.Int(0)))
		data = u.mod.Global(u.objcName(objcrt.CStringLabel), ir.RO, ir.Array(uint64(len(items)), ir.StoreI16.FType())).
			Internal().
			Section(u.objc.abi.Name(objcrt.SecUString)).
			Align(2).
			Init(ir.List(items...))
		flags, n = objcrt.CFStringUTF16, len(w)
	}
	isa := u.objcClassSymbol(u.objc.abi.ConstantStringClass())
	g := u.mod.Global(u.objcName(objcrt.ConstStringLabel), ir.RW, u.objcMetaType("constant_string", objcrt.ConstantString).FType()).
		Internal().
		Section(u.objc.abi.Name(objcrt.SecCFString)).
		Align(8).
		Init(ir.List(
			ir.RelocInit(isa),
			ir.Lit(ir.Int(int64(flags))),
			ir.Lit(ir.Int(0)),
			ir.RelocInit(data),
			ir.Lit(ir.Int(int64(n)))))
	u.objc.strs[key] = g
	return g
}

// ---- instance variables ----

// objcIvarAddr is the address of an instance variable an expression
// names, and its type; false where e names none.
func (fl *fn) objcIvarAddr(e ast.Expr) (ir.Ptr, types.Type, bool, bool) {
	ref := fl.u.objc.info.Ivars[e]
	if ref == nil {
		return ir.Ptr{}, nil, false, false
	}
	var obj ir.Ptr
	if ref.Implicit {
		self, ok := fl.objcSelf()
		if !ok {
			fl.u.errorf(e.Pos(), "an instance variable outside a method")
			return ir.Ptr{}, nil, false, true
		}
		obj = self
	} else {
		m, isMember := e.(*ast.MemberExpr)
		if !isMember {
			return ir.Ptr{}, nil, false, false
		}
		v, ok := fl.expr(m.X).(ir.Ptr)
		if !ok {
			return ir.Ptr{}, nil, false, true
		}
		obj = v
	}
	// The offset is loaded, not added as a constant: the runtime writes
	// it when it realizes the class, which is the non-fragile ABI.
	sym := fl.u.objcIvarOffset(ref.Class.Name, ref.Ivar.Name)
	off := fl.blk.I64.SLoad32(fl.blk.Ptr.GetAddr(sym))
	return fl.blk.Ptr.Add(obj, off), ref.Ivar.Type, true, true
}

// objcIvarOffset is one instance variable's offset variable, imported for
// a class another image implements.
func (u *unit) objcIvarOffset(class, ivar string) ir.Symbol {
	name := objcrt.IvarOffsetSymbol(class, ivar)
	if s, ok := u.objc.ivarSyms[name]; ok {
		return s
	}
	s := ir.Symbol(u.mod.ImportGlobal(u.symbolName(name), ir.StoreI32.FType()))
	u.objc.ivarSyms[name] = s
	return s
}

// ---- properties and subscripts ----

// objcPropRecv is the receiver of a property reference.
func (fl *fn) objcPropRecv(m *ast.MemberExpr, p *sema.ObjCPropRef) (ir.Value, bool) {
	if p.ClassProp {
		return fl.objcClassRef(p.Class.Name), true
	}
	return fl.objcReceiver(m.X, false)
}

func (fl *fn) objcPropGet(m *ast.MemberExpr, p *sema.ObjCPropRef) ir.Value {
	recv, ok := fl.objcPropRecv(m, p)
	if !ok {
		return nil
	}
	return fl.objcSend(recv, false, p.Getter, nil, nil, false, fl.typeOf(m), m.Pos())
}

// objcPropSet lowers `obj.prop = v` and the compound forms: a setter
// message whose value is the expression's.
func (fl *fn) objcPropSet(e *ast.AssignExpr, m *ast.MemberExpr, p *sema.ObjCPropRef) ir.Value {
	recv, ok := fl.objcPropRecv(m, p)
	if !ok {
		return nil
	}
	t := fl.typeOf(m)
	var v ir.Value
	if e.Op == token.ASSIGN {
		vals, ok := fl.objcArgs([]ast.Expr{e.Rhs}, []types.Type{t})
		if !ok {
			return nil
		}
		v = vals[0]
	} else {
		old := fl.objcSend(recv, false, p.Getter, nil, nil, false, t, m.Pos())
		v = fl.objcCompound(e, old, t)
		if v == nil {
			return nil
		}
	}
	fl.objcSend(recv, false, p.Setter, []ir.Value{v}, []types.Type{t}, false, types.Typ(types.Void), e.Pos())
	return v
}

// objcCompound is `old op= rhs` for a property, whose old value came
// from its getter.
func (fl *fn) objcCompound(e *ast.AssignExpr, old ir.Value, t types.Type) ir.Value {
	rhs := fl.expr(e.Rhs)
	if rhs == nil || old == nil {
		return nil
	}
	op, ok := compoundOp(e.Op)
	if !ok {
		fl.u.errorf(e.Pos(), "lowering: %s on a property", e.Op)
		return nil
	}
	return fl.arith(op, e.Pos(), old, fl.convert(rhs, fl.typeOf(e.Rhs), t), t)
}

func (fl *fn) objcSubscriptGet(e *ast.IndexExpr, s *sema.ObjCSubscript) ir.Value {
	recv := fl.expr(e.X)
	if recv == nil || len(e.Args) != 1 {
		return nil
	}
	key, keyT := fl.objcSubscriptKey(e.Args[0], s)
	if key == nil {
		return nil
	}
	return fl.objcSend(recv, false, s.Getter, []ir.Value{key}, []types.Type{keyT}, false, types.ObjCId, e.Pos())
}

func (fl *fn) objcSubscriptSet(a *ast.AssignExpr, e *ast.IndexExpr, s *sema.ObjCSubscript) ir.Value {
	if a.Op != token.ASSIGN {
		fl.u.errorf(a.Pos(), "lowering: %s on an object subscript", a.Op)
		return nil
	}
	recv := fl.expr(e.X)
	if recv == nil || len(e.Args) != 1 {
		return nil
	}
	vals, ok := fl.objcArgs([]ast.Expr{a.Rhs}, []types.Type{types.ObjCId})
	if !ok {
		return nil
	}
	key, keyT := fl.objcSubscriptKey(e.Args[0], s)
	if key == nil {
		return nil
	}
	fl.objcSend(recv, false, s.Setter, []ir.Value{vals[0], key}, []types.Type{types.ObjCId, keyT}, false, types.Typ(types.Void), a.Pos())
	return vals[0]
}

// objcSubscriptKey is a subscript's index, an NSUInteger, or its key, an
// object.
func (fl *fn) objcSubscriptKey(x ast.Expr, s *sema.ObjCSubscript) (ir.Value, types.Type) {
	t := types.Type(types.ObjCId)
	if !s.Keyed {
		t = types.Typ(types.ULong)
	}
	vals, ok := fl.objcArgs([]ast.Expr{x}, []types.Type{t})
	if !ok {
		return nil, nil
	}
	return vals[0], t
}

// ---- literals ----

// objcBoxed lowers @(e): a numberWith… message to NSNumber, or
// stringWithUTF8String: to NSString.
func (fl *fn) objcBoxed(e *ast.ObjCBoxedExpr) ir.Value {
	lit := fl.u.objc.info.Literals[e]
	if lit == nil || lit.Selector == "" {
		return nil
	}
	if rec := classOf(lit.Arg); rec != nil {
		// [NSValue valueWithBytes:&copy objCType:@encode(T)].
		src, ok := fl.objectOf(e.X)
		if !ok {
			return nil
		}
		tmp := fl.alloc(rec, "boxed")
		fl.copyObjectBytes(tmp, src, rec)
		enc := fl.blk.Ptr.GetAddr(fl.u.objcCString(fl.u.objc.abi.Encode(rec, fl.u.model), objcrt.SecCString, objcrt.CStringLabel))
		ptrT := &types.Pointer{Elem: types.Typ(types.Void)}
		return fl.objcSendTo(lit.Class, lit.Selector, []ir.Value{tmp, enc}, []types.Type{ptrT, ptrT}, fl.typeOf(e), e.Pos())
	}
	vals, ok := fl.objcArgs([]ast.Expr{e.X}, []types.Type{lit.Arg})
	if !ok {
		return nil
	}
	return fl.objcSendTo(lit.Class, lit.Selector, vals, []types.Type{lit.Arg}, fl.typeOf(e), e.Pos())
}

// objcObjects stores objects into a stack array, for a collection
// literal's message.
func (fl *fn) objcObjects(xs []ast.Expr) (ir.Ptr, bool) {
	n := int64(len(xs))
	if n == 0 {
		n = 1
	}
	arr := fl.entry.Ptr.Alloc(uint64(8*n), 8)
	for i, x := range xs {
		vals, ok := fl.objcArgs([]ast.Expr{x}, []types.Type{types.ObjCId})
		if !ok {
			return ir.Ptr{}, false
		}
		fl.store(fl.blk.Ptr.Add(arr, fl.blk.I64.Const(int64(8*i))), vals[0], types.ObjCId)
	}
	return arr, true
}

func (fl *fn) objcArrayLit(e *ast.ObjCArrayLit) ir.Value {
	lit := fl.u.objc.info.Literals[e]
	objs, ok := fl.objcObjects(e.Elems)
	if !ok || lit == nil {
		return nil
	}
	nsuint := types.Typ(types.ULong)
	ptr := &types.Pointer{Elem: types.ObjCId}
	return fl.objcSendTo(lit.Class, lit.Selector,
		[]ir.Value{objs, fl.blk.I64.Const(int64(len(e.Elems)))},
		[]types.Type{ptr, nsuint}, fl.typeOf(e), e.Pos())
}

func (fl *fn) objcDictLit(e *ast.ObjCDictLit) ir.Value {
	lit := fl.u.objc.info.Literals[e]
	vals, ok := fl.objcObjects(e.Values)
	if !ok || lit == nil {
		return nil
	}
	keys, ok := fl.objcObjects(e.Keys)
	if !ok {
		return nil
	}
	nsuint := types.Typ(types.ULong)
	ptr := &types.Pointer{Elem: types.ObjCId}
	return fl.objcSendTo(lit.Class, lit.Selector,
		[]ir.Value{vals, keys, fl.blk.I64.Const(int64(len(e.Keys)))},
		[]types.Type{ptr, ptr, nsuint}, fl.typeOf(e), e.Pos())
}

// ---- statements ----

// objcStmt lowers the @-statements, and reports false for any other.
func (fl *fn) objcStmt(s ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.ObjCAutoreleaseStmt:
		// The pool is a scope object: every way out of the block pops it.
		push := fl.u.objcImport(objcrt.AutoreleasePoolPush, ir.NewSig().Ret(ir.TypePtr))
		fl.pushScope()
		tok := fl.blk.Call(push).Ptr(0)
		slot := fl.alloc(types.ObjCId, "pool")
		fl.blk.Ptr.Store(tok, slot)
		fl.trackARC(slot, arcPool)
		fl.stmt(s.Body)
		fl.popScope()
		return true
	case *ast.ObjCSyncStmt:
		// The lock is held by a scope object, released on every way out,
		// the unwinding one included.
		fl.pushScope()
		defer fl.popScope()
		obj := fl.objcRetained(s.X, types.ObjCId)
		p, ok := obj.(ir.Ptr)
		if !ok || fl.blk == nil {
			return true
		}
		held := fl.alloc(types.ObjCId, "sync_obj")
		fl.blk.Ptr.Store(p, held)
		fl.trackARC(held, arcStrong)
		fl.endFullExpr()
		enter := fl.u.objcImport(objcrt.SyncEnter, ir.NewSig().Param(ir.TypePtr).Ret(ir.TypeI32))
		fl.emitCall(enter, p)
		fl.trackARC(held, arcSync)
		fl.stmt(s.Body)
		return true
	case *ast.ObjCThrowStmt:
		if s.X == nil {
			fl.emitCall(fl.u.objcImport(objcrt.ExceptionRethrow, ir.NewSig()))
		} else if obj, ok := fl.expr(s.X).(ir.Ptr); ok {
			fl.emitCall(fl.u.objcImport(objcrt.ExceptionThrow, ir.NewSig().Param(ir.TypePtr)), obj)
		}
		if fl.blk != nil {
			fl.blk.Trap()
			fl.blk = nil
		}
		return true
	case *ast.ObjCForInStmt:
		fl.objcForIn(s)
		return true
	case *ast.ObjCTryStmt:
		fl.objcTry(s)
		return true
	}
	return false
}

// objcObjectTemp is a class-typed property, subscript or message result
// where an object's address is wanted -- `Vec3 p = b.position` -- which
// is the temporary the message returned into.
func (fl *fn) objcObjectTemp(e ast.Expr) (ir.Ptr, types.Type, bool, bool) {
	o := fl.u.objc
	switch x := e.(type) {
	case *ast.MemberExpr:
		if o.info.Props[x] == nil {
			return ir.Ptr{}, nil, false, false
		}
	case *ast.IndexExpr:
		if o.info.Subs[x] == nil {
			return ir.Ptr{}, nil, false, false
		}
	case *ast.ObjCMessageExpr:
	default:
		return ir.Ptr{}, nil, false, false
	}
	t := fl.typeOf(e)
	if classOf(t) == nil {
		return ir.Ptr{}, nil, false, false
	}
	v, _ := fl.objcExpr(e)
	p, ok := v.(ir.Ptr)
	return p, t, ok, true
}

// objcPropIncDec lowers ++ and -- on a property: its getter, then its
// setter with the value one along.
func (fl *fn) objcPropIncDec(m *ast.MemberExpr, p *sema.ObjCPropRef, op token.Kind, prefix bool) ir.Value {
	recv, ok := fl.objcPropRecv(m, p)
	if !ok {
		return nil
	}
	t := fl.typeOf(m)
	old := fl.objcSend(recv, false, p.Getter, nil, nil, false, t, m.Pos())
	if old == nil {
		return nil
	}
	arith := token.ADD
	if op == token.DEC {
		arith = token.SUB
	}
	v := fl.arith(arith, m.Pos(), old, fl.convert(fl.oneOf(t), t, t), t)
	if v == nil {
		return nil
	}
	fl.objcSend(recv, false, p.Setter, []ir.Value{v}, []types.Type{t}, false, types.Typ(types.Void), m.Pos())
	if prefix {
		return v
	}
	return old
}

// objcForIn lowers fast enumeration, `for (T x in coll)`: batches of
// objects from countByEnumeratingWithState:objects:count:, and a check,
// before each element, that the collection has not changed under the
// loop -- objc_enumerationMutation raises if it has.
func (fl *fn) objcForIn(s *ast.ObjCForInStmt) {
	fl.pushScope()
	defer fl.popScope()

	// The loop variable: declared here, or an lvalue named.
	var slot ir.Ptr
	var elemT types.Type
	if s.Decl != nil {
		fl.stmt(&ast.DeclStmt{Span: s.Decl.Span, Decl: s.Decl})
		if len(s.Decl.Inits) != 1 {
			fl.u.errorf(s.Pos(), "fast enumeration declares one variable")
			return
		}
		v, _ := fl.u.res.Info.Defs[s.Decl.Inits[0]].(*sema.VarSymbol)
		if v == nil || fl.slots[v] == (ir.Ptr{}) {
			fl.u.errorf(s.Pos(), "lowering found no storage for the loop variable")
			return
		}
		slot, elemT = fl.slots[v], v.SymType
	} else {
		p, t, ok := fl.lvalue(s.X)
		if !ok {
			return
		}
		slot, elemT = p, t
	}
	coll, ok := fl.expr(s.Coll).(ir.Ptr)
	if !ok || fl.blk == nil {
		return
	}
	collSlot := fl.alloc(types.ObjCId, "enum_coll")
	fl.blk.Ptr.Store(coll, collSlot)

	stateSize := fl.u.objc.abi.SizeOf(objcrt.FastEnumerationState)
	state := fl.entry.Ptr.Alloc(uint64(stateSize), 8)
	items := fl.entry.Ptr.Alloc(uint64(objcrt.FastEnumerationBatch*8), 8)
	mutations := fl.entry.Ptr.Alloc(8, 8)
	idx := fl.entry.Ptr.Alloc(8, 8)
	count := fl.entry.Ptr.Alloc(8, 8)
	fl.blk.MemSet(state, fl.blk.I32.Const(0), fl.blk.I64.Const(stateSize))
	mutOff, _ := fl.u.objc.abi.OffsetOf(objcrt.FastEnumerationState, "mutationsPtr")
	itemsOff, _ := fl.u.objc.abi.OffsetOf(objcrt.FastEnumerationState, "itemsPtr")

	batch := fl.block("forin_batch")
	first := fl.block("forin_first")
	inner := fl.block("forin_inner")
	body := fl.block("forin_body")
	mutated := fl.block("forin_mutated")
	live := fl.block("forin_live")
	next := fl.block("forin_next")
	done := fl.block("forin_done")

	nsuint := types.Typ(types.ULong)
	ptrT := &types.Pointer{Elem: types.Typ(types.Void)}
	ask := func() ir.I64 {
		n := fl.objcSend(fl.blk.Ptr.Load(collSlot), false, objcrt.FastEnumerationSelector,
			[]ir.Value{state, items, fl.blk.I64.Const(objcrt.FastEnumerationBatch)},
			[]types.Type{ptrT, ptrT, nsuint}, false, nsuint, s.Pos())
		nv, _ := n.(ir.I64)
		return nv
	}

	// The first batch, and the mutation count it establishes.
	n := ask()
	if fl.blk == nil {
		return
	}
	fl.blk.I64.Store(n, count)
	fl.blk.I64.Store(fl.blk.I64.Const(0), idx)
	fl.blk.BrIf(fl.blk.I64.Eq(n, fl.blk.I64.Const(0)), done.To(), first.To())
	fl.blk = first
	mp := fl.blk.Ptr.Load(fl.blk.Ptr.Add(state, fl.blk.I64.Const(mutOff)))
	fl.blk.I64.Store(fl.blk.I64.Load(mp), mutations)
	fl.blk.Br(inner.To())

	// Another element of this batch, or the next batch.
	fl.blk = inner
	fl.blk.BrIf(fl.blk.I64.ULt(fl.blk.I64.Load(idx), fl.blk.I64.Load(count)), body.To(), batch.To())

	fl.blk = batch
	n = ask()
	if fl.blk == nil {
		return
	}
	fl.blk.I64.Store(n, count)
	fl.blk.I64.Store(fl.blk.I64.Const(0), idx)
	fl.blk.BrIf(fl.blk.I64.Eq(n, fl.blk.I64.Const(0)), done.To(), inner.To())

	fl.blk = body
	mp = fl.blk.Ptr.Load(fl.blk.Ptr.Add(state, fl.blk.I64.Const(mutOff)))
	fl.blk.BrIf(fl.blk.I64.Eq(fl.blk.I64.Load(mp), fl.blk.I64.Load(mutations)), live.To(), mutated.To())
	fl.blk = mutated
	fl.emitCall(fl.u.objcImport(objcrt.EnumerationMutation, ir.NewSig().Param(ir.TypePtr)), fl.blk.Ptr.Load(collSlot))
	if fl.blk != nil {
		fl.blk.Br(live.To())
	}
	fl.blk = live
	list := fl.blk.Ptr.Load(fl.blk.Ptr.Add(state, fl.blk.I64.Const(itemsOff)))
	elem := fl.blk.Ptr.Load(fl.blk.Ptr.Add(list, fl.blk.I64.Mul(fl.blk.I64.Load(idx), fl.blk.I64.Const(8))))
	fl.objcAssignElem(slot, elem, elemT)

	fl.loop(fl.blk, next, done, s.Body, next)

	fl.blk = next
	fl.blk.I64.Store(fl.blk.I64.Add(fl.blk.I64.Load(idx), fl.blk.I64.Const(1)), idx)
	fl.blk.Br(inner.To())
	fl.blk = done
}

// objcAssignElem stores an enumerated object into the loop variable, as
// an assignment to it would.
func (fl *fn) objcAssignElem(slot ir.Ptr, elem ir.Ptr, t types.Type) {
	switch ownership(t) {
	case types.QObjCStrong:
		v := fl.objcRetain(elem, t)
		old := fl.blk.Ptr.Load(slot)
		fl.blk.Ptr.Store(v.(ir.Ptr), slot)
		fl.objcRelease(old)
	case types.QObjCWeak:
		fl.objcCall1(objcrt.StoreWeak, slot, elem)
	default:
		fl.store(slot, elem, t)
	}
}

// objcEndMethod is what ARC adds where a method's body runs out: -dealloc
// ends by sending dealloc to super, which a program under ARC may not
// write.
func (fl *fn) objcEndMethod() {
	m := fl.u.objc.methods[fl.sym]
	if m == nil || m.isClass || m.method.Selector != "dealloc" || m.class.Super == nil {
		return
	}
	fl.destroyFrom(1)
	st, ok := fl.objcReceiver(&ast.ObjCSuperExpr{}, true)
	if !ok {
		return
	}
	fl.objcSend(st, true, "dealloc", nil, nil, false, types.Typ(types.Void), fl.sym.SymPos)
}

// ---- @try ----

// objcTry lowers @try, @catch and @finally.
//
// The @catch clauses are a try's handlers, answering by the classes'
// EH type-infos, and the objc runtime's begin and end catch bracket each
// one. The @finally block is a scope object around all of it: every way
// out -- falling off the end, return, break, continue, a handler's own
// exception unwinding through -- runs it where the scope is left, and an
// exception no @catch takes is caught by a catch-all, which runs the
// block and throws the exception on.
func (fl *fn) objcTry(s *ast.ObjCTryStmt) {
	finDepth := -1
	if s.Finally != nil {
		fl.pushScope()
		finDepth = len(fl.scopes) - 1
		top := fl.scopes[finDepth]
		top.objs = append(top.objs, localObj{arc: arcFinally, fin: s.Finally, depth: finDepth})
		defer fl.popScope()
	}
	var clauses []ir.PadClause
	catchAll := false
	for _, c := range s.Catches {
		ti := fl.u.objcEHType(fl.u.objc.info.CatchParams[c])
		if ti == nil {
			catchAll = true
		}
		clauses = append(clauses, ir.Catch(ti))
	}
	rethrowArm := s.Finally != nil && !catchAll
	if rethrowArm {
		clauses = append(clauses, ir.Catch(nil))
	}
	if len(clauses) == 0 {
		fl.stmt(s.Body)
		return
	}
	fl.needsEH()
	pad := fl.f.Pad(fmt.Sprintf("objc_catch_%d", fl.nextPad()), clauses...)
	dispatch := fl.block("objc_catch_dispatch")
	exn := dispatch.ParamPtr("exn")
	sel := dispatch.ParamI32("sel")
	pad.Br(dispatch.To(pad.Exn(), pad.Sel()))
	t := &tryFrame{clauses: clauses, dispatch: dispatch, base: pad, exn: exn, sel: sel, depth: len(fl.scopes)}
	done := fl.block("objc_try_done")

	fl.tries = append(fl.tries, t)
	fl.pushScope()
	fl.stmt(s.Body)
	fl.popScope()
	fl.tries = fl.tries[:len(fl.tries)-1]
	if fl.blk != nil {
		fl.blk.Br(done.To())
	}

	begin := fl.u.objcImport(objcrt.BeginCatch, ir.NewSig().Param(ir.TypePtr).Ret(ir.TypePtr))
	end := fl.u.objcImport(objcrt.EndCatch, ir.NewSig())
	fl.blk = dispatch
	for i, c := range s.Catches {
		hit := fl.block(fmt.Sprintf("objc_catch_hit_%d", fl.nextPad()))
		next := fl.block(fmt.Sprintf("objc_catch_next_%d", fl.nextPad()))
		fl.blk.BrIf(fl.blk.I32.Eq(sel, fl.blk.I32.Const(int64(i+1))), hit.To(), next.To())
		fl.blk = hit
		obj := fl.blk.Call(begin, exn).Ptr(0)
		fl.pushScope()
		fl.catchDepth++
		fl.catchEnds = append(fl.catchEnds, end)
		if sym := fl.u.objc.info.CatchParams[c]; sym != nil {
			slot := fl.allocVar(sym)
			fl.slots[sym] = slot
			fl.blk.Ptr.Store(obj, slot)
		}
		fl.stmt(c.Body)
		fl.catchDepth--
		fl.catchEnds = fl.catchEnds[:len(fl.catchEnds)-1]
		fl.popScope()
		if fl.blk != nil {
			fl.emitCall(end)
			if fl.blk != nil {
				fl.blk.Br(done.To())
			}
		}
		fl.blk = next
	}
	if rethrowArm {
		// Nothing caught it: the @finally block, then on it goes.
		fl.blk.Call(begin, exn)
		saved := fl.scopes
		fl.scopes = fl.scopes[:finDepth:finDepth]
		fl.destroyObj(localObj{arc: arcFinally, fin: s.Finally, depth: finDepth})
		fl.emitCall(fl.u.objcImport(objcrt.ExceptionRethrow, ir.NewSig()))
		if fl.blk != nil {
			fl.blk.Trap()
			fl.blk = nil
		}
		fl.scopes = saved
	} else {
		fl.blk.Resume(exn)
	}
	fl.blk = done
}

// objcEHType is the type-info a @catch parameter catches by: its class's
// OBJC_EHTYPE, id's for id, and nil for `@catch (...)`.
func (u *unit) objcEHType(param *sema.VarSymbol) ir.Symbol {
	if param == nil {
		return nil
	}
	c := types.ObjCClassOf(param.SymType)
	if c == nil || c.Builtin {
		return u.objcClassSymbol(objcrt.EHTypeID)
	}
	name := objcrt.EHTypeSymbol(c.Name)
	if s, ok := u.objc.classSyms[name]; ok {
		return s
	}
	if !c.Implemented {
		return u.objcClassSymbol(name)
	}
	// A class this unit implements has its type-info here: Itanium's
	// shape, the vtable pointer at the objc runtime's table plus two
	// slots, then the name and the class.
	g := u.mod.Global(u.symbolName(name), ir.RO, u.objcMetaType("ehtype", objcrt.EHType).FType()).
		Export().Weak().
		Section(u.objc.abi.Name(objcrt.SecConst)).
		Align(8).
		Init(ir.List(
			ir.RelocInit(u.objcClassSymbol(objcrt.EHTypeVTable)).Plus(ir.Int(objcrt.EHTypeVTableOffset)),
			ir.RelocInit(u.objcClassName(c.Name)),
			ir.RelocInit(u.objcClassSymbol(objcrt.ClassSymbol(c.Name)))))
	u.objc.classSyms[name] = g
	return g
}
