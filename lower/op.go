package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// binary lowers binary operators. Operands are converted to a common type first.
func (fl *fn) binary(e *ast.BinaryExpr) ir.Value {
	switch e.Op {
	case token.LAND, token.LOR:
		return fl.shortCircuit(e)

	case token.COMMA:
		// Left operand is evaluated for side effects and discarded.
		if fl.expr(e.X) == nil && fl.blk == nil {
			return nil
		}
		return fl.expr(e.Y)

	case token.SPACESHIP:
		return fl.spaceship(e)
	}

	lt, rt := fl.typeOf(e.X), fl.typeOf(e.Y)

	// Pointer arithmetic: `p + n`, `p - n`, and `n + p`.
	if isPointerish(lt) && isInteger(rt) && (e.Op == token.ADD || e.Op == token.SUB) {
		return fl.pointerArith(e, e.X, e.Y, lt, rt)
	}
	if isInteger(lt) && isPointerish(rt) && e.Op == token.ADD {
		return fl.pointerArith(e, e.Y, e.X, rt, lt)
	}
	if isPointerish(lt) && isPointerish(rt) && e.Op == token.SUB {
		// Pointer difference computes element count from byte distance.
		l, r := fl.value(e.X, lt, lt), fl.value(e.Y, rt, rt)
		if l == nil || r == nil {
			return nil
		}
		lp, lok := l.(ir.Ptr)
		rp, rok := r.(ir.Ptr)
		if !lok || !rok {
			return nil
		}
		var elem types.Type
		switch t := types.Unqualify(types.RemoveReference(lt)).(type) {
		case *types.Pointer:
			elem = t.Elem
		case *types.Array:
			elem = t.Elem
		}
		size, _ := fl.u.sizeAlign(elem)
		diff := fl.blk.Ptr.Diff(lp, rp)
		if size > 1 {
			diff = fl.blk.I64.SDiv(diff, fl.blk.I64.Const(size))
		}
		return diff
	}

	common := fl.commonType(e, lt, rt)

	l := fl.value(e.X, lt, common)
	if l == nil {
		return nil
	}
	r := fl.value(e.Y, rt, common)
	if r == nil {
		return nil
	}

	return fl.arith(e.Op, e.Pos(), l, r, common)
}

// value lowers an operand and converts it to the target type, decaying arrays.
func (fl *fn) value(e ast.Expr, from, to types.Type) ir.Value {
	if _, isArr := types.Unqualify(from).(*types.Array); isArr {
		slot, _, ok := fl.lvalue(e)
		if !ok {
			return nil
		}
		return slot
	}
	v := fl.expr(e)
	if v == nil {
		return nil
	}
	return fl.convert(v, from, to)
}

// pointerArith is `p + n` and `p - n`, scaled by what p points at.
func (fl *fn) pointerArith(e *ast.BinaryExpr, pExpr, nExpr ast.Expr, pType, nType types.Type) ir.Value {
	base, elem, ok := fl.pointerBase(pExpr, pType)
	if !ok {
		return nil
	}
	n := fl.expr(nExpr)
	if n == nil {
		return nil
	}
	if e.Op == token.SUB {
		// Subtracting n elements is adding -n elements.
		off := fl.convert(n, nType, types.Typ(types.LongLong))
		i64, isI64 := off.(ir.I64)
		if !isI64 {
			fl.u.errorf(e.Pos(), "lowering: a pointer offset that is not an integer")
			return nil
		}
		n = fl.blk.I64.Sub(fl.blk.I64.Const(0), i64)
		nType = types.Typ(types.LongLong)
	}
	return fl.scaled(base, n, nType, elem)
}

// pointerBase is the address an operand denotes and what it points at.
func (fl *fn) pointerBase(e ast.Expr, t types.Type) (ir.Ptr, types.Type, bool) {
	if arr, isArr := types.Unqualify(t).(*types.Array); isArr {
		slot, _, ok := fl.lvalue(e)
		if !ok {
			return ir.Ptr{}, nil, false
		}
		return slot, arr.Elem, true
	}
	ptr, isPtr := types.Unqualify(t).(*types.Pointer)
	if !isPtr {
		return ir.Ptr{}, nil, false
	}
	v := fl.expr(e)
	if v == nil {
		return ir.Ptr{}, nil, false
	}
	p, isP := v.(ir.Ptr)
	if !isP {
		return ir.Ptr{}, nil, false
	}
	return p, ptr.Elem, true
}

