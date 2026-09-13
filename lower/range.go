package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// invoke calls a function the analysis resolved where there is no call
// expression to lower: the begin() of a range-based for, the operator++
// of its iterator. obj is the object for a member function (and is nil
// for a free one); args are the explicit arguments, an address for a
// reference or class parameter and a value otherwise; into is where a
// class result is built, or zero for a temporary of the full-expression.
// The result is the class result's address, the scalar result, or nil.
func (fl *fn) invoke(fn *sema.FuncSymbol, obj ir.Ptr, args []ir.Value, into ir.Ptr, at ast.Tok) ir.Value {
	target := fl.u.callee(fn)
	if target == nil {
		fl.u.errorf(at, "lowering has no symbol for %q", fn.SymName)
		return nil
	}
	member := fn.InClass != nil && !fn.Static
	callArgs := make([]ir.Value, 0, len(args)+2)

	// The same shape callRaw gives a call: the hidden result first, or
	// after `this` where the Microsoft convention puts it.
	var result ir.Ptr
	retRec := classOf(fn.FuncType.Ret)
	hiddenAfterThis := false
	if retRec != nil {
		if into != (ir.Ptr{}) {
			result = into
		} else {
			result = fl.alloc(retRec, "")
			fl.temporary(result, retRec)
		}
		hiddenAfterThis = !fl.u.plainForReturn(retRec) && fl.u.model.ABI.ResultAfterThis() && member
		if !hiddenAfterThis {
			callArgs = append(callArgs, result)
		}
	}
	if member {
		if off := fl.u.thisOffset(fn); off != 0 {
			obj = fl.blk.Ptr.Add(obj, fl.blk.I64.Const(off))
		}
		callArgs = append(callArgs, obj)
	}
	if hiddenAfterThis {
		callArgs = append(callArgs, result)
	}
	for i, a := range args {
		if i < len(fn.FuncType.Params) {
			// A class passed by value is a copy the caller makes.
			if rec := classOf(fn.FuncType.Params[i].Type); rec != nil && !fl.u.plainForCalls(rec) {
				if src, isPtr := a.(ir.Ptr); isPtr {
					tmp := fl.alloc(rec, "")
					if !fl.copyObject(tmp, src, rec, nil) {
						return nil
					}
					if !fl.u.model.ABI.CalleeDestroysParameters() {
						fl.temporary(tmp, rec)
					}
					a = tmp
				}
			}
		}
		callArgs = append(callArgs, a)
	}
	res := fl.blk.Call(target, callArgs...)
	if retRec != nil {
		return result
	}
	if res.Len() == 0 {
		return nil
	}
	return res.Value(0)
}

