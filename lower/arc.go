package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/token"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/objcrt"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// Automatic reference counting, as clang's CodeGen does it: every .mm is
// compiled with ARC on.
//
// An expression's object value is either +0 -- borrowed, alive for as
// long as the full-expression -- or +1, owned. A +1 value (a message in
// the alloc, copy, init, mutableCopy or new family, a weak load, a
// bridge_transfer) is registered as a full-expression temporary like a
// C++ temporary with a destructor: whoever needs a +1 value takes it
// (objcTake), and what nobody takes is released when the full-expression
// ends, by the same machinery that destroys C++ temporaries -- on the
// unwinding path too.
//
// A __strong variable owns what it holds, and is released when its scope
// ends; a __weak one is registered with the runtime and unregistered
// then. Both are scope objects, so a return, a break or an exception
// releases them the way it destroys a C++ local.

// arcKind is what ARC does with a scope object.
type arcKind uint8

const (
	arcNone    arcKind = iota
	arcStrong          // release the pointer stored at addr
	arcWeak            // objc_destroyWeak(addr)
	arcPool            // objc_autoreleasePoolPop(*addr), on a normal exit only
	arcSync            // objc_sync_exit(*addr)
	arcFinally         // run an @finally block
	arcByref           // _Block_object_dispose(addr, BLOCK_FIELD_IS_BYREF)
)

// arcOn reports whether this function is compiled with ARC.
func (fl *fn) arcOn() bool { return fl.u.objc != nil }

// retainable reports whether ARC manages a value of type t. A class
// object, Class, is not retained: classes live forever, and a root class
// of the program's own need not answer retain at all.
func retainable(t types.Type) bool {
	t = types.RemoveReference(t)
	return types.IsObjCRetainable(t) && types.ObjCBase(types.ObjCClassOf(t)) != types.ObjCClassObject
}

// ownership is how ARC manages an object of type t: strong, weak,
// unsafe_unretained, autoreleasing, or 0 for a type it does not manage.
func ownership(t types.Type) types.Qual {
	if !retainable(t) {
		return 0
	}
	return types.Ownership(types.RemoveReference(t))
}

// destroyObj ends one scope object or temporary.
func (fl *fn) destroyObj(o localObj) {
	if fl.blk == nil {
		return
	}
	if o.arr != nil && o.arc != arcNone {
		fl.objcEndArray(o.addr, o.arr, o.arc)
		return
	}
	switch o.arc {
	case arcStrong:
		fl.objcRelease(fl.blk.Ptr.Load(o.addr))
		if o.val != nil {
			// An owned temporary's slot is used again by the next time
			// through a loop, and a branch not taken never set it: it is
			// null whenever it owns nothing.
			fl.blk.Ptr.Store(fl.blk.Ptr.Const(), o.addr)
		}
		return
	case arcWeak:
		fl.blk.Call(fl.u.objcImport(objcrt.DestroyWeak, ir.NewSig().Param(ir.TypePtr)), o.addr)
		return
	case arcPool:
		// clang pops a pool on the ways out of its block, and leaves one an
		// exception unwinds through to the pool around it.
		if !fl.inEH {
			fl.blk.Call(fl.u.objcImport(objcrt.AutoreleasePoolPop, ir.NewSig().Param(ir.TypePtr)), fl.blk.Ptr.Load(o.addr))
		}
		return
	case arcSync:
		fl.blk.Call(fl.u.objcImport(objcrt.SyncExit, ir.NewSig().Param(ir.TypePtr).Ret(ir.TypeI32)), fl.blk.Ptr.Load(o.addr))
		return
	case arcByref:
		fl.blk.Call(fl.u.objcImport(objcrt.BlockObjectDispose, ir.NewSig().Param(ir.TypePtr).Param(ir.TypeI32)), o.addr, fl.blk.I32.Const(objcrt.BlockFieldByref))
		return
	case arcFinally:
		// The block runs here as if written here, among the scopes
		// outside its @try only.
		saved := fl.scopes
		fl.scopes = fl.scopes[:o.depth:o.depth]
		fl.stmt(o.fin)
		fl.scopes = saved
		return
	}
	if o.arr != nil {
		fl.destroyElements(o.addr, o.arr)
		return
	}
	fl.destroy(o.addr, o.rec)
}

// ---- the runtime calls ----

