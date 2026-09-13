package sema

import (
	"fmt"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// Structured bindings.

// BindingInfo describes one name of a structured binding.
type BindingInfo struct {
	Hidden *VarSymbol  // variable initialized from the initializer
	Index  int         // element or member index
	Get    *FuncSymbol // get<Index>(hidden) for tuple-like types
}

// checkStructuredBinding declares a structured binding's names.
func (a *Analyzer) checkStructuredBinding(d *ast.StructuredBinding) {
	if d.Value == nil && d.Braced == nil && len(d.Args) == 0 {
		a.errorAt(d.Pos(), "a structured binding needs an initializer")
		return
	}
	var initInfo ExprInfo
	var initExpr ast.Expr
	switch {
	case d.Value != nil:
		initExpr = d.Value
	case len(d.Args) == 1:
		initExpr = d.Args[0]
	case d.Braced != nil && len(d.Braced.Items) == 1:
		initExpr = d.Braced.Items[0]
	default:
		a.errorAt(d.Pos(), "a structured binding is initialized from one expression")
		return
	}
	initInfo = a.CheckExpr(initExpr)

	// Deduce the hidden variable's type from the initializer.
	declared := types.Type(types.Typ(types.AutoKind))
	if d.Ref.IsValid() {
		if d.RefKind != token.LAND {
			declared = types.AddLValueReference(declared)
		} else {
			declared = types.AddRValueReference(declared)
		}
	}
	if isDependentExpr(initInfo) || a.dependentContext() && isDependentType(initInfo.Type) {
		for _, name := range d.Names {
			a.curScope.Insert(&VarSymbol{SymName: name.Text(a.unit), SymType: &types.DependentType{Name: "binding"}, SymPos: name.Pos(), SymScope: a.curScope})
		}
		return
	}
	hiddenType := a.deduceAuto(declared, initInfo)
	if !d.Ref.IsValid() {
		if arr, isArr := types.Unqualify(types.RemoveReference(initInfo.Type)).(*types.Array); isArr {
			hiddenType = arr
		}
	}
	hidden := &VarSymbol{SymName: "", SymType: hiddenType, SymPos: d.Pos(), SymScope: a.curScope, Init: initExpr}
	a.recordDef(d, hidden)
	// Hidden variable declarator for lowering.
	init := &ast.InitDeclarator{Span: d.Span, Assign: d.Assign, Value: initExpr}
	a.recordDef(init, hidden)
	if a.info != nil {
		a.info.BindingInits[d] = init
	}
	if rec := types.AsRecord(types.Unqualify(hiddenType)); rec != nil {
		a.resolveConstructor(init, rec)
	}

	elem := types.Unqualify(types.RemoveReference(hiddenType))
	quals := qualsOf(types.RemoveReference(hiddenType))
	switch e := elem.(type) {
	case *types.Array:
		// One name per element.
		if e.Incomplete || int(e.Len) != len(d.Names) {
			a.errorAt(d.Pos(), fmt.Sprintf("a structured binding of %d names on an array of %d", len(d.Names), e.Len))
			return
		}
		for i, name := range d.Names {
			a.declareBinding(name, types.Qualify(e.Elem, quals), &BindingInfo{Hidden: hidden, Index: i})
		}
		return
	case *types.Record:
		if !e.Complete {
			a.errorAt(d.Pos(), fmt.Sprintf("a structured binding on the incomplete class %s", e.Name))
			return
		}
		// Tuple-like class or non-static data members decomposition.
		if n, tupleLike := a.tupleSize(e); tupleLike {
			if n != len(d.Names) {
				a.errorAt(d.Pos(), fmt.Sprintf("a structured binding of %d names on a tuple of %d", len(d.Names), n))
				return
			}
			for i, name := range d.Names {
				get := a.resolveGet(e, i, hiddenType, initInfo)
				if get == nil {
					a.errorAt(name.Pos(), fmt.Sprintf("no get<%d> for %s", i, e.Name))
					continue
				}
				ret := get.FuncType.Ret
				if !types.IsReference(ret) {
					ret = types.AddRValueReference(ret)
				}
				a.declareBinding(name, ret, &BindingInfo{Hidden: hidden, Index: i, Get: get})
			}
			return
		}
		fields := a.bindableMembers(e)
		if fields == nil {
			a.errorAt(d.Pos(), fmt.Sprintf("%s cannot be decomposed: its members are not all public and its own", e.Name))
			return
		}
		if len(fields) != len(d.Names) {
			a.errorAt(d.Pos(), fmt.Sprintf("a structured binding of %d names on a class with %d members", len(d.Names), len(fields)))
			return
		}
		for i, name := range d.Names {
			a.declareBinding(name, types.Qualify(fields[i].Type, quals), &BindingInfo{Hidden: hidden, Index: i})
		}
		return
	}
	a.errorAt(d.Pos(), fmt.Sprintf("cannot decompose a value of type %s", hiddenType))
}

// declareBinding inserts one binding's name: a variable whose storage
// is a part of the hidden one (see lowering's bindingAddr).
func (a *Analyzer) declareBinding(name *ast.Ident, t types.Type, info *BindingInfo) {
	sym := &VarSymbol{SymName: name.Text(a.unit), SymType: t, SymPos: name.Pos(), SymScope: a.curScope, Binding: info}
	a.recordDef(name, sym)
	if err := a.curScope.Insert(sym); err != nil {
		a.errorAt(name.Pos(), err.Error())
	}
}

// bindableMembers returns public non-static data members for structured binding decomposition.
func (a *Analyzer) bindableMembers(rec *types.Record) []types.Field {
	var fields []types.Field
	owner := rec
	for len(owner.Fields) == 0 && len(owner.Bases) == 1 {
		br := types.AsRecord(types.Unqualify(owner.Bases[0].Type))
		if br == nil {
			break
		}
		owner = br
	}
	for _, f := range owner.Fields {
		if f.Access != types.AccessPublic {
			return nil
		}
		if f.Name == "" {
			if inner := types.AsRecord(types.Unqualify(f.Type)); inner != nil {
				fields = append(fields, inner.Fields...)
				continue
			}
		}
		fields = append(fields, f)
	}
	return fields
}

// tupleSize returns std::tuple_size<E>::value for a tuple-like class.
func (a *Analyzer) tupleSize(rec *types.Record) (int, bool) {
	var tmpl *RecordSymbol
	for _, sym := range a.lookupInStd("tuple_size") {
		if rs, isRec := sym.(*RecordSymbol); isRec && rs.ClassTemplate != nil {
			tmpl = rs
			break
		}
	}
	if tmpl == nil {
		return 0, false
	}
	// Only an explicit or partial specialization counts.
	args := []types.TemplateArg{{IsType: true, Type: rec}}
	key := argsKey(args)
	if _, has := tmpl.ClassTemplate.Explicit[key]; !has {
		if _, _, matched := a.matchPartial(tmpl.ClassTemplate, args); !matched {
			return 0, false
		}
	}
	inst := a.instantiateClass(tmpl, args, 0)
	if inst == nil {
		return 0, false
	}
	for _, sym := range LookupQualified(inst, "value") {
		if v, isVar := sym.(*VarSymbol); isVar {
			if v.HasKnownValue {
				return int(v.KnownValue), true
			}
			if v.Init != nil {
				if val, err := a.evalInScope(a.NewConstContext(), v.Init, v.SymScope); err == nil {
					if iv, isInt := val.(interface{ Int64() int64 }); isInt {
						return int(iv.Int64()), true
					}
				}
			}
		}
	}
	return 0, false
}

// lookupInStd looks up a symbol in namespace std.
func (a *Analyzer) lookupInStd(name string) []Symbol {
	for _, sym := range a.globalScope.LookupLocal("std") {
		if ns, isNs := sym.(*NamespaceSymbol); isNs && ns.InnerScope != nil {
			return ns.InnerScope.LookupNamespaceMember(name)
		}
	}
	return nil
}

// resolveGet resolves the get<i>(e) call for a tuple-like binding.
func (a *Analyzer) resolveGet(rec *types.Record, i int, hiddenType types.Type, init ExprInfo) *FuncSymbol {
	arg := Argument{Type: types.RemoveReference(hiddenType), IsLValue: true}
	if types.IsRValueReference(hiddenType) {
		arg.IsLValue = false
	}
	var candidates []*FuncSymbol
	seen := map[*FuncSymbol]bool{}
	add := func(syms []Symbol) {
		for _, sym := range syms {
			if fn, isFn := sym.(*FuncSymbol); isFn && fn.Template != nil && fn.InClass == nil && !seen[fn] {
				if len(fn.Template.Params) > 0 && !fn.Template.Params[0].IsType {
					seen[fn] = true
					candidates = append(candidates, fn)
				}
			}
		}
	}
	if rs := a.curScope.recordSymbol(rec); rs != nil && rs.SymScope != nil {
		ns := rs.SymScope
		for ns != nil && ns.Kind != NamespaceScope && ns.Kind != GlobalScope {
			ns = ns.Parent
		}
		if ns != nil {
			add(ns.LookupAssociated("get"))
		}
	}
	add(a.lookupInStd("get"))
	add(LookupUnqualified(a.curScope, "get"))

	var best *FuncSymbol
	for _, cand := range candidates {
		idx := cand.Template.Params[0]
		b := Binding{idx.SymName: &valueBound{Type: idx.SymType, Val: int64(i)}}
		if !deduceArg(cand.FuncType.Params[0].Type, arg, b) {
			continue
		}
		targs, err := a.templateArgsFor(cand, b)
		if err != nil {
			continue
		}
		sig, ok := a.instanceSignature(cand, targs)
		if !ok || len(sig.Params) != 1 {
			continue
		}
		if !ClassifyConversion(arg.Type, sig.Params[0].Type, arg.IsLValue).Valid {
			continue
		}
		if inst := a.instantiateWithArgs(cand, targs, 0); inst != nil {
			best = inst
			break
		}
	}
	return best
}
