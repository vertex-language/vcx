package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// expr lowers an expression to a value. Returns nil if lowering failed.
func (fl *fn) expr(e ast.Expr) ir.Value {
	if e == nil || fl.blk == nil {
		return nil
	}
	// Rewritten comparison tree naming the resolved operator function.
	if r := fl.u.res.Info.Rewrites[e]; r != nil {
		return fl.expr(r)
	}

	switch e := e.(type) {
	case *ast.BasicLit:
		return fl.basicLit(e)

	case *ast.ParenExpr:
		return fl.expr(e.X)

	case *ast.Ident, *ast.QualifiedName:
		// Folded constant values.
		if n, known := fl.u.res.Info.Consts[e]; known && classOf(fl.typeOf(e)) == nil {
			if fl.u.regType(fl.typeOf(e)) == ir.TypeI64 {
				return fl.blk.I64.Const(n)
			}
			return fl.blk.I32.Const(n)
		}
		return fl.rvalue(e)

	case *ast.BinaryExpr:
		if op := fl.u.res.Info.Operators[e]; op != nil {
			return fl.operatorValue(e, op, e.X, []ast.Expr{e.Y})
		}
		if e.Op == token.PERIOD_STAR || e.Op == token.ARROW_STAR {
			// Data member value; bound member functions are only called.
			addr, t, ok := fl.memberPointerAddr(e)
			if !ok {
				return nil
			}
			return fl.load(addr, t)
		}
		return fl.binary(e)

	case *ast.UnaryExpr:
		if op := fl.u.res.Info.Operators[e]; op != nil {
			return fl.operatorValue(e, op, e.X, nil)
		}
		return fl.unary(e)

	case *ast.AssignExpr:
		if op := fl.u.res.Info.Operators[e]; op != nil {
			return fl.operatorValue(e, op, e.Lhs, []ast.Expr{e.Rhs})
		}
		return fl.assign(e)

	case *ast.CallExpr:
		return fl.call(e)

	case *ast.CastExpr:
		return fl.convert(fl.expr(e.X), fl.typeOf(e.X), fl.typeOf(e))

	case *ast.NamedCastExpr:
		return fl.convert(fl.expr(e.X), fl.typeOf(e.X), fl.typeOf(e))

	case *ast.CondExpr:
		return fl.conditional(e)

	case *ast.LambdaExpr:
		return fl.lambda(e)

	case *ast.IndexExpr:
		if op := fl.u.res.Info.Operators[e]; op != nil {
			return fl.operatorValue(e, op, e.X, e.Args)
		}
		return fl.rvalue(e)

	case *ast.MemberExpr:
		return fl.rvalue(e)

	case *ast.IncDecExpr:
		if op := fl.u.res.Info.Operators[e]; op != nil {
			return fl.operatorValue(e, op, e.X, nil, fl.blk.I32.Const(0))
		}
		// Postfix inc/dec yields copy of old value.
		return fl.incDec(e.X, e.Op, e.Pos(), false)

	case *ast.StringLit:
		return fl.stringLit(e)

	case *ast.FunctionalCastExpr:
		return fl.functionalCast(e)

	case *ast.SizeofExpr, *ast.AlignofExpr:
		// sizeof and alignof constants.
		return fl.sizeofExpr(e)

	case *ast.TypeTraitExpr:
		return fl.typeTrait(e)

	case *ast.TemplateName:
		return fl.templateId(e)

	case *ast.NewExpr:
		return fl.newExpr(e)

	case *ast.DeleteExpr:
		fl.deleteExpr(e)
		return nil

	case *ast.ThisExpr:
		// Implicit this parameter.
		if !fl.hasThis {
			fl.u.errorf(e.Pos(), "lowering: `this` outside a member function")
			return nil
		}
		return fl.this

	default:
		fl.u.errorf(e.Pos(), "lowering does not handle %T yet", e)
		return nil
	}
}

// basicLit lowers literal values using the constant evaluator.
func (fl *fn) basicLit(e *ast.BasicLit) ir.Value {
	t := fl.typeOf(e)
	switch e.Kind {
	case token.TRUE:
		return fl.blk.I32.Const(1)
	case token.FALSE:
		return fl.blk.I32.Const(0)
	case token.NULLPTR:
		return fl.blk.Ptr.Const()
	}

	if e.Kind == token.FLOAT_LIT {
		f, err := fl.u.evalFloat(e)
		if err != nil {
			fl.u.errorf(e.Pos(), "lowering could not evaluate the literal: %v", err)
			return nil
		}
		if fl.u.regType(t) == ir.TypeF32 {
			return fl.blk.F32.Const(f)
		}
		return fl.blk.F64.Const(f)
	}

	n, err := fl.u.evalInt(e)
	if err != nil {
		fl.u.errorf(e.Pos(), "lowering could not evaluate the literal: %v", err)
		return nil
	}
	if fl.u.regType(t) == ir.TypeI64 {
		return fl.blk.I64.Const(n)
	}
	return fl.blk.I32.Const(n)
}

// rvalue loads the value of an expression from its designated address.
func (fl *fn) rvalue(e ast.Expr) ir.Value {
	slot, t, ok := fl.lvalue(e)
	if !ok {
		return nil
	}
	if _, isArr := types.Unqualify(t).(*types.Array); isArr {
		return slot
	}
	if _, isFn := types.Unqualify(t).(*types.Func); isFn {
		return slot
	}
	if classOf(t) != nil {
		return slot
	}
	return fl.load(slot, t)
}