func isPointerish(t types.Type) bool {
	switch types.Unqualify(t).(type) {
	case *types.Pointer, *types.Array:
		return true
	}
	return false
}

func isInteger(t types.Type) bool { return types.IsInteger(types.Unqualify(t)) }

// arith applies one binary operator to two values of a common type.
func (fl *fn) arith(op token.Kind, at ast.Tok, l, r ir.Value, common types.Type) ir.Value {
	signed := isSigned(common)

	switch li := l.(type) {
	case ir.I32:
		if ri, ok := r.(ir.I32); ok {
			return fl.binaryI32(op, at, li, ri, signed)
		}
	case ir.I64:
		if ri, ok := r.(ir.I64); ok {
			return fl.binaryI64(op, at, li, ri, signed)
		}
	case ir.F64:
		if ri, ok := r.(ir.F64); ok {
			return fl.binaryF64(op, at, li, ri)
		}
	case ir.F32:
		if ri, ok := r.(ir.F32); ok {
			return fl.binaryF32(op, at, li, ri)
		}
	case ir.Ptr:
		if ri, ok := r.(ir.Ptr); ok {
			return fl.binaryPtr(op, at, li, ri)
		}
	}

	fl.u.errorf(at, "lowering does not handle %s on these operand types yet", op)
	return nil
}

func (fl *fn) binaryI32(op token.Kind, at ast.Tok, l, r ir.I32, signed bool) ir.Value {
	n := fl.blk.I32
	switch op {
	case token.ADD:
		return n.Add(l, r)
	case token.SUB:
		return n.Sub(l, r)
	case token.MUL:
		return n.Mul(l, r)
	case token.QUO:
		if signed {
			return n.SDiv(l, r)
		}
		return n.UDiv(l, r)
	case token.REM:
		if signed {
			return n.SRem(l, r)
		}
		return n.URem(l, r)
	case token.AND:
		return n.And(l, r)
	case token.OR:
		return n.Or(l, r)
	case token.XOR:
		return n.Xor(l, r)
	case token.SHL:
		return n.Shl(l, r)
	case token.SHR:
		if signed {
			return n.SShr(l, r)
		}
		return n.UShr(l, r)
	case token.EQL:
		return fl.fromBool(n.Eq(l, r))
	case token.NEQ:
		return fl.fromBool(n.Ne(l, r))
	case token.LSS:
		if signed {
			return fl.fromBool(n.SLt(l, r))
		}
		return fl.fromBool(n.ULt(l, r))
	case token.LEQ:
		if signed {
			return fl.fromBool(n.SLe(l, r))
		}
		return fl.fromBool(n.ULe(l, r))
	case token.GTR:
		// Lower `>` as `<` with swapped operands.
		if signed {
			return fl.fromBool(n.SLt(r, l))
		}
		return fl.fromBool(n.ULt(r, l))
	case token.GEQ:
		if signed {
			return fl.fromBool(n.SLe(r, l))
		}
		return fl.fromBool(n.ULe(r, l))
	}
	fl.u.errorf(at, "lowering does not handle %s yet", op)
	return nil
}