func (fl *fn) objcRetain(v ir.Value, t types.Type) ir.Value {
	p, ok := v.(ir.Ptr)
	if !ok || fl.blk == nil {
		return v
	}
	name := objcrt.Retain
	if _, isBlock := types.Unqualify(types.RemoveReference(t)).(*types.BlockPointer); isBlock {
		// A block on the stack is copied to the heap by the retain.
		name = objcrt.RetainBlock
	} else if call := fl.justCalled(p); call != nil {
		// The return-value handshake: the marker after the call tells
		// objc_autoreleaseReturnValue in the callee to hand the object
		// over instead of autoreleasing it.
		call.Meta(ir.Attached(ir.AttachObjCReturnMarker))
		name = objcrt.RetainAutoreleasedReturnValue
	}
	f := fl.u.objcImport(name, ir.NewSig().Param(ir.TypePtr).Ret(ir.TypePtr))
	return fl.blk.Call(f, p).Ptr(0)
}

func (fl *fn) objcRelease(v ir.Value) {
	p, ok := v.(ir.Ptr)
	if !ok || fl.blk == nil {
		return
	}
	fl.blk.Call(fl.u.objcImport(objcrt.Release, ir.NewSig().Param(ir.TypePtr)), p)
}

func (fl *fn) objcCall1(name string, args ...ir.Value) ir.Ptr {
	sig := ir.NewSig()
	for range args {
		sig = sig.Param(ir.TypePtr)
	}
	return fl.blk.Call(fl.u.objcImport(name, sig.Ret(ir.TypePtr)), args...).Ptr(0)
}

// justCalled is the call p is the result of, when that call is the last
// thing emitted -- where the return-value marker may still go.
func (fl *fn) justCalled(p ir.Ptr) *ir.Inst {
	d := p.Def()
	if d == nil || fl.blk == nil {
		return nil
	}
	in := d.Inst()
	if in == nil || in != fl.blk.Last() {
		return nil
	}
	switch in.Op().Verb {
	case ir.VCall, ir.VCallInd:
		return in
	}
	return nil
}

// ---- owned temporaries ----

// objcOwn registers a +1 value as a full-expression temporary.
func (fl *fn) objcOwn(v ir.Value) ir.Value {
	p, ok := v.(ir.Ptr)
	if !ok || fl.blk == nil {
		return v
	}
	slot := fl.objcTempSlot()
	fl.blk.Ptr.Store(p, slot)
	fl.temps = append(fl.temps, localObj{addr: slot, arc: arcStrong, val: p})
	return v
}

// objcTempSlot is a slot for an owned temporary, null from the start of
// the function so that releasing it before it is set releases nothing.
func (fl *fn) objcTempSlot() ir.Ptr {
	slot := fl.entry.Ptr.Alloc(8, 8)
	fl.entry.Ptr.Store(fl.entry.Ptr.Const(), slot)
	return slot
}

// objcTake takes ownership of v from the temporaries: true where v was a
// +1 temporary, which is now the caller's to release.
func (fl *fn) objcTake(v ir.Value) bool {
	if v == nil {
		return false
	}
	for i := len(fl.temps) - 1; i >= 0; i-- {
		if fl.temps[i].arc == arcStrong && fl.temps[i].val == v {
			fl.temps = append(fl.temps[:i], fl.temps[i+1:]...)
			return true
		}
	}
	return false
}

// objcRetained is e's value at +1: taken from the temporaries where it is
// owned already, retained where it is borrowed.
func (fl *fn) objcRetained(e ast.Expr, to types.Type) ir.Value {
	v, from, converted := fl.convertedScalar(e)
	if !converted {
		v, from = fl.expr(e), fl.typeOf(e)
	}
	if v == nil {
		return nil
	}
	v = fl.convert(v, from, to)
	if fl.objcTake(v) {
		return v
	}
	return fl.objcRetain(v, to)
}

// ---- storage ----

// objcInitLocal initializes an ARC-managed variable at slot and makes it
// a scope object; false where t is not one ARC manages.
func (fl *fn) objcInitLocal(slot ir.Ptr, t types.Type, init ast.Expr) bool {
	if elem, k := arcArrayElem(t); k != arcNone {
		// An array of object pointers owns each element: nil to begin
		// with, and released last first when its scope ends.
		size, _ := fl.u.sizeAlign(t)
		fl.blk.MemSet(slot, fl.blk.I32.Const(0), fl.blk.I64.Const(size))
		_ = elem
		top := fl.scopes[len(fl.scopes)-1]
		top.objs = append(top.objs, localObj{addr: slot, arr: t, arc: k})
		if init != nil {
			return false // the array's initializer, element by element
		}
		return true
	}
	switch ownership(t) {
	case types.QObjCStrong:
		fl.objcInitAt(slot, t, init)
		fl.trackARC(slot, arcStrong)
		return true
	case types.QObjCWeak:
		fl.objcInitAt(slot, t, init)
		fl.trackARC(slot, arcWeak)
		return true
	case types.QObjCAutoreleasing:
		var v ir.Value = fl.blk.Ptr.Const()
		if init != nil {
			v = fl.objcAutoreleased(init, t)
		}
		if p, ok := v.(ir.Ptr); ok && fl.blk != nil {
			fl.blk.Ptr.Store(p, slot)
		}
		return true
	}
	return false
}