// lvalue returns the storage address and type for an lvalue expression.
func (fl *fn) lvalue(e ast.Expr) (ir.Ptr, types.Type, bool) {
	if op := fl.u.res.Info.Operators[e]; op != nil && isReference(op.FuncType.Ret) {
		return fl.operatorAddr(e)
	}
	switch e := e.(type) {
	case *ast.ParenExpr:
		return fl.lvalue(e.X)

	case *ast.Ident, *ast.QualifiedName:
		sym := fl.u.res.Info.Uses[e]
		if fn, isFn := sym.(*sema.FuncSymbol); isFn {
			callee := fl.u.callee(fn)
			if callee == nil {
				fl.u.errorf(e.Pos(), "lowering has no symbol for %q", fn.SymName)
				return ir.Ptr{}, nil, false
			}
			return fl.blk.Ptr.GetAddr(callee), fn.FuncType, true
		}
		v, isVar := sym.(*sema.VarSymbol)
		if !isVar {
			fl.u.errorf(e.Pos(), "lowering does not handle this name as an object yet")
			return ir.Ptr{}, nil, false
		}
		if slot, known := fl.slots[v]; known {
			return fl.throughRef(slot, v.SymType)
		}
		// Structured binding element or member at its offset.
		if v.Binding != nil && v.Binding.Get == nil {
			return fl.bindingAddr(v)
		}
		// Not a local, so it is a namespace-scope object: its storage is a
		// symbol rather than a frame slot, and its address is taken the same
		// way a function's is.
		if g, known := fl.u.globalFor(v); known {
			return fl.throughRef(fl.blk.Ptr.GetAddr(g), v.SymType)
		}
		// Inside a member function, a non-static member implicitly accesses (*this).name.
		if fl.hasThis && fl.sym.InClass != nil {
			if off, t, ok := fl.u.fieldOffset(fl.sym.InClass, v.SymName); ok {
				base := fl.this
				if off != 0 {
					base = fl.blk.Ptr.Add(base, fl.blk.I64.Const(off))
				}
				if bf, isBit := fl.u.bitFieldNamed(fl.sym.InClass, v.SymName); isBit {
					fl.noteBitField(base, bf)
				}
				// A reference member -- a lambda's by-reference capture --
				// holds the address of what it names.
				return fl.throughRef(base, t)
			}
		}
		fl.u.errorf(e.Pos(), "lowering found no storage for %q", v.SymName)
		return ir.Ptr{}, nil, false

	case *ast.MemberExpr:
		// Member access: o.m or p->m computed with member offset.
		return fl.memberAddr(e)

	case *ast.IndexExpr:
		// Subscript a[i] is *(a + i), scaled by element size.
		if len(e.Args) == 0 {
			fl.u.errorf(e.Pos(), "lowering: a subscript with no index")
			return ir.Ptr{}, nil, false
		}
		base, elem, ok := fl.arrayBase(e.X)
		if !ok {
			return ir.Ptr{}, nil, false
		}
		idx := fl.expr(e.Args[0])
		if idx == nil {
			return ir.Ptr{}, nil, false
		}
		return fl.scaled(base, idx, fl.typeOf(e.Args[0]), elem), elem, true

	case *ast.CallExpr:
		// A function returning a reference yields an lvalue address.
		callee := fl.u.res.Info.Calls[e]
		if callee != nil {
			if !isReference(callee.FuncType.Ret) {
				break
			}
			v := fl.callRaw(e, callee)
			p, isPtr := v.(ir.Ptr)
			if !isPtr {
				return ir.Ptr{}, nil, false
			}
			return p, types.RemoveReference(callee.FuncType.Ret), true
		}
		if v, ft, ok := fl.indirectCallRaw(e); ok && isReference(ft.Ret) {
			p, isPtr := v.(ir.Ptr)
			if !isPtr {
				return ir.Ptr{}, nil, false
			}
			return p, types.RemoveReference(ft.Ret), true
		}

	case *ast.BinaryExpr, *ast.AssignExpr, *ast.IncDecExpr:
		if bin, isBin := e.(*ast.BinaryExpr); isBin && (bin.Op == token.PERIOD_STAR || bin.Op == token.ARROW_STAR) {
			// Data member designated by a member pointer.
			return fl.memberPointerAddr(bin)
		}
		// An operator function returning a reference: the same, with
		// the operands as the call's.
		return fl.operatorAddr(e)

	case *ast.CondExpr:
		// When both conditional arms are lvalues, the result is an lvalue.
		return fl.conditionalAddr(e)

	case *ast.StringLit:
		// String literals are lvalue arrays in static storage.
		g, ok := fl.u.stringGlobal(e)
		if !ok {
			return ir.Ptr{}, nil, false
		}
		return fl.blk.Ptr.GetAddr(g), fl.typeOf(e), true

	case *ast.UnaryExpr:
		// Dereference *p designates the object p points at.
		if e.Op == token.MUL {
			v := fl.expr(e.X)
			if v == nil {
				return ir.Ptr{}, nil, false
			}
			ptr, isPtr := v.(ir.Ptr)
			if !isPtr {
				fl.u.errorf(e.Pos(), "lowering: the operand of unary * is not a pointer")
				return ir.Ptr{}, nil, false
			}
			return ptr, fl.typeOf(e), true
		}
	}

	fl.u.errorf(e.Pos(), "lowering does not handle %T as an lvalue yet", e)
	return ir.Ptr{}, nil, false
}

// memberAddr returns the storage address and type of a member access.
func (fl *fn) memberAddr(e *ast.MemberExpr) (ir.Ptr, types.Type, bool) {
	var base ir.Ptr
	var rec *types.Record

	if e.Op == token.ARROW {
		p, pointee, ok := fl.arrowBase(e)
		if !ok {
			return ir.Ptr{}, nil, false
		}
		base = p
		rec = classOf(pointee)
	} else {
		slot, ok := fl.objectOf(e.X)
		if !ok {
			return ir.Ptr{}, nil, false
		}
		base = slot
		rec = types.AsRecord(types.Unqualify(types.RemoveReference(fl.typeOf(e.X))))
	}

	if rec == nil {
		fl.u.errorf(e.Pos(), "lowering: the left operand is not a class")
		return ir.Ptr{}, nil, false
	}

	name := sema.NameString(e.Sel, fl.u.unit)
	off, field, ok := fl.u.fieldOffset(rec, name)
	if !ok {
		if v, isVar := fl.u.res.Info.Uses[e.Sel].(*sema.VarSymbol); isVar && v.InClass != nil {
			if g, known := fl.u.globalFor(v); known {
				return fl.throughRef(fl.blk.Ptr.GetAddr(g), v.SymType)
			}
		}
		fl.u.errorf(e.Pos(), "lowering found no member %q in %s", name, rec.Name)
		return ir.Ptr{}, nil, false
	}
	if off != 0 {
		base = fl.blk.Ptr.Add(base, fl.blk.I64.Const(off))
	}
	if bf, isBit := fl.u.bitFieldNamed(rec, name); isBit {
		fl.noteBitField(base, bf)
	}
	return fl.throughRef(base, field)
}

// arrayBase returns the base pointer and element type for a subscript expression.
func (fl *fn) arrayBase(e ast.Expr) (ir.Ptr, types.Type, bool) {
	t := types.RemoveReference(fl.typeOf(e))
	if arr, isArr := types.Unqualify(t).(*types.Array); isArr {
		slot, _, ok := fl.lvalue(e)
		if !ok {
			return ir.Ptr{}, nil, false
		}
		return slot, arr.Elem, true
	}

	v := fl.expr(e)
	if v == nil {
		return ir.Ptr{}, nil, false
	}
	p, isPtr := v.(ir.Ptr)
	if !isPtr {
		fl.u.errorf(e.Pos(), "lowering: the subscripted operand is neither an array nor a pointer")
		return ir.Ptr{}, nil, false
	}
	ptr, isPtrType := types.Unqualify(t).(*types.Pointer)
	if !isPtrType {
		fl.u.errorf(e.Pos(), "lowering: the subscripted operand has no element type")
		return ir.Ptr{}, nil, false
	}
	return p, ptr.Elem, true
}

// scaled offsets a pointer by n elements of the given element type.
func (fl *fn) scaled(base ir.Ptr, n ir.Value, nType, elem types.Type) ir.Ptr {
	size, _ := fl.u.sizeAlign(elem)
	off := fl.convert(n, nType, types.Typ(types.LongLong))
	i64, ok := off.(ir.I64)
	if !ok {
		fl.u.errorf(ast.NoTok, "lowering: a subscript index that is not an integer")
		return base
	}
	if size != 1 {
		i64 = fl.blk.I64.Mul(i64, fl.blk.I64.Const(size))
	}
	return fl.blk.Ptr.Add(base, i64)
}

