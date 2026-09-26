package lower

import (
	"fmt"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/objcrt"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// Blocks, as Apple's runtime lays them out and clang compiles them.
//
// A block literal is a structure: an isa, flags, the invoke function the
// body was compiled into, a descriptor, and then the variables it
// captured. One that captures nothing is a constant in the image
// (_NSConcreteGlobalBlock); any other is built on the stack
// (_NSConcreteStackBlock), and copying it -- objc_retainBlock, which is
// what ARC does when it keeps one -- moves it to the heap, through the
// descriptor's copy helper, which retains what the block holds.
//
// The invoke function is the body as a function whose first parameter is
// the block: a captured variable is read at its field. A __block variable
// is not captured by value but shared: it lives in a byref structure
// whose forwarding pointer leads to the one live copy -- the stack one
// until a block holding it is copied, the heap one after -- and every use
// goes through it.

// blockLayout is where a block's captures sit.
type blockLayout struct {
	info   *sema.BlockInfo
	caps   []blockCap
	size   int64
	flags  objcrt.BlockFlag
	desc   ir.Symbol
	global ir.Symbol // the literal, for a block that captures nothing
}

// blockCap is one captured variable and its field.
type blockCap struct {
	v     *sema.VarSymbol
	off   int64
	byref bool
}

// byrefLayout is a __block variable's structure.
type byrefLayout struct {
	varOff  int64
	size    int64
	helpers bool
	keep    ir.Symbol
	destroy ir.Symbol
}

// blockOf is the block an invoke function belongs to, nil for any other
// function.
func (u *unit) blockOf(sym *sema.FuncSymbol) *sema.BlockInfo {
	if u.objc == nil {
		return nil
	}
	return u.objc.blocks[sym]
}

// layoutBlock places a block's captures after its header.
func (u *unit) layoutBlock(info *sema.BlockInfo) *blockLayout {
	if l, ok := u.objc.blockLayouts[info]; ok {
		return l
	}
	l := &blockLayout{info: info, flags: objcrt.BlockHasSignature}
	off := u.objc.abi.SizeOf(objcrt.BlockLiteral)
	for _, v := range info.Captures {
		if info.ByRef[v] {
			off = roundUp(off, 8)
			l.caps = append(l.caps, blockCap{v: v, off: off, byref: true})
			off += 8
			l.flags |= objcrt.BlockHasCopyDispose
			continue
		}
		size, align := u.sizeAlign(v.SymType)
		if isReference(v.SymType) {
			size, align = 8, 8
		}
		if size == 0 {
			size = 1
		}
		off = roundUp(off, align)
		l.caps = append(l.caps, blockCap{v: v, off: off})
		off += size
		if u.capNeedsHelpers(v.SymType) {
			l.flags |= objcrt.BlockHasCopyDispose
		}
	}
	l.size = roundUp(off, 8)
	if len(l.caps) == 0 {
		l.flags |= objcrt.BlockIsGlobal
	}
	u.objc.blockLayouts[info] = l
	return l
}

// capNeedsHelpers reports whether copying a block that holds a value of
// type t has more to do than copy its bytes.
func (u *unit) capNeedsHelpers(t types.Type) bool {
	if isReference(t) {
		return false
	}
	if ownership(t) == types.QObjCStrong || ownership(t) == types.QObjCWeak {
		return true
	}
	rec := classOf(t)
	return rec != nil && (u.destructor(rec) != nil || u.memberwise(rec, true))
}

// blockExpr lowers a block literal: the address of the block.
func (fl *fn) blockExpr(e *ast.BlockExpr) ir.Value {
	u := fl.u
	info := u.res.Info.Blocks[e]
	if info == nil {
		u.errorf(e.Pos(), "lowering found no block for this literal")
		return nil
	}
	invoke, ok := u.funcs[info.Invoke]
	if !ok {
		u.errorf(e.Pos(), "lowering has no invoke function for this block")
		return nil
	}
	l := u.layoutBlock(info)
	// The body's self, super and instance variables are this function's.
	if m := fl.objcContext(); m != nil {
		u.objc.blockOwner[info.Invoke] = m
	}
	desc := u.blockDescriptor(l)
	if len(l.caps) == 0 {
		if l.global == nil {
			l.global = u.mod.Global(u.objcName(objcrt.BlockLiteralLabel), ir.RO, u.objcMetaType("block_literal", objcrt.BlockLiteral).FType()).
				Internal().
				Section(u.objc.abi.Name(objcrt.SecBlockConst)).
				Align(8).
				Init(ir.List(
					ir.RelocInit(u.objcClassSymbol(objcrt.GlobalBlockClass)),
					ir.Lit(ir.Int(int64(l.flags))),
					ir.Lit(ir.Int(0)),
					ir.RelocInit(invoke),
					ir.RelocInit(desc)))
		}
		return fl.blk.Ptr.GetAddr(l.global)
	}

	b := fl.blk
	blk := fl.entry.Ptr.Alloc(uint64(l.size), 8)
	at := func(off int64) ir.Ptr { return fl.blk.Ptr.Add(blk, fl.blk.I64.Const(off)) }
	b.Ptr.Store(b.Ptr.GetAddr(u.objcClassSymbol(objcrt.StackBlockClass)), blk)
	b.I32.Store(b.I32.Const(int64(int32(l.flags))), at(8))
	b.I32.Store(b.I32.Const(0), at(12))
	b.Ptr.Store(b.Ptr.GetAddr(invoke), at(16))
	b.Ptr.Store(b.Ptr.GetAddr(desc), at(24))
	for _, c := range l.caps {
		field := at(c.off)
		if c.byref {
			bp, ok := fl.byrefs[c.v]
			if !ok {
				u.errorf(e.Pos(), "lowering found no __block storage for %q", c.v.SymName)
				continue
			}
			fl.blk.Ptr.Store(bp, field)
			continue
		}
		fl.captureInto(field, c.v, e.Pos())
	}
	return blk
}

// captureInto copies a variable into a stack block's field: an object
// retained and a weak reference registered, both ended with the
// full-expression, as the stack block is; a class copied; anything else
// its bytes.
func (fl *fn) captureInto(field ir.Ptr, v *sema.VarSymbol, at ast.Tok) {
	src, ok := fl.slots[v]
	if !ok {
		fl.u.errorf(at, "lowering found no storage for the captured %q", v.SymName)
		return
	}
	t := v.SymType
	if isReference(t) {
		fl.blk.Ptr.Store(fl.blk.Ptr.Load(src), field)
		return
	}
	switch ownership(t) {
	case types.QObjCStrong:
		fl.blk.Ptr.Store(fl.objcRetain(fl.blk.Ptr.Load(src), t).(ir.Ptr), field)
		fl.temps = append(fl.temps, localObj{addr: field, arc: arcStrong})
		return
	case types.QObjCWeak:
		fl.blk.Call(fl.u.objcImport(objcrt.CopyWeak, ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr)), field, src)
		fl.temps = append(fl.temps, localObj{addr: field, arc: arcWeak})
		return
	}
	if rec := classOf(t); rec != nil {
		fl.copyObject(field, src, rec, nil)
		fl.temporary(field, rec)
		return
	}
	size, _ := fl.u.sizeAlign(t)
	fl.blk.MemCpy(field, src, fl.blk.I64.Const(size))
}