// trackARC makes an ARC variable a scope object.
func (fl *fn) trackARC(slot ir.Ptr, k arcKind) {
	if len(fl.scopes) == 0 {
		return
	}
	top := fl.scopes[len(fl.scopes)-1]
	top.objs = append(top.objs, localObj{addr: slot, arc: k})
}

// objcValue is e's value at +0, converted to t.
func (fl *fn) objcValue(e ast.Expr, t types.Type) ir.Value {
	v, from, converted := fl.convertedScalar(e)
	if !converted {
		v, from = fl.expr(e), fl.typeOf(e)
	}
	if v == nil {
		return nil
	}
	return fl.convert(v, from, t)
}

// objcAutoreleased is e's value put in the autorelease pool, which is how
// an __autoreleasing location -- an out-parameter -- holds one.
func (fl *fn) objcAutoreleased(e ast.Expr, t types.Type) ir.Value {
	v := fl.objcValue(e, t)
	if v == nil || fl.blk == nil {
		return nil
	}
	if fl.objcTake(v) {
		return fl.objcCall1(objcrt.Autorelease, v)
	}
	return fl.objcCall1(objcrt.RetainAutorelease, v)
}

// objcAssignExpr lowers `lhs = rhs` to an ARC-managed location: the value
// first, then the location, then the store; false where lhs is not
// ARC-managed.
func (fl *fn) objcAssignExpr(e *ast.AssignExpr) (ir.Value, bool) {
	t := fl.typeOf(e.Lhs)
	var v ir.Value
	switch ownership(t) {
	case types.QObjCStrong:
		v = fl.objcRetained(e.Rhs, types.RemoveReference(t))
	case types.QObjCWeak:
		v = fl.objcValue(e.Rhs, types.RemoveReference(t))
	case types.QObjCAutoreleasing:
		v = fl.objcAutoreleased(e.Rhs, types.RemoveReference(t))
	default:
		return nil, false
	}
	if v == nil || fl.blk == nil {
		return nil, true
	}
	slot, lt, ok := fl.lvalue(e.Lhs)
	if !ok {
		return nil, true
	}
	switch ownership(lt) {
	case types.QObjCStrong:
		old := fl.blk.Ptr.Load(slot)
		fl.blk.Ptr.Store(v.(ir.Ptr), slot)
		fl.objcRelease(old)
	case types.QObjCWeak:
		return fl.objcCall1(objcrt.StoreWeak, slot, v), true
	default:
		fl.blk.Ptr.Store(v.(ir.Ptr), slot)
	}
	return v, true
}

// objcLoad reads an ARC-managed location as an rvalue: a weak reference
// through the runtime, at +1 until the full-expression ends; false for
// any other, which is an ordinary load.
func (fl *fn) objcLoad(slot ir.Ptr, t types.Type) (ir.Value, bool) {
	if ownership(t) != types.QObjCWeak {
		return nil, false
	}
	return fl.objcOwn(fl.objcCall1(objcrt.LoadWeakRetained, slot)), true
}

// ---- functions ----

// objcParam makes a parameter an ARC variable on entry: a strong one is
// retained and released on exit -- except self, which a method borrows,
// and which an init method was handed at +1 and owns already.
func (fl *fn) objcParam(p *sema.VarSymbol, slot ir.Ptr) {
	if !fl.arcOn() || fl.blk == nil || ownership(p.SymType) != types.QObjCStrong {
		if fl.arcOn() && ownership(p.SymType) == types.QObjCWeak {
			v := fl.blk.Ptr.Load(slot)
			fl.blk.Call(fl.u.objcImport(objcrt.InitWeak, ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr).Ret(ir.TypePtr)), slot, v)
			fl.trackARC(slot, arcWeak)
		}
		return
	}
	if m := fl.u.objc.methods[fl.sym]; m != nil && len(fl.sym.Params) > 0 && fl.sym.Params[0] == p {
		if m.method.Family == "init" && !m.isClass {
			fl.trackARC(slot, arcStrong)
		}
		return
	}
	if len(fl.sym.Params) > 1 && fl.sym.Params[1] == p && fl.u.objc.methods[fl.sym] != nil {
		return // _cmd
	}
	fl.blk.Ptr.Store(fl.objcRetain(fl.blk.Ptr.Load(slot), p.SymType).(ir.Ptr), slot)
	fl.trackARC(slot, arcStrong)
}

