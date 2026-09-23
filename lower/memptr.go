package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// Member pointers in the Microsoft representation for a single-inheritance class:
// data member pointer is member offset in 4 bytes (-1 when null);
// function member pointer is the function address. Multiple or virtual bases
// require wider representation forms.
//
// Under Itanium, data member pointers use ptrdiff_t offset and function member
// pointers include function address and this adjustment.

// memberPointerValue is `&C::m`.
func (fl *fn) memberPointerValue(e *ast.UnaryExpr) ir.Value {
	ref := fl.u.res.Info.MemberPointers[e]
	if ref == nil {
		return nil
	}
	if !fl.u.singleInheritance(ref.Class) {
		fl.u.errorf(e.Pos(), "lowering: a pointer to member of a class with multiple or virtual bases is not handled yet")
		return nil
	}
	if ref.Func != nil {
		if ref.Func.Virtual {
			// [itanium-abi 2.3]: a pointer to a virtual function holds
			// the slot's byte offset in the table plus one, rather than
			// an address. A function is never at an odd address, so the
			// low bit tells the two apart, and the call site reads it.
			tableOff, slot, found := fl.u.vslot(ref.Func)
			if !found {
				fl.u.errorf(e.Pos(), "lowering found no table slot for %s::%s", ref.Class.Name, ref.Func.SymName)
				return nil
			}
			if tableOff != 0 {
				fl.u.errorf(e.Pos(), "lowering: a pointer to a virtual member of a secondary base is not handled yet")
				return nil
			}
			return fl.blk.Ptr.FromI64(fl.blk.I64.Const(int64(slot)*fl.u.model.SizePtr + 1))
		}
		target := fl.u.callee(ref.Func)
		if target == nil {
			fl.u.errorf(e.Pos(), "lowering has no symbol for %s::%s", ref.Class.Name, ref.Func.SymName)
			return nil
		}
		return fl.blk.Ptr.GetAddr(target)
	}
	off, _, ok := fl.u.fieldOffset(ref.Class, ref.Field)
	if !ok {
		fl.u.errorf(e.Pos(), "lowering found no member %q in %s", ref.Field, ref.Class.Name)
		return nil
	}
	if fl.u.model.ABI.IsItanium() {
		return fl.blk.I64.Const(off)
	}
	return fl.blk.I32.Const(off)
}

// singleInheritance is whether a class has at most one chain of
// non-virtual bases, so that a member pointer needs no adjustment.
func (u *unit) singleInheritance(rec *types.Record) bool {
	for rec != nil {
		if len(rec.Bases) > 1 {
			return false
		}
		if len(rec.Bases) == 0 {
			return true
		}
		if rec.Bases[0].Virtual {
			return false
		}
		rec = classOf(rec.Bases[0].Type)
	}
	return true
}

// memberPointerObject is the object `obj.*pm` or `p->*pm` reaches into.
func (fl *fn) memberPointerObject(e *ast.BinaryExpr) (ir.Ptr, bool) {
	if e.Op == token.ARROW_STAR {
		v := fl.expr(e.X)
		if v == nil {
			return ir.Ptr{}, false
		}
		p, isPtr := v.(ir.Ptr)
		if !isPtr {
			fl.u.errorf(e.Pos(), "lowering: the left operand of ->* is not a pointer")
			return ir.Ptr{}, false
		}
		return p, true
	}
	return fl.objectOf(e.X)
}

// memberPointerAddr is the address of the data member `obj.*pm` names.
func (fl *fn) memberPointerAddr(e *ast.BinaryExpr) (ir.Ptr, types.Type, bool) {
	mp, isMP := types.Unqualify(fl.typeOf(e.Y)).(*types.MemberPointer)
	if !isMP || types.IsFunc(mp.Elem) {
		fl.u.errorf(e.Pos(), "lowering: %s on something that is not a pointer to data member", e.Op)
		return ir.Ptr{}, nil, false
	}
	obj, ok := fl.memberPointerObject(e)
	if !ok {
		return ir.Ptr{}, nil, false
	}
	off := fl.expr(e.Y)
	if off == nil {
		return ir.Ptr{}, nil, false
	}
	switch o := off.(type) {
	case ir.I32:
		return fl.blk.Ptr.Add(obj, fl.blk.I64.SExtI32(o)), mp.Elem, true
	case ir.I64:
		return fl.blk.Ptr.Add(obj, o), mp.Elem, true
	}
	fl.u.errorf(e.Pos(), "lowering: a data member pointer that is not an offset")
	return ir.Ptr{}, nil, false
}