// truth converts an expression to a boolean 1-bit branch condition.
func (fl *fn) truth(e ast.Expr) *ir.I1 {
	if cmp, ok := unparen(e).(*ast.BinaryExpr); ok && isComparison(cmp.Op) && fl.u.res.Info.Operators[cmp] == nil && fl.u.res.Info.Rewrites[cmp] == nil {
		if c, made := fl.compare(cmp); made {
			return c
		}
	}

	v := fl.expr(e)
	if v == nil {
		return nil
	}
	b := fl.blk
	switch val := v.(type) {
	case ir.I1:
		return &val
	case ir.I32:
		c := b.I32.Ne(val, b.I32.Const(0))
		return &c
	case ir.I64:
		c := b.I64.Ne(val, b.I64.Const(0))
		return &c
	case ir.F64:
		c := b.F64.Ne(val, b.F64.Const(0))
		return &c
	case ir.F32:
		c := b.F32.Ne(val, b.F32.Const(0))
		return &c
	case ir.Ptr:
		c := b.Ptr.Ne(val, b.Ptr.Const())
		return &c
	}
	fl.u.errorf(e.Pos(), "lowering cannot use this value as a condition")
	return nil
}

// unparen unwraps enclosing parentheses.
func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

func isComparison(k token.Kind) bool {
	switch k {
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		return true
	}
	return false
}

// assign stores into an lvalue and yields the value stored.
func (fl *fn) assign(e *ast.AssignExpr) ir.Value {
	slot, t, ok := fl.lvalue(e.Lhs)
	if !ok {
		return nil
	}
	if rec := classOf(t); rec != nil && e.Op == token.ASSIGN {
		// Copy assignment via user-defined or implicit copy assignment operator.
		src, ok := fl.objectOf(e.Rhs)
		if !ok {
			return nil
		}
		if !fl.assignObject(slot, src, rec, e.Pos()) {
			return nil
		}
		return slot
	}
	v := fl.expr(e.Rhs)
	if v == nil {
		return nil
	}

	if e.Op != token.ASSIGN {
		// Compound assignment a op= b evaluates a once.
		op, known := compoundOp(e.Op)
		if !known {
			fl.u.errorf(e.Pos(), "lowering does not handle %s yet", e.Op)
			return nil
		}
		lhs := fl.load(slot, t)
		if lhs == nil {
			return nil
		}
		v = fl.arith(op, e.Pos(), lhs, fl.convert(v, fl.typeOf(e.Rhs), t), t)
		if v == nil {
			return nil
		}
	}

	v = fl.convert(v, t, t)
	fl.store(slot, v, t)
	return v
}

// compoundOp is the binary operator inside a compound assignment.
func compoundOp(k token.Kind) (token.Kind, bool) {
	switch k {
	case token.ADD_ASSIGN:
		return token.ADD, true
	case token.SUB_ASSIGN:
		return token.SUB, true
	case token.MUL_ASSIGN:
		return token.MUL, true
	case token.QUO_ASSIGN:
		return token.QUO, true
	case token.REM_ASSIGN:
		return token.REM, true
	case token.AND_ASSIGN:
		return token.AND, true
	case token.OR_ASSIGN:
		return token.OR, true
	case token.XOR_ASSIGN:
		return token.XOR, true
	case token.SHL_ASSIGN:
		return token.SHL, true
	case token.SHR_ASSIGN:
		return token.SHR, true
	}
	return token.ILLEGAL, false
}

// incDec lowers ++ and --. Prefix yields the new value as an lvalue; postfix
// yields a copy of the old value.
func (fl *fn) incDec(x ast.Expr, op token.Kind, at ast.Tok, prefix bool) ir.Value {
	slot, t, ok := fl.lvalue(x)
	if !ok {
		return nil
	}
	old := fl.load(slot, t)
	if old == nil {
		return nil
	}

	one := fl.oneOf(t)
	var next ir.Value
	switch v := old.(type) {
	case ir.I32:
		o := one.(ir.I32)
		if op == token.INC {
			next = fl.blk.I32.Add(v, o)
		} else {
			next = fl.blk.I32.Sub(v, o)
		}
	case ir.I64:
		o := one.(ir.I64)
		if op == token.INC {
			next = fl.blk.I64.Add(v, o)
		} else {
			next = fl.blk.I64.Sub(v, o)
		}
	case ir.F64:
		o := one.(ir.F64)
		if op == token.INC {
			next = fl.blk.F64.Add(v, o)
		} else {
			next = fl.blk.F64.Sub(v, o)
		}
	case ir.Ptr:
		// Advance or retreat by element size, not one byte.
		ptr, isPtr := types.Unqualify(t).(*types.Pointer)
		if !isPtr {
			fl.u.errorf(at, "lowering: %s on an address that is not a pointer", op)
			return nil
		}
		elemSize, _ := fl.u.sizeAlign(ptr.Elem)
		if op == token.DEC {
			elemSize = -elemSize
		}
		next = fl.blk.Ptr.Add(v, fl.blk.I64.Const(elemSize))
	default:
		fl.u.errorf(at, "lowering does not handle %s on this type yet", op)
		return nil
	}

	next = fl.convert(next, t, t)
	fl.store(slot, next, t)
	if prefix {
		return next
	}
	return old
}

// oneOf is the 1 an increment adds, in the operand's own register type.
func (fl *fn) oneOf(t types.Type) ir.Value {
	switch fl.u.regType(t) {
	case ir.TypeI64:
		return fl.blk.I64.Const(1)
	case ir.TypeF32:
		return fl.blk.F32.Const(1)
	case ir.TypeF64:
		return fl.blk.F64.Const(1)
	default:
		return fl.blk.I32.Const(1)
	}
}

// conditional lowers conditional expressions via branching.
func (fl *fn) conditional(e *ast.CondExpr) ir.Value {
	t := fl.typeOf(e)

	c := fl.truth(e.Cond)
	if c == nil {
		return nil
	}
	slot := fl.alloc(t, "")
	rec := classOf(t)
	if rec != nil {
		// Class prvalue result constructed into slot.
		fl.temporary(slot, rec)
	}

	then := fl.block("cond_then")
	els := fl.block("cond_else")
	join := fl.block("cond_join")
	fl.blk.BrIf(*c, then.To(), els.To())

	fl.blk = then
	if rec != nil {
		fl.exprInto(slot, e.Then, rec)
	} else if v := fl.expr(e.Then); v != nil {
		fl.store(slot, fl.convert(v, fl.typeOf(e.Then), t), t)
	}
	if fl.blk != nil {
		fl.blk.Br(join.To())
	}

	fl.blk = els
	if rec != nil {
		fl.exprInto(slot, e.Else, rec)
	} else if v := fl.expr(e.Else); v != nil {
		fl.store(slot, fl.convert(v, fl.typeOf(e.Else), t), t)
	}
	if fl.blk != nil {
		fl.blk.Br(join.To())
	}

	fl.blk = join
	if rec != nil {
		return slot
	}
	return fl.load(slot, t)
}