// objcReturnsRetained reports whether the function hands its object
// result back at +1: a method of a retaining family.
func (fl *fn) objcReturnsRetained() bool {
	if m := fl.u.objc.methods[fl.sym]; m != nil {
		return m.method.ReturnsRetained
	}
	return false
}

// objcReturnValue is a return statement's object value: at +1, and, for a
// function that returns at +0, autoreleased by the handshake once the
// function's scopes have ended (objcFinishReturn).
func (fl *fn) objcReturnValue(e ast.Expr, ret types.Type) ir.Value {
	return fl.objcRetained(e, ret)
}

// objcReturn returns a +1 object: as it is from a retaining method, and
// otherwise through objc_autoreleaseReturnValue as a tail call. The
// runtime looks for the caller's marker at its own return address, so
// only a tail call lets the handshake see it.
func (fl *fn) objcReturn(v ir.Value) {
	if fl.objcReturnsRetained() || v == nil {
		fl.blk.Return(v)
		return
	}
	fl.blk.TailCall(fl.u.objcImport(objcrt.AutoreleaseReturnValue, ir.NewSig().Param(ir.TypePtr).Ret(ir.TypePtr)), v)
}

// objcInitAt initializes an ARC-managed location that held nothing: a
// strong one with e at +1, a weak one registered with the runtime. False
// for a type ARC does not manage.
func (fl *fn) objcInitAt(slot ir.Ptr, t types.Type, init ast.Expr) bool {
	switch ownership(t) {
	case types.QObjCStrong:
		var v ir.Value = fl.blk.Ptr.Const()
		if init != nil {
			v = fl.objcRetained(init, t)
			if v == nil || fl.blk == nil {
				return true
			}
		}
		fl.blk.Ptr.Store(v.(ir.Ptr), slot)
		return true
	case types.QObjCWeak:
		var v ir.Value = fl.blk.Ptr.Const()
		if init != nil {
			v = fl.objcValue(init, t)
			if v == nil || fl.blk == nil {
				return true
			}
		}
		fl.blk.Call(fl.u.objcImport(objcrt.InitWeak, ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr).Ret(ir.TypePtr)), slot, v)
		return true
	}
	return false
}

// arcArrayElem is the element an array of ARC-managed pointers holds --
// through arrays of arrays -- and how it is ended; arcNone for any other
// type.
func arcArrayElem(t types.Type) (types.Type, arcKind) {
	arr, ok := types.Unqualify(t).(*types.Array)
	if !ok || arr.Incomplete {
		return nil, arcNone
	}
	for {
		inner, isArr := types.Unqualify(arr.Elem).(*types.Array)
		if !isArr {
			break
		}
		arr = inner
	}
	switch ownership(arr.Elem) {
	case types.QObjCStrong:
		return arr.Elem, arcStrong
	case types.QObjCWeak:
		return arr.Elem, arcWeak
	}
	return nil, arcNone
}

// objcEndArray ends every element of an array of ARC-managed pointers,
// last first.
func (fl *fn) objcEndArray(addr ir.Ptr, t types.Type, k arcKind) {
	size, _ := fl.u.sizeAlign(t)
	n := size / 8
	if n == 0 {
		return
	}
	i := fl.entry.Ptr.Alloc(8, 8)
	fl.blk.I64.Store(fl.blk.I64.Const(n), i)
	cond, body, done := fl.block("arc_array_cond"), fl.block("arc_array_body"), fl.block("arc_array_done")
	fl.blk.Br(cond.To())
	fl.blk = cond
	fl.blk.BrIf(fl.blk.I64.Eq(fl.blk.I64.Load(i), fl.blk.I64.Const(0)), done.To(), body.To())
	fl.blk = body
	k1 := fl.blk.I64.Sub(fl.blk.I64.Load(i), fl.blk.I64.Const(1))
	fl.blk.I64.Store(k1, i)
	at := fl.blk.Ptr.Add(addr, fl.blk.I64.Mul(k1, fl.blk.I64.Const(8)))
	fl.destroyObj(localObj{addr: at, arc: k})
	fl.blk.Br(cond.To())
	fl.blk = done
}