// blockDescriptor is a block's descriptor: its size, the copy and dispose
// helpers where it needs them, and its signature.
func (u *unit) blockDescriptor(l *blockLayout) ir.Symbol {
	if l.desc != nil {
		return l.desc
	}
	sig := l.info.Type.Func
	enc := u.objc.abi.BlockTypes(sig.Ret, sig.Params, u.model)
	items := []ir.Init{ir.Lit(ir.Int(0)), ir.Lit(ir.Int(l.size))}
	fields := objcrt.BlockDescriptor
	if l.flags&objcrt.BlockHasCopyDispose != 0 {
		fields = objcrt.BlockDescriptorWithHelpers
		items = append(items, ir.RelocInit(u.blockCopyHelper(l)), ir.RelocInit(u.blockDisposeHelper(l)))
	}
	items = append(items, ir.RelocInit(u.objcMethodType(enc)), ir.Lit(ir.Int(0)))
	name := "block_descriptor"
	if l.flags&objcrt.BlockHasCopyDispose != 0 {
		name = "block_descriptor_helpers"
	}
	l.desc = u.mod.Global(u.objcName(objcrt.BlockDescriptorLabel), ir.RO, u.objcMetaType(name, fields).FType()).
		Internal().
		Section(u.objc.abi.Name(objcrt.SecBlockConst)).
		Align(8).
		Init(ir.List(items...))
	return l.desc
}