// memberPointerCall is `(obj.*pmf)(args)`: an indirect call with the
// object as its first argument.
func (fl *fn) memberPointerCall(c *ast.CallExpr, bin *ast.BinaryExpr) ir.Value {
	mp, isMP := types.Unqualify(fl.typeOf(bin.Y)).(*types.MemberPointer)
	if !isMP {
		return nil
	}
	ft, isFunc := types.Unqualify(mp.Elem).(*types.Func)
	if !isFunc {
		return nil
	}
	obj, ok := fl.memberPointerObject(bin)
	if !ok {
		return nil
	}
	target := fl.expr(bin.Y)
	if target == nil {
		return nil
	}
	targetPtr, isPtr := target.(ir.Ptr)
	if !isPtr {
		fl.u.errorf(c.Pos(), "lowering: a function member pointer that is not an address")
		return nil
	}
	// The call's shape is a member function's: `this` first, the hidden
	// result where the ABI puts it.
	rec := classOf(mp.Class)
	callee := &sema.FuncSymbol{SymName: "member", FuncType: ft, InClass: rec}
	irType := fl.u.funcType(callee)
	args := make([]ir.Value, 0, len(c.Args)+2)
	var result ir.Ptr
	retRec := classOf(ft.Ret)
	hiddenAfterThis := false
	if retRec != nil {
		if fl.resultInto != (ir.Ptr{}) {
			result = fl.resultInto
			fl.resultInto = ir.Ptr{}
		} else {
			result = fl.alloc(retRec, "")
			fl.temporary(result, retRec)
		}
		hiddenAfterThis = !fl.u.plainForReturn(retRec) && fl.u.model.ABI.ResultAfterThis()
		if !hiddenAfterThis {
			args = append(args, result)
		}
	}
	args = append(args, obj)
	if hiddenAfterThis {
		args = append(args, result)
	}
	for i, a := range c.Args {
		var want types.Type
		if i < len(ft.Params) {
			want = ft.Params[i].Type
		}
		if r := classOf(want); r != nil {
			addr, ok := fl.objectOf(a)
			if !ok {
				return nil
			}
			if !fl.u.plainForCalls(r) {
				tmp := fl.alloc(r, "")
				if !fl.copyObject(tmp, addr, r, nil) {
					return nil
				}
				addr = tmp
			}
			args = append(args, addr)
			continue
		}
		if isReference(want) {
			addr, ok := fl.bind(a, want)
			if !ok {
				return nil
			}
			args = append(args, addr)
			continue
		}
		v := fl.expr(a)
		if v == nil {
			return nil
		}
		args = append(args, fl.convert(v, fl.typeOf(a), want))
	}
	targetPtr = fl.memberFuncTarget(targetPtr, obj)
	res := fl.blk.CallInd(targetPtr, irType, args...)
	if retRec != nil {
		return result
	}
	if res.Len() == 0 {
		return nil
	}
	return res.Value(0)
}

// memberFuncTarget is the function a pointer to member function names for
// an object: what it holds, or -- where the low bit is set -- the entry
// its value less one indexes in the object's table ([itanium-abi 2.3]).
func (fl *fn) memberFuncTarget(pmf ir.Ptr, obj ir.Ptr) ir.Ptr {
	b := fl.blk
	raw := b.I64.FromPtr(pmf)
	virt := b.I64.Ne(b.I64.And(raw, b.I64.Const(1)), b.I64.Const(0))

	viaTable := fl.block("pmf_virtual")
	direct := fl.block("pmf_direct")
	join := fl.block("pmf_join")
	target := join.ParamPtr("target")
	b.BrIf(virt, viaTable.To(), direct.To())

	vptr := viaTable.Ptr.Load(obj)
	entry := viaTable.Ptr.Add(vptr, viaTable.I64.Sub(raw, viaTable.I64.Const(1)))
	viaTable.Br(join.To(viaTable.Ptr.Load(entry)))

	direct.Br(join.To(pmf))

	fl.blk = join
	return target
}