// call lowers a resolved function call. Calls returning references yield loaded values.
func (fl *fn) call(e *ast.CallExpr) ir.Value {
	// Functional cast T(args) creates a temporary via selected constructor.
	if ctor := fl.u.res.Info.Temporaries[e]; ctor != nil {
		rec := classOf(fl.typeOf(e))
		if rec == nil {
			rec = ctor.InClass
		}
		tmp := fl.alloc(rec, "")
		fl.temporary(tmp, rec)
		fl.constructObject(tmp, rec, ctor, e.Args, e.Pos())
		return tmp
	}
	if rec := classOf(fl.typeOf(e)); rec != nil && fl.u.res.Info.Calls[e] == nil && fl.u.res.Info.Operators[e] == nil && fl.calleeNamesType(e) {
		// Value-initialized temporary for T() or copy initialization for T(x).
		tmp := fl.alloc(rec, "")
		fl.temporary(tmp, rec)
		if len(e.Args) == 1 {
			if src, ok := fl.objectOf(e.Args[0]); ok {
				fl.copyObject(tmp, src, rec, nil)
			}
			return tmp
		}
		size, _ := fl.u.sizeAlign(rec)
		fl.blk.MemSet(tmp, fl.blk.I32.Const(0), fl.blk.I64.Const(size))
		fl.defaultConstruct(tmp, rec, e.Pos())
		return tmp
	}
	if id, isIdent := e.Fun.(*ast.Ident); isIdent {
		if v, isBuiltin := fl.builtinCall(id.Text(fl.u.unit), e); isBuiltin {
			return v
		}
	}
	if v, ok := fl.scalarFunctionalCast(e); ok {
		return v
	}
	if bin, isBin := unparen(e.Fun).(*ast.BinaryExpr); isBin && (bin.Op == token.PERIOD_STAR || bin.Op == token.ARROW_STAR) {
		return fl.memberPointerCall(e, bin)
	}
	if op := fl.u.res.Info.Operators[e]; op != nil {
		return fl.operatorValue(e, op, e.Fun, e.Args)
	}
	callee := fl.u.res.Info.Calls[e]
	var v ir.Value
	var retType types.Type
	if callee != nil {
		v = fl.callRaw(e, callee)
		retType = callee.FuncType.Ret
	} else if val, ft, ok := fl.indirectCallRaw(e); ok {
		v = val
		retType = ft.Ret
	} else {
		fl.u.errorf(e.Pos(), "lowering has no resolved callee for this call")
		return nil
	}
	if v == nil || !isReference(retType) {
		return v
	}
	p, isPtr := v.(ir.Ptr)
	if !isPtr {
		return nil
	}
	unrefRet := types.RemoveReference(retType)
	if classOf(unrefRet) != nil {
		return p
	}
	return fl.load(p, unrefRet)
}

// callRaw is the call itself, its result exactly as the function returned it.
func (fl *fn) callRaw(e *ast.CallExpr, callee *sema.FuncSymbol) ir.Value {
	// A virtual call goes through the table and names no function: an
	// import of the callee here would be an undefined symbol for one
	// that is never called directly -- a pure virtual has no definition
	// anywhere.
	var target ir.Callee
	if !isVirtualCall(e, callee) {
		target = fl.u.callee(callee)
		if target == nil {
			fl.u.errorf(e.Pos(), "lowering has no symbol for %q", callee.SymName)
			return nil
		}
	}

	args := make([]ir.Value, 0, len(e.Args)+2)

	// A class result needs storage before the call is made: a temporary
	// in this frame, passed where the callee's signature put its hidden
	// result parameter, and the value of the call expression afterwards.
	var result ir.Ptr
	retRec := classOf(callee.FuncType.Ret)
	hiddenAfterThis := false
	if retRec != nil {
		// The temporary, or the object an initialization asked to have
		// built in place (exprInto).
		if fl.resultInto != (ir.Ptr{}) {
			result = fl.resultInto
			fl.resultInto = ir.Ptr{}
		} else {
			result = fl.alloc(retRec, "")
			fl.temporary(result, retRec)
		}
		hiddenAfterThis = !fl.u.plainForReturn(retRec) && fl.u.model.ABI.ResultAfterThis() && callee.InClass != nil && !callee.Static
		if !hiddenAfterThis {
			args = append(args, result)
		}
	}

	// The implicit object argument (this) comes first for member calls.
	if callee.InClass != nil && !callee.Static {
		obj, ok := fl.objectArg(e)
		if !ok {
			return nil
		}
		// A direct call passes the subobject the function expects, which
		// for an override of a secondary base's virtual is that base's
		// (types.ThisOffset). A virtual call finds the same subobject from
		// the slot's table, and adjusting here as well moved it twice.
		if off := fl.u.thisOffset(callee); off != 0 && !isVirtualCall(e, callee) {
			if p, isPtr := obj.(ir.Ptr); isPtr {
				obj = fl.blk.Ptr.Add(p, fl.blk.I64.Const(off))
			}
		}
		args = append(args, obj)
	}
	if hiddenAfterThis {
		args = append(args, result)
	}

	return fl.finishCall(e, callee, target, args, e.Args, result, retRec, hiddenAfterThis)
}

// finishCall converts and appends the explicit arguments and makes the
// call: args so far holds the result slot and the object, as the ABI
// orders them. e is the call expression for a virtual dispatch, or nil
// for a call that has none (an operator's).
func (fl *fn) finishCall(e *ast.CallExpr, callee *sema.FuncSymbol, target ir.Callee, args []ir.Value, argExprs []ast.Expr, result ir.Ptr, retRec *types.Record, hiddenAfterThis bool, extra ...ir.Value) ir.Value {
	for i, a := range argExprs {
		var want types.Type
		if i < len(callee.FuncType.Params) {
			want = callee.FuncType.Params[i].Type
		}
		if rec := classOf(want); rec != nil {
			// A class argument: the address of the object, which `byval`
			// has the backend copy for a plain class. For any other the
			// copy is the caller's to make, by the copy constructor, and
			// that is not lowered yet.
			addr, ok := fl.convertedObject(a, rec)
			if !ok {
				addr, ok = fl.objectOf(a)
			}
			if !ok {
				return nil
			}
			if !fl.u.plainForCalls(rec) {
				// The parameter object is a copy this caller makes, by
				// the copy constructor, and passes by address. Who
				// destroys it afterwards is the ABI's call: the callee
				// under Microsoft (defineFunc tracks it), the caller
				// under Itanium, at the end of the full-expression.
				tmp := fl.alloc(rec, "")
				if !fl.copyObject(tmp, addr, rec, nil) {
					return nil
				}
				if !fl.u.model.ABI.CalleeDestroysParameters() {
					fl.temporary(tmp, rec)
				}
				addr = tmp
			}
			args = append(args, addr)
			continue
		}
		if isReference(want) {
			// Reference parameter is passed by address.
			addr, ok := fl.bind(a, want)
			if !ok {
				return nil
			}
			args = append(args, addr)
			continue
		}
		v, converted := fl.convertedScalar(a)
		if !converted {
			v = fl.expr(a)
		}
		if v == nil {
			return nil
		}
		args = append(args, fl.convert(v, fl.typeOf(a), want))
	}

	// The postfix increment's int, which no expression supplies.
	args = append(args, extra...)

	// Virtual function calls through an object dispatch through the vtable.
	if e != nil && isVirtualCall(e, callee) && len(args) > 0 {
		objIdx := 0
		if retRec != nil && !hiddenAfterThis {
			objIdx = 1
		}
		v := fl.virtualCall(e, callee, args[objIdx], args, objIdx)
		if retRec != nil {
			return result
		}
		return v
	}

	res := fl.blk.Call(target, args...)
	if retRec != nil {
		return result
	}
	if res.Len() == 0 {
		return nil
	}
	return res.Value(0)
}