// rangeForClass lowers a range-for loop over a class: begin() and end() iterators
// are kept in frame slots, and the loop compares, dereferences, and advances
// via chosen operators or built-ins.
func (fl *fn) rangeForClass(s *ast.RangeForStmt, proto *sema.RangeProtocol) {
	// `auto &&__range = range-initializer;` -- the range's object, which
	// lives for the whole statement when it is a temporary.
	rangeAddr, ok := fl.objectOf(s.Range)
	if !ok {
		return
	}
	fl.extendTemporary(rangeAddr)

	sd, isSimple := s.Decl.(*ast.SimpleDecl)
	if !isSimple || len(sd.Inits) == 0 {
		fl.u.errorf(s.Pos(), "lowering: unsupported ranged loop declaration")
		return
	}
	sym := fl.localSymbol(sd.Inits[0])
	if sym == nil {
		fl.u.errorf(s.Pos(), "lowering: no symbol for ranged loop variable")
		return
	}

	iterT := proto.Iter
	iterRec := classOf(iterT)
	makeIter := func(fn *sema.FuncSymbol, name string) (ir.Ptr, bool) {
		slot := fl.alloc(iterT, name)
		var obj ir.Ptr
		var args []ir.Value
		if fn.InClass != nil && !fn.Static {
			obj = rangeAddr
		} else {
			args = []ir.Value{rangeAddr}
		}
		if iterRec != nil {
			if fl.invoke(fn, obj, args, slot, s.Range.Pos()) == nil {
				return ir.Ptr{}, false
			}
			fl.track(slot, iterT)
			return slot, true
		}
		v := fl.invoke(fn, obj, args, ir.Ptr{}, s.Range.Pos())
		if v == nil {
			return ir.Ptr{}, false
		}
		fl.store(slot, v, iterT)
		return slot, true
	}
	begin, ok := makeIter(proto.Begin, "__begin")
	if !ok {
		return
	}
	end, ok := makeIter(proto.End, "__end")
	if !ok {
		return
	}
	fl.endFullExpr()

	varSlot := fl.alloc(sym.SymType, sym.SymName)
	fl.slots[sym] = varSlot

	head := fl.block("range_head")
	body := fl.block("range_body")
	step := fl.block("range_step")
	exit := fl.block("range_exit")
	fl.blk.Br(head.To())

	// `__begin != __end`
	fl.blk = head
	var cond ir.I1
	if iterRec == nil {
		cond = fl.blk.Ptr.Ne(fl.blk.Ptr.Load(begin), fl.blk.Ptr.Load(end))
	} else {
		v := fl.invokeOperator(proto.NotEqual, begin, []ir.Value{end}, s.Range.Pos())
		if v == nil {
			return
		}
		c := fl.toBool(v, proto.NotEqual.FuncType.Ret)
		if proto.Negated {
			c = fl.blk.I1.Not(c)
		}
		cond = c
	}
	fl.endFullExpr()
	fl.blk.BrIf(cond, body.To(), exit.To())

	oldBreak, oldContinue, oldDepth := fl.breakTo, fl.continueTo, fl.loopDepth
	fl.breakTo, fl.continueTo, fl.loopDepth = exit, step, len(fl.scopes)

	// `decl = *__begin;`
	fl.blk = body
	fl.pushScope()
	var elemAddr ir.Ptr
	var elemVal ir.Value
	elemT := proto.Elem
	if iterRec == nil {
		elemAddr = fl.blk.Ptr.Load(begin)
	} else {
		v := fl.invokeOperator(proto.Deref, begin, nil, s.Range.Pos())
		if v == nil {
			return
		}
		if p, isPtr := v.(ir.Ptr); isPtr && (proto.ElemRef || classOf(elemT) != nil) {
			elemAddr = p
		} else {
			elemVal = v
		}
	}
	switch {
	case isReference(sym.SymType):
		if elemAddr == (ir.Ptr{}) {
			// A reference to a value *begin made: a temporary of the
			// iteration, which the reference's scope keeps alive.
			tmp := fl.alloc(elemT, "")
			fl.store(tmp, fl.convert(elemVal, elemT, types.RemoveReference(sym.SymType)), types.RemoveReference(sym.SymType))
			elemAddr = tmp
		}
		fl.blk.Ptr.Store(elemAddr, varSlot)
	case classOf(sym.SymType) != nil:
		if !fl.copyObject(varSlot, elemAddr, classOf(sym.SymType), nil) {
			return
		}
		fl.track(varSlot, sym.SymType)
	default:
		if elemVal == nil {
			elemVal = fl.load(elemAddr, elemT)
		}
		fl.store(varSlot, fl.convert(elemVal, elemT, sym.SymType), sym.SymType)
	}
	fl.endFullExpr()

	fl.stmt(s.Body)
	fl.popScope()
	if fl.blk != nil {
		fl.blk.Br(step.To())
	}
	fl.breakTo, fl.continueTo, fl.loopDepth = oldBreak, oldContinue, oldDepth

	// `++__begin`
	fl.blk = step
	if iterRec == nil {
		elemSize, _ := fl.u.sizeAlign(elemT)
		next := fl.blk.Ptr.Add(fl.blk.Ptr.Load(begin), fl.blk.I64.Const(elemSize))
		fl.blk.Ptr.Store(next, begin)
	} else {
		fl.invokeOperator(proto.Increment, begin, nil, s.Range.Pos())
		fl.endFullExpr()
	}
	fl.blk.Br(head.To())
	fl.blk = exit
}

// invokeOperator calls an operator function on an iterator: as a member
// on it, or as a free function with it as the first argument.
func (fl *fn) invokeOperator(fn *sema.FuncSymbol, iter ir.Ptr, rest []ir.Value, at ast.Tok) ir.Value {
	if fn.InClass != nil && !fn.Static {
		return fl.invoke(fn, iter, rest, ir.Ptr{}, at)
	}
	return fl.invoke(fn, ir.Ptr{}, append([]ir.Value{iter}, rest...), ir.Ptr{}, at)
}

// toBool is a scalar value as a condition, the way truth reads one.
func (fl *fn) toBool(v ir.Value, t types.Type) ir.I1 {
	b := fl.blk
	switch val := v.(type) {
	case ir.I1:
		return val
	case ir.I32:
		return b.I32.Ne(val, b.I32.Const(0))
	case ir.I64:
		return b.I64.Ne(val, b.I64.Const(0))
	case ir.Ptr:
		return b.Ptr.Ne(val, b.Ptr.Const())
	case ir.F64:
		return b.F64.Ne(val, b.F64.Const(0))
	case ir.F32:
		return b.F32.Ne(val, b.F32.Const(0))
	}
	fl.u.errorf(ast.NoTok, "lowering: a %q is not a condition", t)
	return b.I1.Const(false)
}