func (fl *fn) binaryI64(op token.Kind, at ast.Tok, l, r ir.I64, signed bool) ir.Value {
	n := fl.blk.I64
	switch op {
	case token.ADD:
		return n.Add(l, r)
	case token.SUB:
		return n.Sub(l, r)
	case token.MUL:
		return n.Mul(l, r)
	case token.QUO:
		if signed {
			return n.SDiv(l, r)
		}
		return n.UDiv(l, r)
	case token.REM:
		if signed {
			return n.SRem(l, r)
		}
		return n.URem(l, r)
	case token.AND:
		return n.And(l, r)
	case token.OR:
		return n.Or(l, r)
	case token.XOR:
		return n.Xor(l, r)
	case token.SHL:
		return n.Shl(l, r)
	case token.SHR:
		if signed {
			return n.SShr(l, r)
		}
		return n.UShr(l, r)
	case token.EQL:
		return fl.fromBool(n.Eq(l, r))
	case token.NEQ:
		return fl.fromBool(n.Ne(l, r))
	case token.LSS:
		if signed {
			return fl.fromBool(n.SLt(l, r))
		}
		return fl.fromBool(n.ULt(l, r))
	case token.LEQ:
		if signed {
			return fl.fromBool(n.SLe(l, r))
		}
		return fl.fromBool(n.ULe(l, r))
	case token.GTR:
		// Lower `>` as `<` with swapped operands.
		if signed {
			return fl.fromBool(n.SLt(r, l))
		}
		return fl.fromBool(n.ULt(r, l))
	case token.GEQ:
		if signed {
			return fl.fromBool(n.SLe(r, l))
		}
		return fl.fromBool(n.ULe(r, l))
	}
	fl.u.errorf(at, "lowering does not handle %s yet", op)
	return nil
}

func (fl *fn) binaryF64(op token.Kind, at ast.Tok, l, r ir.F64) ir.Value {
	n := fl.blk.F64
	switch op {
	case token.ADD:
		return n.Add(l, r)
	case token.SUB:
		return n.Sub(l, r)
	case token.MUL:
		return n.Mul(l, r)
	case token.QUO:
		return n.Div(l, r)
	case token.EQL:
		return fl.fromBool(n.Eq(l, r))
	case token.NEQ:
		return fl.fromBool(n.Ne(l, r))
	case token.LSS:
		return fl.fromBool(n.Lt(l, r))
	case token.LEQ:
		return fl.fromBool(n.Le(l, r))
	case token.GTR:
		return fl.fromBool(n.Lt(r, l))
	case token.GEQ:
		return fl.fromBool(n.Le(r, l))
	}
	fl.u.errorf(at, "lowering does not handle %s on doubles yet", op)
	return nil
}

// binaryF32 lowers binary operations on floats at single precision.
func (fl *fn) binaryF32(op token.Kind, at ast.Tok, l, r ir.F32) ir.Value {
	n := fl.blk.F32
	switch op {
	case token.ADD:
		return n.Add(l, r)
	case token.SUB:
		return n.Sub(l, r)
	case token.MUL:
		return n.Mul(l, r)
	case token.QUO:
		return n.Div(l, r)
	case token.EQL:
		return fl.fromBool(n.Eq(l, r))
	case token.NEQ:
		return fl.fromBool(n.Ne(l, r))
	case token.LSS:
		return fl.fromBool(n.Lt(l, r))
	case token.LEQ:
		return fl.fromBool(n.Le(l, r))
	case token.GTR:
		return fl.fromBool(n.Lt(r, l))
	case token.GEQ:
		return fl.fromBool(n.Le(r, l))
	}
	fl.u.errorf(at, "lowering does not handle %s on floats yet", op)
	return nil
}

// binaryPtr lowers pointer comparisons and differences.
func (fl *fn) binaryPtr(op token.Kind, at ast.Tok, l, r ir.Ptr) ir.Value {
	n := fl.blk.Ptr
	switch op {
	case token.EQL:
		return fl.fromBool(n.Eq(l, r))
	case token.NEQ:
		return fl.fromBool(n.Ne(l, r))
	case token.LSS:
		return fl.fromBool(n.Lt(l, r))
	case token.LEQ:
		return fl.fromBool(n.Le(l, r))
	case token.GTR:
		return fl.fromBool(n.Lt(r, l))
	case token.GEQ:
		return fl.fromBool(n.Le(r, l))
	case token.SUB:
		return n.Diff(l, r)
	}
	fl.u.errorf(at, "lowering does not handle %s on pointers yet", op)
	return nil
}

// fromBool widens a 1-bit comparison result to i32.
func (fl *fn) fromBool(c ir.I1) ir.Value {
	return fl.blk.I32.ZExtI1(c)
}