// indirectCallRaw lowers a call through a function pointer or function value.
func (fl *fn) indirectCallRaw(e *ast.CallExpr) (ir.Value, *types.Func, bool) {
	t := fl.typeOf(e.Fun)
	if t == nil {
		return nil, nil, false
	}
	unqual := types.Unqualify(types.RemoveReference(t))
	var ft *types.Func
	switch u := unqual.(type) {
	case *types.Func:
		ft = u
	case *types.Pointer:
		if f, isFn := u.Elem.(*types.Func); isFn {
			ft = f
		}
	}
	if ft == nil {
		return nil, nil, false
	}

	calleeVal := fl.expr(e.Fun)
	if calleeVal == nil {
		return nil, nil, false
	}
	calleePtr, isPtr := calleeVal.(ir.Ptr)
	if !isPtr {
		fl.u.errorf(e.Pos(), "lowering: indirect call callee is not a pointer")
		return nil, nil, false
	}

	fnSym := &sema.FuncSymbol{
		SymName:  "indirect",
		FuncType: ft,
	}
	irFuncType := fl.u.funcType(fnSym)

	args := make([]ir.Value, 0, len(e.Args)+1)
	var result ir.Ptr
	retRec := classOf(ft.Ret)
	if retRec != nil {
		result = fl.alloc(retRec, "")
		if fl.resultInto != (ir.Ptr{}) {
			result = fl.resultInto
			fl.resultInto = ir.Ptr{}
		}
		args = append(args, result)
	}

	for i, a := range e.Args {
		var want types.Type
		if i < len(ft.Params) {
			want = ft.Params[i].Type
		}
		if rec := classOf(want); rec != nil {
			addr, ok := fl.objectOf(a)
			if !ok {
				return nil, nil, false
			}
			if !fl.u.plainForCalls(rec) {
				tmp := fl.alloc(rec, "")
				if !fl.copyObject(tmp, addr, rec, nil) {
					return nil, nil, false
				}
				addr = tmp
			}
			args = append(args, addr)
			continue
		}
		if isReference(want) {
			addr, ok := fl.bind(a, want)
			if !ok {
				return nil, nil, false
			}
			args = append(args, addr)
			continue
		}
		v := fl.expr(a)
		if v == nil {
			return nil, nil, false
		}
		args = append(args, fl.convert(v, fl.typeOf(a), want))
	}

	res := fl.blk.CallInd(calleePtr, irFuncType, args...)
	if retRec != nil {
		return result, ft, true
	}
	if res.Len() == 0 {
		return nil, ft, true
	}
	return res.Value(0), ft, true
}

// objectArg is the address a member call is made on.
func (fl *fn) objectArg(e *ast.CallExpr) (ir.Value, bool) {
	mem, isMember := unparen(e.Fun).(*ast.MemberExpr)
	if !isMember {
		// `f()` inside a member function calls `this->f()`.
		if !fl.hasThis {
			fl.u.errorf(e.Pos(), "lowering: a member call with no object")
			return nil, false
		}
		return fl.this, true
	}
	if mem.Op == token.ARROW {
		p, _, ok := fl.arrowBase(mem)
		if !ok {
			return nil, false
		}
		return p, true
	}
	// The object is whatever the expression denotes: a variable's
	// storage, or the temporary a call or a cast made -- `boxed(4).get()`
	// calls a member on a prvalue, which lives in this frame.
	slot, ok := fl.objectOf(mem.X)
	if !ok {
		return nil, false
	}
	return slot, true
}

// throughRef reads a reference's storage for the object it denotes.
//
// A reference is an address at the machine level and its slot holds that
// address; the object the name means is at the other end. For any other
// type the slot is the object.
func (fl *fn) throughRef(slot ir.Ptr, declared types.Type) (ir.Ptr, types.Type, bool) {
	if !isReference(declared) {
		return slot, declared, true
	}
	return fl.blk.Ptr.Load(slot), types.RemoveReference(declared), true
}

// bind returns the address a reference of type ref binds to when initialized
// from e.
//
// An lvalue binds directly (with base subobject adjustment if needed).
// A prvalue is materialized into a temporary.
func (fl *fn) bind(e ast.Expr, ref types.Type) (ir.Ptr, bool) {
	target := types.RemoveReference(ref)
	if addr, from, ok := fl.lvalueQuiet(e); ok {
		if off := fl.u.baseAdjust(&types.Pointer{Elem: from}, &types.Pointer{Elem: target}); off != 0 {
			addr = fl.blk.Ptr.Add(addr, fl.blk.I64.Const(off))
		}
		return addr, true
	}
	// A class prvalue bound to a reference is materialized into a temporary.
	if rec := classOf(target); rec != nil {
		if addr, ok := fl.convertedObject(e, rec); ok {
			return addr, true
		}
	}
	if rec := classOf(target); rec != nil && classOf(fl.typeOf(e)) != nil {
		addr, ok := fl.objectOf(e)
		if !ok {
			return ir.Ptr{}, false
		}
		if off := fl.u.baseAdjust(&types.Pointer{Elem: fl.typeOf(e)}, &types.Pointer{Elem: target}); off != 0 {
			addr = fl.blk.Ptr.Add(addr, fl.blk.I64.Const(off))
		}
		return addr, true
	}
	v := fl.expr(e)
	if v == nil {
		return ir.Ptr{}, false
	}
	tmp := fl.alloc(target, "")
	fl.store(tmp, fl.convert(v, fl.typeOf(e), target), target)
	return tmp, true
}

