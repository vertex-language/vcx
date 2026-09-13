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
