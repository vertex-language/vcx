package sema

import (
	"fmt"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/types"
)

// checkListInit checks list initialization of target type from a braced-init-list.
func (a *Analyzer) checkListInit(list *ast.InitList, target types.Type) {
	if a.info != nil {
		a.info.Types[list] = target
	}
	bare := types.Unqualify(target)

	if isDependentType(target) {
		for _, item := range list.Items {
			a.CheckExpr(item)
		}
		return
	}
	if braced := a.brace(list, target); braced != list {
		if a.info != nil {
			a.info.Braced[list] = braced
			a.info.Types[braced] = target
		}
		list = braced
	}

	if arr, isArr := bare.(*types.Array); isArr {
		if arr.Incomplete {
			arr.Incomplete = false
			arr.Len = int64(len(list.Items))
		} else if int64(len(list.Items)) > arr.Len {
			a.errorAt(list.Pos(), fmt.Sprintf("too many initializers for an array of %d", arr.Len))
		}
		for _, item := range list.Items {
			a.checkListItem(item, arr.Elem)
		}
		return
	}

	rec := types.AsRecord(bare)
	if rec == nil {
		// A scalar: at most one item, converted.
		switch len(list.Items) {
		case 0:
		case 1:
			a.checkListItem(list.Items[0], target)
		default:
			a.errorAt(list.Pos(), fmt.Sprintf("too many initializers for %s", target))
		}
		return
	}

	if !hasUserConstructor(rec) {
		// Aggregate initialization: items initialize members in order.
		fields := aggregateFields(rec)
		if len(list.Items) > len(fields) {
			a.errorAt(list.Pos(), fmt.Sprintf("too many initializers for %s", rec.Name))
		}
		for i, item := range list.Items {
			// [dcl.init.aggr]/3.1: `.__type_ = true` initializes the member it
			// names, whatever its position.
			if d, isDesignated := item.(*ast.DesignatedInit); isDesignated {
				name := ""
				if d.Name != nil {
					name = d.Name.Text(a.unit)
				}
				found := false
				for _, f := range fields {
					if f.Name != "" && f.Name == name {
						a.checkListItem(d.Value, f.Type)
						found = true
						break
					}
				}
				if !found {
					a.errorAt(item.Pos(), fmt.Sprintf("no member named %q in %s", name, rec.Name))
				}
				continue
			}
			if i < len(fields) {
				a.checkListItem(item, fields[i].Type)
			}
		}
		return
	}

	// A class with constructors: the items are arguments.
	args := make([]Argument, 0, len(list.Items))
	for _, item := range list.Items {
		info := a.CheckExpr(item)
		if isDependentExpr(info) && a.dependentContext() {
			return
		}
		args = append(args, Argument{Type: info.Type, IsLValue: info.ValCat == LValue})
	}
	if _, err := a.chooseConstructor(rec, args); err != nil {
		a.errorAt(list.Pos(), fmt.Sprintf("no matching constructor for %s: %v", rec.Name, err))
	}
}

// checkListItem is one element of a braced list against the type of the
// member or element it lands in: a nested list recurses, an expression
// converts.
func (a *Analyzer) checkListItem(item ast.Expr, want types.Type) {
	if inner, isList := item.(*ast.InitList); isList {
		a.checkListInit(inner, want)
		return
	}
	info := a.CheckExpr(item)
	if isDependentExpr(info) || isDependentType(want) {
		return
	}
	cs := ClassifyConversion(info.Type, want, info.ValCat == LValue)
	if !cs.Valid {
		a.errorAt(item.Pos(), fmt.Sprintf("cannot initialize %s with a value of type %q", want, info.Type))
		return
	}
	// Element of class type: record converting constructor if needed.
	if rec := types.AsRecord(types.Unqualify(want)); rec != nil && a.info != nil {
		have := types.AsRecord(types.Unqualify(types.RemoveReference(info.Type)))
		if have == nil || have != rec && !types.IsBaseOf(rec, have) {
			if ctor := a.convertingConstructor(rec, Argument{Type: info.Type, IsLValue: info.ValCat == LValue}, item.Pos()); ctor != nil {
				a.ensureInstantiated(ctor)
				a.info.Conversions[item] = ctor
			}
		}
	}
}

// aggregateFields returns bases and non-static data members in initialization order.
func aggregateFields(rec *types.Record) []types.Field {
	var out []types.Field
	for _, b := range rec.Bases {
		out = append(out, types.Field{Name: "", Type: b.Type})
	}
	for _, f := range rec.Fields {
		out = append(out, f)
	}
	return out
}