// arrowBase returns the pointer x-> dereferences and its pointee type,
// evaluating chained operator-> calls if needed.
func (fl *fn) arrowBase(e *ast.MemberExpr) (ir.Ptr, types.Type, bool) {
	chain := fl.u.res.Info.Arrows[e]
	if len(chain) == 0 {
		v := fl.expr(e.X)
		if v == nil {
			return ir.Ptr{}, nil, false
		}
		p, isPtr := v.(ir.Ptr)
		if !isPtr {
			fl.u.errorf(e.Pos(), "lowering: the left operand of -> is not a pointer")
			return ir.Ptr{}, nil, false
		}
		ptr, isPtrType := types.Unqualify(fl.typeOf(e.X)).(*types.Pointer)
		if !isPtrType {
			fl.u.errorf(e.Pos(), "lowering: the left operand of -> has no pointee type")
			return ir.Ptr{}, nil, false
		}
		return p, ptr.Elem, true
	}
	obj, ok := fl.objectOf(e.X)
	if !ok {
		return ir.Ptr{}, nil, false
	}
	for _, op := range chain {
		target := fl.u.callee(op)
		if target == nil {
			fl.u.errorf(e.Pos(), "lowering has no symbol for %s::operator->", op.InClass.Name)
			return ir.Ptr{}, nil, false
		}
		if off := fl.u.thisOffset(op); off != 0 {
			obj = fl.blk.Ptr.Add(obj, fl.blk.I64.Const(off))
		}
		ret := op.FuncType.Ret
		if rr := classOf(ret); rr != nil {
			// A class by value: a temporary of this full-expression, in
			// the place the ABI puts a hidden result (see callRaw), and
			// the next operator-> is applied to it.
			tmp := fl.alloc(rr, "")
			fl.temporary(tmp, rr)
			if !fl.u.plainForReturn(rr) && fl.u.model.ABI.ResultAfterThis() {
				fl.blk.Call(target, obj, tmp)
			} else {
				fl.blk.Call(target, tmp, obj)
			}
			obj = tmp
			continue
		}
		res := fl.blk.Call(target, obj)
		if res.Len() == 0 {
			return ir.Ptr{}, nil, false
		}
		p, isPtr := res.Value(0).(ir.Ptr)
		if !isPtr {
			fl.u.errorf(e.Pos(), "lowering: operator-> did not return a pointer")
			return ir.Ptr{}, nil, false
		}
		if ptr := types.AsPointer(ret); ptr != nil {
			if len(chain) > 0 && op == chain[len(chain)-1] {
				return p, ptr.Elem, true
			}
		}
		// A reference to a class: the next operator-> is called on it.
		obj = p
	}
	return ir.Ptr{}, nil, false
}

// calleeNamesType is whether a call's callee is a type -- a functional
// cast -- rather than a function: the analysis resolved the name to a
// class or recorded no callee for it.
// scalarFunctionalCast lowers functional casts for non-class types parsed
// as calls via type aliases (e.g. V(d) or V()).
func (fl *fn) scalarFunctionalCast(e *ast.CallExpr) (ir.Value, bool) {
	f := unparen(e.Fun)
	if _, isType := fl.u.res.Info.Uses[f].(*sema.TypeSymbol); !isType {
		return nil, false
	}
	t := fl.typeOf(e)
	if t == nil || classOf(t) != nil || fl.u.res.Info.Calls[e] != nil || fl.u.res.Info.Operators[e] != nil {
		return nil, false
	}
	switch len(e.Args) {
	case 0:
		return fl.zeroOf(t), true
	case 1:
		v := fl.expr(e.Args[0])
		if v == nil {
			return nil, true
		}
		return fl.convert(v, fl.typeOf(e.Args[0]), t), true
	}
	return nil, false
}

func (fl *fn) calleeNamesType(e *ast.CallExpr) bool {
	switch f := unparen(e.Fun).(type) {
	case *ast.Ident, *ast.QualifiedName, *ast.TemplateName:
		if _, isRec := fl.u.res.Info.Uses[f].(*sema.RecordSymbol); isRec {
			return true
		}
		if _, isType := fl.u.res.Info.Uses[f].(*sema.TypeSymbol); isType {
			return true
		}
		return fl.u.res.Info.Uses[f] == nil
	}
	return false
}

// convertedScalar lowers user-defined conversion functions or lambda conversion
// to function pointer.
func (fl *fn) convertedScalar(e ast.Expr) (ir.Value, bool) {
	conv := fl.u.res.Info.Conversions[e]
	if conv == nil || conv.InClass == nil || conv.SymName == conv.InClass.Name {
		return nil, false
	}
	if inv := fl.u.lambdaInvoker(conv.InClass); inv != nil {
		fl.expr(e) // the closure object is made and dropped
		return fl.blk.Ptr.GetAddr(inv), true
	}
	obj, ok := fl.objectOf(e)
	if !ok {
		return nil, false
	}
	v := fl.invoke(conv, obj, nil, ir.Ptr{}, e.Pos())
	if v == nil {
		return nil, false
	}
	if isReference(conv.FuncType.Ret) {
		if p, isPtr := v.(ir.Ptr); isPtr {
			return fl.load(p, types.RemoveReference(conv.FuncType.Ret)), true
		}
	}
	return v, true
}

// convertedObject creates a temporary via a converting constructor if selected.
func (fl *fn) convertedObject(e ast.Expr, rec *types.Record) (ir.Ptr, bool) {
	ctor := fl.u.res.Info.Conversions[e]
	if ctor == nil {
		return ir.Ptr{}, false
	}
	tmp := fl.alloc(rec, "")
	fl.constructWith(tmp, ctor, []ast.Expr{e}, e.Pos())
	fl.temporary(tmp, rec)
	return tmp, true
}

// isLValueShape is whether lvalueQuiet would find an address for an
// expression, decided without emitting anything: the same cases, read
// off the tree.
func (fl *fn) isLValueShape(e ast.Expr) bool {
	switch e := unparen(e).(type) {
	case *ast.Ident, *ast.QualifiedName, *ast.MemberExpr, *ast.IndexExpr, *ast.CondExpr, *ast.StringLit, *ast.AssignExpr:
		return true
	case *ast.UnaryExpr:
		return e.Op == token.MUL
	case *ast.CallExpr:
		callee := fl.u.res.Info.Calls[e]
		return callee != nil && isReference(callee.FuncType.Ret)
	case *ast.BinaryExpr, *ast.IncDecExpr:
		op := fl.u.res.Info.Operators[e]
		return op != nil && isReference(op.FuncType.Ret)
	}
	return false
}

// lvalueQuiet is lvalue() for an expression that may well be a prvalue,
// where failing is an answer and not a diagnostic.
func (fl *fn) lvalueQuiet(e ast.Expr) (ir.Ptr, types.Type, bool) {
	switch e := unparen(e).(type) {
	case *ast.Ident, *ast.QualifiedName, *ast.MemberExpr, *ast.IndexExpr, *ast.CondExpr, *ast.StringLit:
		return fl.lvalue(e)
	case *ast.UnaryExpr:
		if e.Op == token.MUL {
			return fl.lvalue(e)
		}
	case *ast.CallExpr:
		if callee := fl.u.res.Info.Calls[e]; callee != nil && isReference(callee.FuncType.Ret) {
			return fl.lvalue(e)
		}
	case *ast.BinaryExpr, *ast.IncDecExpr:
		if op := fl.u.res.Info.Operators[e]; op != nil && isReference(op.FuncType.Ret) {
			return fl.lvalue(e)
		}
	case *ast.AssignExpr:
		if op := fl.u.res.Info.Operators[e]; op != nil {
			if isReference(op.FuncType.Ret) {
				return fl.lvalue(e)
			}
			return ir.Ptr{}, nil, false
		}
		return fl.lvalue(e.Lhs)
	}
	return ir.Ptr{}, nil, false
}

