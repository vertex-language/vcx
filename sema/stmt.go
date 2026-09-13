package sema

import (
	"fmt"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/types"
)

// RangeProtocol records resolved functions and types for class-based range-for iteration.
type RangeProtocol struct {
	Begin, End *FuncSymbol // member functions of the range, or free ones taking it
	Iter       types.Type  // what begin() returns, the iterator
	NotEqual   *FuncSymbol // operator!=, or operator== when Negated
	Negated    bool
	Increment  *FuncSymbol // prefix operator++
	Deref      *FuncSymbol // operator*
	Elem       types.Type  // what *b is, the element, without its reference
	ElemRef    bool        // *b is an lvalue
}

// rangeProtocol resolves begin/end and iterator operators for range-for on class types.
func (a *Analyzer) rangeProtocol(s *ast.RangeForStmt, rangeInfo ExprInfo) *RangeProtocol {
	rangeT := types.RemoveReference(rangeInfo.Type)
	rec := types.AsRecord(types.Unqualify(rangeT))
	if rec == nil {
		a.errorAt(s.Range.Pos(), fmt.Sprintf("cannot iterate over a %q: not an array or a class with begin and end", rangeInfo.Type))
		return nil
	}
	object := &Argument{Type: rangeT, IsLValue: true}
	find := func(name string) *FuncSymbol {
		if cands := a.memberFuncs(rec, name); len(cands) > 0 {
			ndiags := len(a.diags)
			if fn, err := a.resolveAmongOn(cands, object, nil, nil, s.Range.Pos()); err == nil {
				return fn
			}
			a.diags = a.diags[:ndiags]
		}
		// Free begin(range) / end(range) via argument-dependent lookup.
		var free []*FuncSymbol
		seen := map[*FuncSymbol]bool{}
		add := func(syms []Symbol) {
			for _, sym := range syms {
				if fn, isFn := sym.(*FuncSymbol); isFn && fn.InClass == nil && !seen[fn] {
					seen[fn] = true
					free = append(free, fn)
				}
			}
		}
		if rs := a.curScope.recordSymbol(rec); rs != nil && rs.SymScope != nil {
			ns := rs.SymScope
			for ns != nil && ns.Kind != NamespaceScope && ns.Kind != GlobalScope {
				ns = ns.Parent
			}
			if ns != nil {
				add(ns.LookupAssociated(name))
			}
		}
		if len(free) == 0 {
			return nil
		}
		ndiags := len(a.diags)
		fn, err := a.resolveAmong(free, nil, []Argument{*object}, s.Range.Pos())
		if err != nil {
			a.diags = a.diags[:ndiags]
			return nil
		}
		return fn
	}
	proto := &RangeProtocol{Begin: find("begin"), End: find("end")}
	if proto.Begin == nil || proto.End == nil {
		a.errorAt(s.Range.Pos(), fmt.Sprintf("cannot iterate over a %q: no begin and end found", rangeInfo.Type))
		return nil
	}
	a.ensureInstantiated(proto.Begin)
	a.ensureInstantiated(proto.End)
	proto.Iter = types.RemoveReference(proto.Begin.FuncType.Ret)
	iter := ExprInfo{Type: proto.Iter, ValCat: LValue}

	if ptr := types.AsPointer(proto.Iter); ptr != nil {
		// A pointer iterates itself.
		proto.Elem, proto.ElemRef = ptr.Elem, true
		if a.info != nil {
			a.info.Ranges[s] = proto
		}
		return proto
	}
	if types.AsRecord(types.Unqualify(proto.Iter)) == nil {
		a.errorAt(s.Range.Pos(), fmt.Sprintf("cannot iterate over a %q: begin() returns a %q, which is not a pointer or a class", rangeInfo.Type, proto.Iter))
		return nil
	}
	end := ExprInfo{Type: types.RemoveReference(proto.End.FuncType.Ret), ValCat: LValue}
	if fn, _, ok := a.chooseOperator("operator!=", []ExprInfo{iter, end}, s.Range.Pos()); ok && fn != nil {
		proto.NotEqual = fn
	} else if fn, _, ok := a.chooseOperator("operator==", []ExprInfo{iter, end}, s.Range.Pos()); ok && fn != nil {
		proto.NotEqual, proto.Negated = fn, true
	} else {
		a.errorAt(s.Range.Pos(), fmt.Sprintf("cannot iterate over a %q: its iterator %q has no operator!=", rangeInfo.Type, proto.Iter))
		return nil
	}
	if fn, _, ok := a.chooseOperator("operator++", []ExprInfo{iter}, s.Range.Pos()); ok && fn != nil {
		proto.Increment = fn
	} else {
		a.errorAt(s.Range.Pos(), fmt.Sprintf("cannot iterate over a %q: its iterator %q has no prefix operator++", rangeInfo.Type, proto.Iter))
		return nil
	}
	fn, elem, ok := a.chooseOperator("operator*", []ExprInfo{iter}, s.Range.Pos())
	if !ok || fn == nil {
		a.errorAt(s.Range.Pos(), fmt.Sprintf("cannot iterate over a %q: its iterator %q has no operator*", rangeInfo.Type, proto.Iter))
		return nil
	}
	proto.Deref, proto.Elem, proto.ElemRef = fn, elem.Type, elem.ValCat == LValue
	if a.info != nil {
		a.info.Ranges[s] = proto
	}
	return proto
}