// shortCircuit lowers short-circuiting logical AND and OR.
func (fl *fn) shortCircuit(e *ast.BinaryExpr) ir.Value {
	slot := fl.entry.Ptr.Alloc(4, 4)

	l := fl.truth(e.X)
	if l == nil {
		return nil
	}

	rhs := fl.block("logic_rhs")
	join := fl.block("logic_join")
	short := fl.block("logic_short")

	if e.Op == token.LAND {
		fl.blk.BrIf(*l, rhs.To(), short.To())
	} else {
		fl.blk.BrIf(*l, short.To(), rhs.To())
	}

	// The side that decides: && is false, || is true.
	fl.blk = short
	decided := int64(0)
	if e.Op == token.LOR {
		decided = 1
	}
	fl.blk.I32.Store(fl.blk.I32.Const(decided), slot)
	fl.blk.Br(join.To())

	fl.blk = rhs
	r := fl.truth(e.Y)
	if r == nil {
		return nil
	}
	fl.blk.I32.Store(fl.blk.I32.ZExtI1(*r), slot)
	fl.blk.Br(join.To())

	fl.blk = join
	return fl.blk.I32.Load(slot)
}

// unary lowers prefix operators.
func (fl *fn) unary(e *ast.UnaryExpr) ir.Value {
	if e.Op == token.AND && fl.u.res.Info.MemberPointers[e] != nil {
		return fl.memberPointerValue(e)
	}
	switch e.Op {
	case token.INC, token.DEC:
		// Prefix form yields the new value.
		return fl.incDec(e.X, e.Op, e.Pos(), true)

	case token.AND:
		// Address-of yields the lvalue's slot address.
		slot, _, ok := fl.lvalue(e.X)
		if !ok {
			return nil
		}
		return slot

	case token.MUL:
		// Dereference pointer lvalue.
		slot, t, ok := fl.lvalue(e)
		if !ok {
			return nil
		}
		return fl.load(slot, t)
	}

	v := fl.expr(e.X)
	if v == nil {
		return nil
	}

	switch e.Op {
	case token.ADD:
		// Unary plus is identity after promotion.
		return v

	case token.NOT:
		c := fl.truth(e.X)
		if c == nil {
			return nil
		}
		return fl.fromBool(fl.blk.I1.Not(*c))

	case token.SUB:
		switch val := v.(type) {
		case ir.I32:
			return fl.blk.I32.Sub(fl.blk.I32.Const(0), val)
		case ir.I64:
			return fl.blk.I64.Sub(fl.blk.I64.Const(0), val)
		case ir.F64:
			return fl.blk.F64.Neg(val)
		case ir.F32:
			return fl.blk.F32.Neg(val)
		}

	case token.TILDE:
		switch val := v.(type) {
		case ir.I32:
			return fl.blk.I32.Not(val)
		case ir.I64:
			return fl.blk.I64.Not(val)
		}
	}

	fl.u.errorf(e.Pos(), "lowering does not handle unary %s yet", e.Op)
	return nil
}

// commonType determines the common type of operands for binary expressions.
func (fl *fn) commonType(e *ast.BinaryExpr, lt, rt types.Type) types.Type {
	switch e.Op {
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		if _, isMP := types.Unqualify(rt).(*types.MemberPointer); isMP {
			return rt
		}
		if _, isMP := types.Unqualify(lt).(*types.MemberPointer); isMP {
			return lt
		}
		if c := types.CommonType(lt, rt); c != nil {
			return c
		}
		return lt
	}
	if t := fl.typeOf(e); t != nil {
		return t
	}
	return lt
}