// conditionalAddr is the lvalue form of the conditional: each arm's
// address is computed in its branch and the chosen one flows to the join
// through a frame slot.
func (fl *fn) conditionalAddr(e *ast.CondExpr) (ir.Ptr, types.Type, bool) {
	c := fl.truth(e.Cond)
	if c == nil {
		return ir.Ptr{}, nil, false
	}
	slot := fl.alloc(&types.Pointer{Elem: types.Typ(types.Void)}, "")

	then := fl.block("cond_then")
	els := fl.block("cond_else")
	join := fl.block("cond_join")
	fl.blk.BrIf(*c, then.To(), els.To())

	var t types.Type
	fl.blk = then
	if addr, at, ok := fl.lvalue(e.Then); ok {
		t = at
		fl.blk.Ptr.Store(addr, slot)
	}
	if fl.blk != nil {
		fl.blk.Br(join.To())
	}

	fl.blk = els
	if addr, _, ok := fl.lvalue(e.Else); ok {
		fl.blk.Ptr.Store(addr, slot)
	}
	if fl.blk != nil {
		fl.blk.Br(join.To())
	}

	fl.blk = join
	if t == nil {
		return ir.Ptr{}, nil, false
	}
	return fl.blk.Ptr.Load(slot), t, true
}

func isReference(t types.Type) bool {
	switch t.(type) {
	case *types.LValueReference, *types.RValueReference:
		return true
	}
	return false
}

// functionalCast lowers functional-style casts and temporaries.
func (fl *fn) functionalCast(e *ast.FunctionalCastExpr) ir.Value {
	t := fl.typeOf(e)
	rec := classOf(t)
	if rec == nil {
		// A scalar: one operand, converted.
		var x ast.Expr
		switch {
		case len(e.ArgList) == 1:
			x = e.ArgList[0]
		case e.Args != nil && len(e.Args.Items) == 1:
			x = e.Args.Items[0]
		case e.Args != nil && len(e.Args.Items) == 0, len(e.ArgList) == 0:
			// `T()` and `T{}` -- value-initialization: zero.
			return fl.zeroOf(t)
		default:
			fl.u.errorf(e.Pos(), "lowering: a scalar cast with %d operands", len(e.ArgList))
			return nil
		}
		v := fl.expr(x)
		if v == nil {
			return nil
		}
		return fl.convert(v, fl.typeOf(x), t)
	}

	tmp := fl.alloc(rec, "")
	if ctor := fl.u.res.Info.Casts[e]; ctor != nil {
		args := e.ArgList
		if e.Args != nil {
			for _, item := range e.Args.Items {
				if x, isExpr := item.(ast.Expr); isExpr {
					args = append(args, x)
				}
			}
		}
		fl.constructWith(tmp, ctor, args, e.Pos())
		return tmp
	}
	fl.installVPtr(tmp, rec)
	switch {
	case e.Args != nil:
		fl.initList(tmp, t, e.Args)
	case len(e.ArgList) == 1:
		// `T(u)` on a plain class: a copy.
		if src, ok := fl.objectOf(e.ArgList[0]); ok {
			fl.copyObjectBytes(tmp, src, rec)
		}
	case len(e.ArgList) == 0:
		// Value-initialization of a plain class: zeroed.
		size, _ := fl.u.sizeAlign(rec)
		fl.blk.MemSet(tmp, fl.blk.I32.Const(0), fl.blk.I64.Const(size))
	}
	return tmp
}

// sizeofExpr is a sizeof or alignof, which is a number the layout knows.
func (fl *fn) sizeofExpr(e ast.Expr) ir.Value {
	var t types.Type
	var operand ast.Expr
	isAlign := false
	switch x := e.(type) {
	case *ast.SizeofExpr:
		if x.Ellipsis.IsValid() {
			// sizeof...(pack): the count the analysis noted.
			n, known := fl.u.res.Info.Consts[e]
			if !known {
				fl.u.errorf(e.Pos(), "lowering has no count for this sizeof...")
				return nil
			}
			if fl.u.regType(fl.typeOf(e)) == ir.TypeI64 {
				return fl.blk.I64.Const(n)
			}
			return fl.blk.I32.Const(n)
		}
		if x.Type != nil {
			t = fl.u.typeOfTypeId(x.Type)
		} else {
			operand = x.X
		}
	case *ast.AlignofExpr:
		isAlign = true
		t = fl.u.typeOfTypeId(x.Type)
	}
	if operand != nil {
		// sizeof expr: unevaluated operand type (arrays do not decay here).
		t = fl.typeOf(operand)
	}
	if t == nil {
		fl.u.errorf(e.Pos(), "lowering: sizeof of an unknown type")
		return nil
	}
	size, align := fl.u.sizeAlign(t)
	n := size
	if isAlign {
		n = align
	}
	if fl.u.regType(fl.typeOf(e)) == ir.TypeI64 {
		return fl.blk.I64.Const(n)
	}
	return fl.blk.I32.Const(n)
}

// typeTrait is a compiler builtin in expression position: every trait is
// a constant the evaluator knows, except the one that asks whether it is
// being evaluated by it -- which, here, it is not.
func (fl *fn) typeTrait(e *ast.TypeTraitExpr) ir.Value {
	name := e.Name.Text(fl.u.unit)
	if name == "__builtin_is_constant_evaluated" || name == "__is_constant_evaluated" {
		return fl.blk.I32.Const(0)
	}
	n, known := fl.u.res.Info.Consts[e]
	if !known {
		fl.u.errorf(e.Pos(), "lowering has no value for %s", name)
		return nil
	}
	if fl.u.regType(fl.typeOf(e)) == ir.TypeI64 {
		return fl.blk.I64.Const(n)
	}
	return fl.blk.I32.Const(n)
}

// templateId is a variable template's instance in expression position:
// the constant sema folded it to. An instance that is not a constant --
// a mutable one, or one of class type -- has no object to load yet.
func (fl *fn) templateId(e *ast.TemplateName) ir.Value {
	n, known := fl.u.res.Info.Consts[e]
	if !known {
		fl.u.errorf(e.Pos(), "lowering does not handle a variable template instance that is not a constant yet")
		return nil
	}
	if fl.u.regType(fl.typeOf(e)) == ir.TypeI64 {
		return fl.blk.I64.Const(n)
	}
	return fl.blk.I32.Const(n)
}

