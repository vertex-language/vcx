package sema

import (
	"fmt"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// comparisonCategory represents strong, weak, or partial ordering.
type comparisonCategory int

const (
	strongOrdering comparisonCategory = iota
	weakOrdering
	partialOrdering
)

func (c comparisonCategory) name() string {
	return [...]string{"strong_ordering", "weak_ordering", "partial_ordering"}[c]
}

// orderingType looks up the std comparison category record in the standard library.
func (a *Analyzer) orderingType(c comparisonCategory, at ast.Tok) types.Type {
	for _, sym := range a.lookupInStd(c.name()) {
		if rs, isRec := sym.(*RecordSymbol); isRec && rs.Record != nil && rs.Record.Complete {
			return rs.Record
		}
	}
	a.errorAt(at, fmt.Sprintf("<=> needs std::%s, which <compare> declares", c.name()))
	return nil
}

// categoryOf returns the comparison category yielded by a type's <=>.
func (a *Analyzer) categoryOf(t types.Type, at ast.Tok) (comparisonCategory, bool) {
	t = types.Unqualify(types.RemoveReference(t))
	switch {
	case types.IsFloat(t):
		return partialOrdering, true
	case types.IsArithmetic(t), types.IsEnum(t), types.IsPointer(t):
		return strongOrdering, true
	}
	rec := types.AsRecord(t)
	if rec == nil {
		return 0, false
	}
	operand := ExprInfo{Type: t, ValCat: LValue}
	ndiags := len(a.diags)
	fn, info, ok := a.chooseOperator("operator<=>", []ExprInfo{operand, operand}, at)
	a.diags = a.diags[:ndiags]
	if !ok || fn == nil {
		return 0, false
	}
	return a.categoryOfType(info.Type)
}

// categoryOfType reads the category off one of the three ordering classes.
func (a *Analyzer) categoryOfType(t types.Type) (comparisonCategory, bool) {
	rec := types.AsRecord(types.Unqualify(types.RemoveReference(t)))
	if rec == nil {
		return 0, false
	}
	for c := strongOrdering; c <= partialOrdering; c++ {
		if rec.Name == c.name() {
			return c, true
		}
	}
	return 0, false
}

// builtinSpaceship types built-in scalar `a <=> b` expressions.
func (a *Analyzer) builtinSpaceship(b *ast.BinaryExpr, left, right ExprInfo) ExprInfo {
	lt := types.Unqualify(types.RemoveReference(left.Type))
	rt := types.Unqualify(types.RemoveReference(right.Type))
	ok := (types.IsArithmetic(lt) || types.IsEnum(lt) || types.IsPointer(lt)) &&
		(types.IsArithmetic(rt) || types.IsEnum(rt) || types.IsPointer(rt))
	if !ok {
		a.errorAt(b.Pos(), fmt.Sprintf("invalid operands to <=> (%q and %q)", left.Type, right.Type))
		return ExprInfo{Type: types.Typ(types.Int), ValCat: PrValue}
	}
	category := strongOrdering
	if types.IsFloat(lt) || types.IsFloat(rt) {
		category = partialOrdering
	}
	t := a.orderingType(category, b.Pos())
	if t == nil {
		return ExprInfo{Type: types.Typ(types.Int), ValCat: PrValue}
	}
	return ExprInfo{Type: t, ValCat: PrValue}
}

// rewrittenRelational rewrites relational operators in terms of operator<=>.
func (a *Analyzer) rewrittenRelational(b *ast.BinaryExpr, left, right ExprInfo) (ExprInfo, bool) {
	switch b.Op {
	case token.LSS, token.GTR, token.LEQ, token.GEQ:
	default:
		return ExprInfo{}, false
	}
	boolean := ExprInfo{Type: types.Typ(types.Bool), ValCat: PrValue}
	zero := &ast.BasicLit{Span: ast.Span{Lo: b.OpPos, Hi: b.OpPos}, Kind: token.INT_LIT, Text: "0"}
	try := func(x, y ast.Expr, xi, yi ExprInfo, reversed bool) ast.Expr {
		spaceship := &ast.BinaryExpr{Span: b.Span, X: x, OpPos: b.OpPos, Op: token.SPACESHIP, Y: y}
		ndiags := len(a.diags)
		fn, _, resolved := a.chooseOperator("operator<=>", []ExprInfo{xi, yi}, b.Pos())
		if !resolved || fn == nil || len(a.diags) > ndiags {
			a.diags = a.diags[:ndiags]
			return nil
		}
		var outer *ast.BinaryExpr
		if reversed {
			outer = &ast.BinaryExpr{Span: b.Span, X: zero, OpPos: b.OpPos, Op: b.Op, Y: spaceship}
		} else {
			outer = &ast.BinaryExpr{Span: b.Span, X: spaceship, OpPos: b.OpPos, Op: b.Op, Y: zero}
		}
		info := a.CheckExpr(outer)
		if len(a.diags) > ndiags {
			a.diags = a.diags[:ndiags]
			return nil
		}
		if !types.IsBool(types.Unqualify(info.Type)) {
			return nil
		}
		return outer
	}
	rewritten := try(b.X, b.Y, left, right, false)
	if rewritten == nil {
		rewritten = try(b.Y, b.X, right, left, true)
	}
	if rewritten == nil {
		return ExprInfo{}, false
	}
	if a.info != nil {
		a.info.Rewrites[b] = rewritten
	}
	return boolean, true
}

// deduceDefaultedComparisons deduces the return type of a defaulted operator<=>
// with auto return type based on base and member subobjects.
func (a *Analyzer) deduceDefaultedComparisons(rec *types.Record, at ast.Tok) {
	a.implicitEquality(rec, at)
	for _, m := range rec.Methods {
		if !m.Defaulted || m.Name != "operator<=>" {
			continue
		}
		fn := a.methodSyms[m]
		if fn == nil || fn.FuncType == nil || !mentionsAuto(fn.FuncType.Ret) {
			continue
		}
		category := strongOrdering
		consider := func(t types.Type) bool {
			c, known := a.categoryOf(t, at)
			if !known {
				a.errorAt(at, fmt.Sprintf("cannot default operator<=> for %s: a %q has no three-way comparison", rec.Name, t))
				return false
			}
			if c > category {
				category = c
			}
			return true
		}
		for _, b := range rec.Bases {
			if !consider(b.Type) {
				return
			}
		}
		for _, f := range rec.Fields {
			if !consider(elementType(f.Type)) {
				return
			}
		}
		if t := a.orderingType(category, at); t != nil {
			fn.FuncType.Ret = t
			m.Func.Ret = t
		}
	}
}

// implicitEquality declares an implicit defaulted operator== if a class
// has a defaulted operator<=> and no explicit operator==.
func (a *Analyzer) implicitEquality(rec *types.Record, at ast.Tok) {
	var spaceship *types.Method
	for _, m := range rec.Methods {
		switch m.Name {
		case "operator==":
			return
		case "operator<=>":
			if m.Defaulted && spaceship == nil {
				spaceship = m
			}
		}
	}
	if spaceship == nil {
		return
	}
	scope := a.curScope
	if rs := a.curScope.recordSymbol(rec); rs != nil && rs.ClassScope != nil {
		scope = rs.ClassScope
	}
	ft := &types.Func{
		Ret:    types.Typ(types.Bool),
		Params: []types.Param{{Type: types.AddLValueReference(types.AddConst(rec))}},
		Quals:  types.QConst,
	}
	method := &types.Method{Name: "operator==", Func: ft, Access: spaceship.Access, Defaulted: true}
	rec.Methods = append(rec.Methods, method)
	fn := &FuncSymbol{
		SymName: "operator==", FuncType: ft, SymPos: at, SymScope: scope,
		Inline: true, Defaulted: true, InClass: rec, Access: spaceship.Access, Method: method,
	}
	if surviving, err := scope.InsertFunc(fn); err == nil {
		fn = surviving
	}
	a.noteDeclared(fn)
	if a.methodSyms == nil {
		a.methodSyms = map[*types.Method]*FuncSymbol{}
	}
	a.methodSyms[method] = fn
}

// elementType is an array's innermost element type, or t itself.
func elementType(t types.Type) types.Type {
	for {
		arr, isArr := types.Unqualify(t).(*types.Array)
		if !isArr {
			return t
		}
		t = arr.Elem
	}
}

// isDefaultedComparison reports whether fn is a defaulted operator== or operator<=>.
func isDefaultedComparison(fn *FuncSymbol) bool {
	return fn != nil && fn.Defaulted && (fn.SymName == "operator==" || fn.SymName == "operator<=>")
}