func rangeElementType(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	isConst := types.IsConst(types.RemoveReference(t))
	switch r := types.Unqualify(types.RemoveReference(t)).(type) {
	case *types.Array:
		if isConst && !types.IsConst(r.Elem) {
			return types.AddConst(r.Elem)
		}
		return r.Elem
	case *types.Pointer:
		if isConst && !types.IsConst(r.Elem) {
			return types.AddConst(r.Elem)
		}
		return r.Elem
	}
	return nil
}

func (a *Analyzer) CheckStmt(stmt ast.Stmt) {
	if stmt == nil {
		return
	}

	switch s := stmt.(type) {
	case *ast.EmptyStmt:
		return

	case *ast.ExprStmt:
		a.CheckExpr(s.X)

	case *ast.DeclStmt:
		a.CheckDecl(s.Decl)

	case *ast.CompoundStmt:
		oldScope := a.curScope
		a.curScope = NewScope(oldScope, BlockScope, nil)
		for _, child := range s.Stmts {
			a.CheckStmt(child)
		}
		a.curScope = oldScope

	case *ast.IfStmt:
		if s.Init != nil {
			a.CheckStmt(s.Init)
		}
		if s.Constexpr.IsValid() {
			if condExpr, ok := s.Cond.(ast.Expr); ok {
				// if constexpr condition is a constant expression. In dependent
				// contexts without a concrete value, both branches are checked.
				ctx := a.NewConstContext()
				passed, err := ctx.EvalBool(condExpr)
				switch {
				case err != nil && a.dependentContext():
					a.CheckExpr(condExpr)
					a.CheckStmt(s.Then)
					if s.Else != nil {
						a.CheckStmt(s.Else)
					}
				case err != nil:
					a.errorAt(condExpr.Pos(), fmt.Sprintf("if constexpr condition is not a constant expression: %v", err))
				case passed:
					// Record outcome for lowering to retain the taken branch.
					a.noteConst(condExpr, 1)
					a.CheckStmt(s.Then)
				case s.Else != nil:
					a.noteConst(condExpr, 0)
					a.CheckStmt(s.Else)
				default:
					a.noteConst(condExpr, 0)
				}
				return
			}
		}
		if s.Cond != nil {
			if condExpr, ok := s.Cond.(ast.Expr); ok {
				c := a.CheckExpr(condExpr)
				if !isDependentExpr(c) && !contextuallyConvertibleToBool(c.Type) {
					a.errorAt(condExpr.Pos(), "condition must be convertible to bool")
				}
			}
		}
		a.CheckStmt(s.Then)
		if s.Else != nil {
			a.CheckStmt(s.Else)
		}

	case *ast.SwitchStmt:
		a.loopDepth++
		if s.Init != nil {
			a.CheckStmt(s.Init)
		}
		if s.Cond != nil {
			if condExpr, ok := s.Cond.(ast.Expr); ok {
				c := a.CheckExpr(condExpr)
				if !types.IsInteger(c.Type) {
					a.errorAt(condExpr.Pos(), "switch condition must be of integral or enumeration type")
				}
			}
		}
		a.CheckStmt(s.Body)
		a.loopDepth--

	case *ast.WhileStmt:
		a.loopDepth++
		if s.Cond != nil {
			if condExpr, ok := s.Cond.(ast.Expr); ok {
				c := a.CheckExpr(condExpr)
				if !isDependentExpr(c) && !contextuallyConvertibleToBool(c.Type) {
					a.errorAt(condExpr.Pos(), "while condition must be convertible to bool")
				}
			}
		}
		a.CheckStmt(s.Body)
		a.loopDepth--

	case *ast.DoStmt:
		a.loopDepth++
		a.CheckStmt(s.Body)
		if s.Cond != nil {
			c := a.CheckExpr(s.Cond)
			if !isDependentExpr(c) && !contextuallyConvertibleToBool(c.Type) {
				a.errorAt(s.Cond.Pos(), "do-while condition must be convertible to bool")
			}
		}
		a.loopDepth--

	case *ast.ForStmt:
		a.loopDepth++
		oldScope := a.curScope
		a.curScope = NewScope(oldScope, BlockScope, nil)
		if s.Init != nil {
			a.CheckStmt(s.Init)
		}
		if s.Cond != nil {
			if condExpr, ok := s.Cond.(ast.Expr); ok {
				c := a.CheckExpr(condExpr)
				if !isDependentExpr(c) && !contextuallyConvertibleToBool(c.Type) {
					a.errorAt(condExpr.Pos(), "for condition must be convertible to bool")
				}
			}
		}
		if s.Post != nil {
			a.CheckExpr(s.Post)
		}
		a.CheckStmt(s.Body)
		a.curScope = oldScope
		a.loopDepth--

	case *ast.RangeForStmt:
		a.loopDepth++
		oldScope := a.curScope
		a.curScope = NewScope(oldScope, BlockScope, nil)
		if s.Init != nil {
			a.CheckStmt(s.Init)
		}
		rangeInfo := a.CheckExpr(s.Range)
		if s.Decl != nil {
			// The loop variable's type is deduced from the range element type (*begin()).
			prev := a.rangeElem
			a.rangeElem = rangeElementType(rangeInfo.Type)
			if a.rangeElem == nil && !isDependentExpr(rangeInfo) {
				if proto := a.rangeProtocol(s, rangeInfo); proto != nil {
					a.rangeElem = proto.Elem
				}
			}
			a.CheckDecl(s.Decl)
			a.rangeElem = prev
		}
		a.CheckStmt(s.Body)
		a.curScope = oldScope
		a.loopDepth--

	case *ast.BreakStmt:
		if a.loopDepth <= 0 {
			a.errorAt(s.Pos(), "'break' statement not in loop or switch statement")
		}

	case *ast.ContinueStmt:
		if a.loopDepth <= 0 {
			a.errorAt(s.Pos(), "'continue' statement not in loop statement")
		}

	case *ast.ReturnStmt:
		a.checkReturnStmt(s)

	case *ast.LabeledStmt:
		if s.Value != nil {
			a.CheckExpr(s.Value)
		}
		a.CheckStmt(s.Stmt)

	case *ast.AttrStmt:
		// Attributes appertain to the statement.
		a.CheckStmt(s.Stmt)

	case *ast.TryStmt:
		if s.Body != nil {
			a.CheckStmt(s.Body)
		}
		for _, h := range s.Handlers {
			if h.Body != nil {
				a.CheckStmt(h.Body)
			}
		}
	}
}