// builtinCall lowers the expression builtins sema admitted (see
// sema.builtinCall): the hints are nothing, and __builtin_addressof is
// the operand's address.
func (fl *fn) builtinCall(name string, e *ast.CallExpr) (ir.Value, bool) {
	if v, ok := fl.gnuBuiltinCall(name, e); ok {
		return v, true
	}
	switch name {
	case "__assume", "__builtin_assume", "__builtin_unreachable", "__debugbreak", "__noop", "__fastfail":
		return nil, true
	case "__builtin_addressof":
		if len(e.Args) != 1 {
			return nil, true
		}
		p, _, ok := fl.lvalue(e.Args[0])
		if !ok {
			return nil, true
		}
		return p, true
	}
	return nil, false
}

// operatorCall makes the call an overloaded operator resolved to: the
// operands as the call's object and arguments, in the expression's
// order. The result is as the function returned it -- a pointer for a
// reference, the temporary's address for a class.
func (fl *fn) operatorCall(e ast.Expr, op *sema.FuncSymbol) ir.Value {
	var lhs ast.Expr
	var rest []ast.Expr
	switch x := e.(type) {
	case *ast.BinaryExpr:
		lhs, rest = x.X, []ast.Expr{x.Y}
	case *ast.UnaryExpr:
		lhs = x.X
	case *ast.AssignExpr:
		lhs, rest = x.Lhs, []ast.Expr{x.Rhs}
	case *ast.IncDecExpr:
		return fl.operatorRaw(e, op, x.X, nil, fl.blk.I32.Const(0))
	case *ast.IndexExpr:
		lhs, rest = x.X, x.Args
	case *ast.CallExpr:
		lhs, rest = x.Fun, x.Args
	default:
		fl.u.errorf(e.Pos(), "lowering: an operator on a %T", e)
		return nil
	}
	return fl.operatorRaw(e, op, lhs, rest)
}

// operatorValue is operatorCall as a value: a reference result is loaded.
func (fl *fn) operatorValue(e ast.Expr, op *sema.FuncSymbol, lhs ast.Expr, rest []ast.Expr, extra ...ir.Value) ir.Value {
	v := fl.operatorRaw(e, op, lhs, rest, extra...)
	if v == nil || !isReference(op.FuncType.Ret) {
		return v
	}
	p, isPtr := v.(ir.Ptr)
	if !isPtr {
		return nil
	}
	retType := types.RemoveReference(op.FuncType.Ret)
	if classOf(retType) != nil {
		return p
	}
	return fl.load(p, retType)
}

func (fl *fn) operatorRaw(e ast.Expr, op *sema.FuncSymbol, lhs ast.Expr, rest []ast.Expr, extra ...ir.Value) ir.Value {
	target := fl.u.callee(op)
	if target == nil {
		fl.u.errorf(e.Pos(), "lowering has no symbol for %q", op.SymName)
		return nil
	}
	args := make([]ir.Value, 0, len(rest)+2)
	var result ir.Ptr
	retRec := classOf(op.FuncType.Ret)
	hiddenAfterThis := false
	if retRec != nil {
		result = fl.alloc(retRec, "")
		if fl.resultInto != (ir.Ptr{}) {
			result = fl.resultInto
			fl.resultInto = ir.Ptr{}
		}
		hiddenAfterThis = !fl.u.plainForReturn(retRec) && fl.u.model.ABI.ResultAfterThis() && op.InClass != nil && !op.Static
		if !hiddenAfterThis {
			args = append(args, result)
		}
	}
	argExprs := rest
	if op.InClass != nil && !op.Static {
		// A member: the left operand is the object.
		obj, ok := fl.objectOf(lhs)
		if !ok {
			return nil
		}
		if off := fl.u.thisOffset(op); off != 0 {
			obj = fl.blk.Ptr.Add(obj, fl.blk.I64.Const(off))
		}
		args = append(args, obj)
	} else {
		// A non-member: the left operand is the first argument.
		argExprs = append([]ast.Expr{lhs}, rest...)
	}
	if hiddenAfterThis {
		args = append(args, result)
	}
	return fl.finishCall(nil, op, target, args, argExprs, result, retRec, hiddenAfterThis, extra...)
}

// operatorAddr is the address an operator function returning a
// reference designates.
func (fl *fn) operatorAddr(e ast.Expr) (ir.Ptr, types.Type, bool) {
	op := fl.u.res.Info.Operators[e]
	if op == nil || !isReference(op.FuncType.Ret) {
		fl.u.errorf(e.Pos(), "lowering: an operator that is not an lvalue")
		return ir.Ptr{}, nil, false
	}
	v := fl.operatorCall(e, op)
	p, isPtr := v.(ir.Ptr)
	if !isPtr {
		return ir.Ptr{}, nil, false
	}
	return p, types.RemoveReference(op.FuncType.Ret), true
}

// bindingAddr is the address of the part of a hidden variable a
// structured binding's name means: an array element by index, a member
// by the layout's offset -- through the anonymous unions and the one
// base the members may sit in (see sema.bindableMembers).
func (fl *fn) bindingAddr(v *sema.VarSymbol) (ir.Ptr, types.Type, bool) {
	b := v.Binding
	slot, known := fl.slots[b.Hidden]
	if !known {
		fl.u.errorf(v.SymPos, "lowering has no storage for the object %q is a part of", v.SymName)
		return ir.Ptr{}, nil, false
	}
	base, hiddenT, ok := fl.throughRef(slot, b.Hidden.SymType)
	if !ok {
		return ir.Ptr{}, nil, false
	}
	switch t := types.Unqualify(hiddenT).(type) {
	case *types.Array:
		size, _ := fl.u.sizeAlign(t.Elem)
		return fl.blk.Ptr.Add(base, fl.blk.I64.Const(size*int64(b.Index))), types.RemoveReference(v.SymType), true
	case *types.Record:
		rec := t
		off := int64(0)
		for len(rec.Fields) == 0 && len(rec.Bases) == 1 {
			br := types.AsRecord(types.Unqualify(rec.Bases[0].Type))
			if br == nil {
				break
			}
			baseOffs := make([]int64, len(rec.Bases))
			fl.u.model.LayoutWithBases(rec, make([]int64, len(rec.Fields)), baseOffs)
			off += baseOffs[0]
			rec = br
		}
		i := 0
		offs := make([]int64, len(rec.Fields))
		fl.u.model.LayoutWithBases(rec, offs, make([]int64, len(rec.Bases)))
		for fi, f := range rec.Fields {
			if f.Name == "" {
				if inner := types.AsRecord(types.Unqualify(f.Type)); inner != nil {
					innerOffs := make([]int64, len(inner.Fields))
					fl.u.model.LayoutWithBases(inner, innerOffs, make([]int64, len(inner.Bases)))
					for gi := range inner.Fields {
						if i == b.Index {
							return fl.blk.Ptr.Add(base, fl.blk.I64.Const(off+offs[fi]+innerOffs[gi])), types.RemoveReference(v.SymType), true
						}
						i++
					}
					continue
				}
			}
			if i == b.Index {
				return fl.blk.Ptr.Add(base, fl.blk.I64.Const(off+offs[fi])), types.RemoveReference(v.SymType), true
			}
			i++
		}
	}
	fl.u.errorf(v.SymPos, "lowering cannot place the part %q of a structured binding", v.SymName)
	return ir.Ptr{}, nil, false
}