// chooseConstructor runs overload resolution over a class's constructors.
func (a *Analyzer) chooseConstructor(rec *types.Record, args []Argument) (*FuncSymbol, error) {
	ctors := a.memberFuncs(rec, rec.Name)
	if len(ctors) == 0 {
		return nil, fmt.Errorf("%s has no constructor", rec.Name)
	}
	return a.resolveAmong(ctors, nil, args, ast.NoTok)
}

// brace is list with the braces [dcl.init.aggr]/16 lets a program leave
// out put back: where an item lands on a subaggregate -- an array, or a
// class initialized as an aggregate -- and is not itself a braced list or
// a value of that subaggregate's type, the subaggregate takes as many of
// the following items as it has elements. `{ {kind}, nullptr }` and
// `{ kind, nullptr }` are the same initialization of a struct whose first
// base holds kind.
//
// The answer is list itself where nothing was elided.
func (a *Analyzer) brace(list *ast.InitList, target types.Type) *ast.InitList {
	if !isAggregateType(target) {
		return list
	}
	i := 0
	elided := false
	out := a.braceInto(target, list.Items, &i, list, &elided)
	if !elided {
		return list
	}
	// Items past the end are left for the caller to report as too many.
	out.Items = append(out.Items, list.Items[i:]...)
	return out
}

// braceInto builds the list for one aggregate of type t out of items
// from *i on.
func (a *Analyzer) braceInto(t types.Type, items []ast.Expr, i *int, at *ast.InitList, elided *bool) *ast.InitList {
	out := &ast.InitList{Span: at.Span, Lbrace: at.Lbrace, Rbrace: at.Rbrace}
	if *i < len(items) {
		out.Span = ast.Span{Lo: items[*i].Pos(), Hi: items[*i].End()}
	}
	switch bare := types.Unqualify(t).(type) {
	case *types.Array:
		for k := int64(0); *i < len(items) && (bare.Incomplete || k < bare.Len); k++ {
			out.Items = append(out.Items, a.braceItem(bare.Elem, items, i, at, elided))
		}
	default:
		rec := types.AsRecord(bare)
		if rec == nil {
			break
		}
		for _, f := range aggregateFields(rec) {
			if *i >= len(items) {
				break
			}
			out.Items = append(out.Items, a.braceItem(f.Type, items, i, at, elided))
		}
	}
	if len(out.Items) > 0 {
		out.Span.Hi = out.Items[len(out.Items)-1].End()
	}
	return out
}

// braceItem is what initializes one element or member: the next item, or
// a list of the items a subaggregate takes.
func (a *Analyzer) braceItem(t types.Type, items []ast.Expr, i *int, at *ast.InitList, elided *bool) ast.Expr {
	item := items[*i]
	switch item.(type) {
	case *ast.InitList, *ast.DesignatedInit:
		*i++
		return item
	}
	if !isAggregateType(t) || a.initializesWhole(item, t) {
		*i++
		return item
	}
	*elided = true
	return a.braceInto(t, items, i, at, elided)
}

// initializesWhole reports whether an item initializes a subaggregate of
// type t by itself: a value of that class or one derived from it, or a
// string literal for an array of characters. The check it makes is kept,
// so the item is not checked a second time when its turn comes.
func (a *Analyzer) initializesWhole(item ast.Expr, t types.Type) bool {
	bare := types.Unqualify(t)
	if arr, isArr := bare.(*types.Array); isArr {
		if _, isStr := item.(*ast.StringLit); isStr {
			return isCharacterType(arr.Elem)
		}
		return false
	}
	info, done := a.prechecked[item]
	if !done {
		info = a.CheckExpr(item)
		if a.prechecked == nil {
			a.prechecked = map[ast.Expr]ExprInfo{}
		}
		a.prechecked[item] = info
	}
	if isDependentExpr(info) {
		return true
	}
	want := types.AsRecord(bare)
	have := types.AsRecord(types.Unqualify(types.RemoveReference(info.Type)))
	return want != nil && have != nil && (have == want || types.IsBaseOf(want, have))
}

// isAggregateType reports whether t is initialized from a braced list
// element by element: an array, or a class with no user-declared
// constructor.
func isAggregateType(t types.Type) bool {
	bare := types.Unqualify(t)
	if _, isArr := bare.(*types.Array); isArr {
		return true
	}
	rec := types.AsRecord(bare)
	return rec != nil && !hasUserConstructor(rec)
}

// isCharacterType reports whether t is one a string literal initializes
// an array of.
func isCharacterType(t types.Type) bool {
	b, ok := types.Unqualify(t).(*types.Basic)
	if !ok {
		return false
	}
	switch b.K {
	case types.Char, types.SChar, types.UChar, types.WChar, types.Char8, types.Char16, types.Char32:
		return true
	}
	return false
}