// helperFunc is a helper the runtime calls with n pointers, and its first
// block.
func (u *unit) helperFunc(prefix string, n int) (*ir.Func, *ir.Block, []ir.Ptr) {
	f := u.mod.Func(u.objcName(prefix)).Internal()
	var ps []ir.Ptr
	for i := 0; i < n; i++ {
		ps = append(ps, f.ParamPtr(fmt.Sprintf("p%d", i)))
	}
	return f, f.Entry(), ps
}

// blockCopyHelper is what _Block_copy calls after moving the block's
// bytes to the heap: each capture that owns something takes its own
// reference.
func (u *unit) blockCopyHelper(l *blockLayout) ir.Symbol {
	f, b, ps := u.helperFunc("__copy_helper_block_", 2)
	dst, src := ps[0], ps[1]
	for _, c := range l.caps {
		d := b.Ptr.Add(dst, b.I64.Const(c.off))
		s := b.Ptr.Add(src, b.I64.Const(c.off))
		assign := u.objcImport(objcrt.BlockObjectAssign, ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypeI32))
		switch {
		case c.byref:
			b.Call(assign, d, b.Ptr.Load(s), b.I32.Const(objcrt.BlockFieldByref))
		case isReference(c.v.SymType):
		case ownership(c.v.SymType) == types.QObjCStrong:
			if _, isBlock := types.Unqualify(c.v.SymType).(*types.BlockPointer); isBlock {
				b.Call(assign, d, b.Ptr.Load(s), b.I32.Const(objcrt.BlockFieldBlock))
				continue
			}
			v := b.Call(u.objcImport(objcrt.Retain, ir.NewSig().Param(ir.TypePtr).Ret(ir.TypePtr)), b.Ptr.Load(s)).Ptr(0)
			b.Ptr.Store(v, d)
		case ownership(c.v.SymType) == types.QObjCWeak:
			b.Call(u.objcImport(objcrt.CopyWeak, ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr)), d, s)
		default:
			if rec := classOf(c.v.SymType); rec != nil && u.memberwise(rec, true) {
				fl := &fn{u: u, f: f, blk: b, entry: b, slots: map[sema.Symbol]ir.Ptr{}, sym: &sema.FuncSymbol{SymName: "__copy_helper_block_"}}
				fl.copyObject(d, s, rec, nil)
				b = fl.blk
			}
		}
	}
	b.Return()
	return f
}

// blockDisposeHelper is what _Block_release calls before freeing a heap
// block: each capture gives up what it owns.
func (u *unit) blockDisposeHelper(l *blockLayout) ir.Symbol {
	f, b, ps := u.helperFunc("__destroy_helper_block_", 1)
	blk := ps[0]
	for _, c := range l.caps {
		at := b.Ptr.Add(blk, b.I64.Const(c.off))
		dispose := u.objcImport(objcrt.BlockObjectDispose, ir.NewSig().Param(ir.TypePtr).Param(ir.TypeI32))
		switch {
		case c.byref:
			b.Call(dispose, b.Ptr.Load(at), b.I32.Const(objcrt.BlockFieldByref))
		case isReference(c.v.SymType):
		case ownership(c.v.SymType) == types.QObjCStrong:
			if _, isBlock := types.Unqualify(c.v.SymType).(*types.BlockPointer); isBlock {
				b.Call(dispose, b.Ptr.Load(at), b.I32.Const(objcrt.BlockFieldBlock))
				continue
			}
			b.Call(u.objcImport(objcrt.Release, ir.NewSig().Param(ir.TypePtr)), b.Ptr.Load(at))
		case ownership(c.v.SymType) == types.QObjCWeak:
			b.Call(u.objcImport(objcrt.DestroyWeak, ir.NewSig().Param(ir.TypePtr)), at)
		default:
			if rec := classOf(c.v.SymType); rec != nil && u.destructor(rec) != nil {
				b.Call(u.destructor(rec), at)
			}
		}
	}
	b.Return()
	return f
}