func (a *Analyzer) checkReturnStmt(r *ast.ReturnStmt) {
	if a.curFunc == nil || a.curFunc.FuncType == nil {
		a.errorAt(r.Pos(), "'return' statement not inside a function")
		return
	}

	retT := a.curFunc.FuncType.Ret

	// If function returns void
	if types.IsVoid(retT) {
		if r.X != nil {
			info := a.CheckExpr(r.X)
			if !types.IsVoid(info.Type) && !isDependentExpr(info) {
				a.errorAt(r.Pos(), "void function should not return a value")
			}
		}
		return
	}

	// Function returns non-void
	if r.X == nil {
		a.errorAt(r.Pos(), fmt.Sprintf("non-void function %q must return a value", a.curFunc.Name()))
		return
	}

	// return { ... } copy-list-initializes the return object.
	if list, isList := r.X.(*ast.InitList); isList && retT.Kind() != types.AutoKind {
		a.checkListInit(list, retT)
		return
	}

	info := a.CheckExpr(r.X)

	// Null pointer constant converts to pointer return types.
	if isNullConstant(r.X, info) && isPointerLike(retT) {
		return
	}

	// Deduce return type if declared as 'auto'.
	if retT.Kind() == types.AutoKind {
		if a.dependentContext() || isDependentExpr(info) {
			a.curFunc.FuncType.Ret = &types.DependentType{Name: "auto"}
			return
		}
		a.curFunc.FuncType.Ret = types.Unqualify(types.Decay(types.RemoveReference(info.Type)))
		return
	}

	// Defer conversion check if either type is dependent.
	if isDependentExpr(info) || isDependentType(retT) {
		return
	}

	cs := ClassifyConversion(info.Type, retT, info.ValCat == LValue)
	if !cs.Valid {
		a.errorAt(r.Pos(), fmt.Sprintf("cannot convert return value of type %q to return type %q", info.Type, retT))
	}
}