// compare lowers a relational operator to a 1-bit boolean condition.
func (fl *fn) compare(e *ast.BinaryExpr) (*ir.I1, bool) {
	lt, rt := fl.typeOf(e.X), fl.typeOf(e.Y)
	if classOf(lt) != nil || classOf(rt) != nil {
		// A class operand is an operator function's, never a built-in's.
		return nil, false
	}
	common := types.CommonType(lt, rt)
	if common == nil {
		common = lt
	}
	// A null pointer constant compared with a member pointer becomes a null member pointer.
	if _, isMP := types.Unqualify(rt).(*types.MemberPointer); isMP {
		common = rt
	} else if _, isMP := types.Unqualify(lt).(*types.MemberPointer); isMP {
		common = lt
	}

	l := fl.expr(e.X)
	if l == nil {
		return nil, false
	}
	l = fl.convert(l, lt, common)

	r := fl.expr(e.Y)
	if r == nil {
		return nil, false
	}
	r = fl.convert(r, rt, common)

	signed := isSigned(common)

	switch li := l.(type) {
	case ir.I32:
		ri, ok := r.(ir.I32)
		if !ok {
			return nil, false
		}
		return cmpI32(fl.blk.I32, e.Op, li, ri, signed)
	case ir.I64:
		ri, ok := r.(ir.I64)
		if !ok {
			return nil, false
		}
		return cmpI64(fl.blk.I64, e.Op, li, ri, signed)
	case ir.F64:
		ri, ok := r.(ir.F64)
		if !ok {
			return nil, false
		}
		return cmpF64(fl.blk.F64, e.Op, li, ri)
	case ir.F32:
		ri, ok := r.(ir.F32)
		if !ok {
			return nil, false
		}
		return cmpF32(fl.blk.F32, e.Op, li, ri)
	}
	return nil, false
}

func cmpI32(n ir.I32NS, op token.Kind, l, r ir.I32, signed bool) (*ir.I1, bool) {
	var c ir.I1
	switch op {
	case token.EQL:
		c = n.Eq(l, r)
	case token.NEQ:
		c = n.Ne(l, r)
	case token.LSS:
		if signed {
			c = n.SLt(l, r)
		} else {
			c = n.ULt(l, r)
		}
	case token.LEQ:
		if signed {
			c = n.SLe(l, r)
		} else {
			c = n.ULe(l, r)
		}
	case token.GTR:
		if signed {
			c = n.SLt(r, l)
		} else {
			c = n.ULt(r, l)
		}
	case token.GEQ:
		if signed {
			c = n.SLe(r, l)
		} else {
			c = n.ULe(r, l)
		}
	default:
		return nil, false
	}
	return &c, true
}

func cmpI64(n ir.I64NS, op token.Kind, l, r ir.I64, signed bool) (*ir.I1, bool) {
	var c ir.I1
	switch op {
	case token.EQL:
		c = n.Eq(l, r)
	case token.NEQ:
		c = n.Ne(l, r)
	case token.LSS:
		if signed {
			c = n.SLt(l, r)
		} else {
			c = n.ULt(l, r)
		}
	case token.LEQ:
		if signed {
			c = n.SLe(l, r)
		} else {
			c = n.ULe(l, r)
		}
	case token.GTR:
		if signed {
			c = n.SLt(r, l)
		} else {
			c = n.ULt(r, l)
		}
	case token.GEQ:
		if signed {
			c = n.SLe(r, l)
		} else {
			c = n.ULe(r, l)
		}
	default:
		return nil, false
	}
	return &c, true
}

func cmpF32(n ir.F32NS, op token.Kind, l, r ir.F32) (*ir.I1, bool) {
	var c ir.I1
	switch op {
	case token.EQL:
		c = n.Eq(l, r)
	case token.NEQ:
		c = n.Ne(l, r)
	case token.LSS:
		c = n.Lt(l, r)
	case token.LEQ:
		c = n.Le(l, r)
	case token.GTR:
		c = n.Lt(r, l)
	case token.GEQ:
		c = n.Le(r, l)
	default:
		return nil, false
	}
	return &c, true
}

func cmpF64(n ir.F64NS, op token.Kind, l, r ir.F64) (*ir.I1, bool) {
	var c ir.I1
	switch op {
	case token.EQL:
		c = n.Eq(l, r)
	case token.NEQ:
		c = n.Ne(l, r)
	case token.LSS:
		c = n.Lt(l, r)
	case token.LEQ:
		c = n.Le(l, r)
	case token.GTR:
		c = n.Lt(r, l)
	case token.GEQ:
		c = n.Le(r, l)
	default:
		return nil, false
	}
	return &c, true
}