// enterBlock binds an invoke function's captures to their fields: a
// by-value capture is its field, a __block one the byref structure the
// field points to.
func (fl *fn) enterBlock(info *sema.BlockInfo) {
	args := fl.u.params[fl.sym]
	if len(args) == 0 {
		return
	}
	blk, ok := args[0].(ir.Ptr)
	if !ok {
		return
	}
	for _, c := range fl.u.layoutBlock(info).caps {
		field := fl.blk.Ptr.Add(blk, fl.blk.I64.Const(c.off))
		if c.byref {
			bp := fl.blk.Ptr.Load(field)
			fl.byrefs[c.v] = bp
			fl.slots[c.v] = bp
			continue
		}
		if isReference(c.v.SymType) {
			// The field holds the address the reference is bound to.
			fl.slots[c.v] = field
			continue
		}
		fl.slots[c.v] = field
	}
}

// ---- calls ----

// blockCall lowers a call through a block: its invoke function, with the
// block itself first.
func (fl *fn) blockCall(e *ast.CallExpr, bp *types.BlockPointer) ir.Value {
	v, ok := fl.expr(e.Fun).(ir.Ptr)
	if !ok || fl.blk == nil {
		return nil
	}
	ft := &types.Func{Ret: bp.Func.Ret, Variadic: bp.Func.Variadic}
	ft.Params = append(ft.Params, types.Param{Type: &types.Pointer{Elem: types.Typ(types.Void)}})
	var params []types.Type
	for _, p := range bp.Func.Params {
		ft.Params = append(ft.Params, p)
		params = append(params, p.Type)
	}
	fnType := fl.u.funcType(&sema.FuncSymbol{SymName: "block_invoke", FuncType: ft})
	vals, ok := fl.objcArgs(e.Args, params)
	if !ok {
		return nil
	}
	invoke := fl.blk.Ptr.Load(fl.blk.Ptr.Add(v, fl.blk.I64.Const(16)))
	args := []ir.Value{v}
	var result ir.Ptr
	retRec := classOf(bp.Func.Ret)
	if retRec != nil {
		result = fl.alloc(retRec, "")
		if fl.resultInto != (ir.Ptr{}) {
			result = fl.resultInto
			fl.resultInto = ir.Ptr{}
		}
		args = append([]ir.Value{result}, args...)
	}
	args = append(args, vals...)
	res := fl.emitCallInd(invoke, fnType, args...)
	if len(fl.writebacks) > 0 {
		fl.objcWriteBack()
	}
	if retRec != nil {
		return result
	}
	if len(res) == 0 {
		return nil
	}
	return res[0]
}

// ---- __block variables ----

// layoutByref places a __block variable after its structure's header.
func (u *unit) layoutByref(v *sema.VarSymbol) *byrefLayout {
	if l, ok := u.objc.byrefLayouts[v]; ok {
		return l
	}
	l := &byrefLayout{helpers: u.capNeedsHelpers(v.SymType)}
	header := u.objc.abi.SizeOf(objcrt.BlockByref)
	if l.helpers {
		header = u.objc.abi.SizeOf(objcrt.BlockByrefWithHelpers)
	}
	size, align := u.sizeAlign(v.SymType)
	l.varOff = roundUp(header, align)
	l.size = roundUp(l.varOff+size, 8)
	if l.helpers {
		l.keep, l.destroy = u.byrefHelpers(v, l)
	}
	u.objc.byrefLayouts[v] = l
	return l
}