// arcMember is how ARC ends a member of type t -- an object pointer or an
// array of them -- or arcNone.
func arcMember(t types.Type) arcKind {
	switch ownership(elementOf(t)) {
	case types.QObjCStrong:
		return arcStrong
	case types.QObjCWeak:
		return arcWeak
	}
	return arcNone
}

// hasARCMembers reports whether a class holds an object ARC manages, in a
// member or a base: such a class is not trivial -- its implicit
// constructor nils the object, its copy retains it, its destructor
// releases it -- as clang makes it in Objective-C++.
func (u *unit) hasARCMembers(rec *types.Record) bool {
	if u.objc == nil {
		return false
	}
	for _, b := range rec.Bases {
		if br := classOf(b.Type); br != nil && u.hasARCMembers(br) {
			return true
		}
	}
	for _, f := range rec.Fields {
		if arcMember(f.Type) != arcNone {
			return true
		}
		if fr := classOf(elementOf(f.Type)); fr != nil && fr != rec && u.hasARCMembers(fr) {
			return true
		}
	}
	return false
}

// objcCopyMember copies one ARC-managed member: a strong one retained, a
// weak one registered anew -- into storage that held nothing (ctor) or
// replacing what it held.
func (fl *fn) objcCopyMember(dst, src ir.Ptr, t types.Type, ctor bool) {
	if arr, isArr := types.Unqualify(t).(*types.Array); isArr {
		elemSize, _ := fl.u.sizeAlign(arr.Elem)
		for i := int64(0); i < arr.Len; i++ {
			off := fl.blk.I64.Const(i * elemSize)
			fl.objcCopyMember(fl.blk.Ptr.Add(dst, off), fl.blk.Ptr.Add(src, off), arr.Elem, ctor)
		}
		return
	}
	switch ownership(t) {
	case types.QObjCStrong:
		v := fl.objcRetain(fl.blk.Ptr.Load(src), t)
		if ctor {
			fl.blk.Ptr.Store(v.(ir.Ptr), dst)
			return
		}
		old := fl.blk.Ptr.Load(dst)
		fl.blk.Ptr.Store(v.(ir.Ptr), dst)
		fl.objcRelease(old)
	case types.QObjCWeak:
		if ctor {
			fl.blk.Call(fl.u.objcImport(objcrt.CopyWeak, ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr)), dst, src)
			return
		}
		v := fl.objcCall1(objcrt.LoadWeakRetained, src)
		fl.objcCall1(objcrt.StoreWeak, dst, v)
		fl.objcRelease(v)
	}
}

// ---- pass-by-writeback ----

// writeback is an out-parameter's temporary and the strong variable its
// value goes back to once the call returns.
type writeback struct {
	tmp, to ir.Ptr
	t       types.Type
}

// objcOutArg is ARC's pass-by-writeback: `&x`, x strong, passed where the
// parameter is `T * __autoreleasing *`. The callee is handed a temporary
// holding x's value, and after the call x is assigned what the temporary
// then holds (objcWriteBack). False for any other argument.
func (fl *fn) objcOutArg(a ast.Expr, want types.Type) (ir.Value, bool) {
	p, ok := types.Unqualify(types.RemoveReference(want)).(*types.Pointer)
	if !ok || ownership(p.Elem) != types.QObjCAutoreleasing {
		return nil, false
	}
	u, ok := unparen(a).(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return nil, false
	}
	addr, t, ok := fl.lvalue(u.X)
	if !ok || ownership(t) != types.QObjCStrong {
		return nil, false
	}
	tmp := fl.alloc(types.ObjCId, "writeback")
	fl.blk.Ptr.Store(fl.blk.Ptr.Load(addr), tmp)
	fl.writebacks = append(fl.writebacks, writeback{tmp: tmp, to: addr, t: t})
	return tmp, true
}

// objcWriteBack assigns each out-parameter's temporary back to its
// variable, after the call that filled them.
func (fl *fn) objcWriteBack() {
	wbs := fl.writebacks
	fl.writebacks = nil
	if fl.blk == nil {
		return
	}
	for _, w := range wbs {
		v := fl.objcRetain(fl.blk.Ptr.Load(w.tmp), w.t)
		old := fl.blk.Ptr.Load(w.to)
		fl.blk.Ptr.Store(v.(ir.Ptr), w.to)
		fl.objcRelease(old)
	}
}