// byrefHelpers are a byref structure's keep and destroy: moving the
// variable to the heap copy, and ending it there.
func (u *unit) byrefHelpers(v *sema.VarSymbol, l *byrefLayout) (ir.Symbol, ir.Symbol) {
	keep, kb, kps := u.helperFunc("__Block_byref_object_copy_", 2)
	dst := kb.Ptr.Add(kps[0], kb.I64.Const(l.varOff))
	src := kb.Ptr.Add(kps[1], kb.I64.Const(l.varOff))
	switch ownership(v.SymType) {
	case types.QObjCStrong:
		// Moved: the stack copy is never read again.
		kb.Ptr.Store(kb.Ptr.Load(src), dst)
		kb.Ptr.Store(kb.Ptr.Const(), src)
	case types.QObjCWeak:
		kb.Call(u.objcImport("objc_moveWeak", ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr)), dst, src)
	default:
		size, _ := u.sizeAlign(v.SymType)
		kb.MemCpy(dst, src, kb.I64.Const(size))
	}
	kb.Return()

	destroy, db, dps := u.helperFunc("__Block_byref_object_dispose_", 1)
	at := db.Ptr.Add(dps[0], db.I64.Const(l.varOff))
	switch ownership(v.SymType) {
	case types.QObjCStrong:
		db.Call(u.objcImport(objcrt.Release, ir.NewSig().Param(ir.TypePtr)), db.Ptr.Load(at))
	case types.QObjCWeak:
		db.Call(u.objcImport(objcrt.DestroyWeak, ir.NewSig().Param(ir.TypePtr)), at)
	default:
		if rec := classOf(v.SymType); rec != nil && u.destructor(rec) != nil {
			db.Call(u.destructor(rec), at)
		}
	}
	db.Return()
	return keep, destroy
}

// declareByref declares a __block variable: its structure on the stack,
// forwarding to itself, the variable initialized in it, and a scope
// object that gives the structure up when the scope ends.
func (fl *fn) declareByref(sym *sema.VarSymbol, init *ast.InitDeclarator) {
	l := fl.u.layoutByref(sym)
	st := fl.entry.Ptr.Alloc(uint64(l.size), 8)
	b := fl.blk
	b.MemSet(st, b.I32.Const(0), b.I64.Const(l.size))
	b.Ptr.Store(st, b.Ptr.Add(st, b.I64.Const(8)))
	flags := int64(0)
	if l.helpers {
		flags |= int64(objcrt.BlockByrefHasCopyDispose)
	}
	b.I32.Store(b.I32.Const(flags), b.Ptr.Add(st, b.I64.Const(16)))
	b.I32.Store(b.I32.Const(l.size), b.Ptr.Add(st, b.I64.Const(20)))
	if l.helpers {
		b.Ptr.Store(b.Ptr.GetAddr(l.keep), b.Ptr.Add(st, b.I64.Const(24)))
		b.Ptr.Store(b.Ptr.GetAddr(l.destroy), b.Ptr.Add(st, b.I64.Const(32)))
	}
	fl.byrefs[sym] = st
	fl.slots[sym] = st
	top := fl.scopes[len(fl.scopes)-1]
	top.objs = append(top.objs, localObj{addr: st, arc: arcByref})

	at := b.Ptr.Add(st, b.I64.Const(l.varOff))
	t := sym.SymType
	if fl.arcOn() && fl.objcInitAt(at, t, init.Value) {
		return
	}
	if init.Value == nil {
		return
	}
	if rec := classOf(t); rec != nil {
		fl.exprInto(at, init.Value, rec)
		return
	}
	v, from, converted := fl.convertedScalar(init.Value)
	if !converted {
		v, from = fl.expr(init.Value), fl.typeOf(init.Value)
	}
	if v != nil {
		fl.store(at, fl.convert(v, from, t), t)
	}
}

// objcByrefAddr is where a __block variable's value is: through the
// forwarding pointer of the structure this function reaches it by.
func (fl *fn) objcByrefAddr(e ast.Expr) (ir.Ptr, types.Type, bool, bool) {
	switch e.(type) {
	case *ast.Ident, *ast.QualifiedName:
	default:
		return ir.Ptr{}, nil, false, false
	}
	v, ok := fl.u.res.Info.Uses[e].(*sema.VarSymbol)
	if !ok || !v.ByRefBlock {
		return ir.Ptr{}, nil, false, false
	}
	st, ok := fl.byrefs[v]
	if !ok {
		return ir.Ptr{}, nil, false, false
	}
	l := fl.u.layoutByref(v)
	fwd := fl.blk.Ptr.Load(fl.blk.Ptr.Add(st, fl.blk.I64.Const(8)))
	return fl.blk.Ptr.Add(fwd, fl.blk.I64.Const(l.varOff)), v.SymType, true, true
}
