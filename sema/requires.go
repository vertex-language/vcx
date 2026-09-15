package sema

import (
	"fmt"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/constexpr"
	"github.com/vertex-language/vcx/types"
)

// satisfyConcept evaluates a concept-id with parameters bound to concrete types.
func (a *Analyzer) satisfyConcept(ctx *constexpr.Context, concept *ConceptSymbol, args []types.Type) (constexpr.Value, error) {
	bound := NewScope(concept.SymScope, BlockScope, nil)
	for i, param := range concept.Params {
		if param != nil && param.IsPack && param.SymName != "" {
			// `template <class _Tp, class... _Args> concept constructible_from`:
			// the pack takes the rest of the arguments, none at all in
			// `constructible_from<_Tp>`.
			pack := &types.Pack{}
			for _, t := range args[min(i, len(args)):] {
				if p, isPack := t.(*types.Pack); isPack {
					pack.Elems = append(pack.Elems, p.Elems...)
					continue
				}
				pack.Elems = append(pack.Elems, types.TemplateArg{IsType: true, Type: t})
			}
			bound.Insert(&TypeSymbol{SymName: param.SymName, SymType: pack, SymPos: param.SymPos})
			break
		}
		if param == nil || param.SymName == "" || i >= len(args) || args[i] == nil {
			continue
		}
		bound.Insert(&TypeSymbol{SymName: param.SymName, SymType: args[i], SymPos: param.SymPos})
	}
	return a.evalInScope(ctx, concept.Constraint, bound)
}

// conceptArgTypes resolves concept-id arguments as types within the current scope.
func (a *Analyzer) conceptArgTypes(args []ast.Node) ([]types.Type, error) {
	out := make([]types.Type, 0, len(args))
	for _, arg := range args {
		typeId, ok := arg.(*ast.TypeId)
		if !ok {
			return nil, fmt.Errorf("a concept-id with a non-type argument is not decidable here yet")
		}
		info := BuildDeclSpecs(typeId.Specs, a.curScope, a.unit)
		out = append(out, BuildDeclarator(typeId.Decl, info.Type, a.curScope, a.unit))
	}
	return out, nil
}

// satisfiesRequires evaluates a requires-expression under the current scope.
// Substitution failures in requirements result in false rather than ill-formed errors.
func (a *Analyzer) satisfiesRequires(re *ast.RequiresExpr) (bool, error) {
	// Forming the requirements is ordinary checking, and must not think
	// it is inside a template definition: a dependent answer here would
	// be neither true nor false.
	savedParams, savedRequires, savedScope := a.curTemplateParams, a.curTemplateRequires, a.curScope
	a.curTemplateParams, a.curTemplateRequires = nil, nil
	a.inRequires++
	defer func() {
		a.curTemplateParams, a.curTemplateRequires, a.curScope = savedParams, savedRequires, savedScope
		a.inRequires--
	}()

	// Parameters have function-prototype scope within the requires body.
	scope := NewScope(a.curScope, BlockScope, nil)
	for _, p := range re.Params {
		info := BuildDeclSpecs(p.Specs, scope, a.unit)
		if info.Unresolved != "" {
			return false, fmt.Errorf("the parameter type %s of the requires-expression does not resolve", info.Unresolved)
		}
		t := BuildDeclarator(p.Decl, info.Type, scope, a.unit)
		if pack := packIn(t); pack != nil && isPackDeclarator(p.Decl) && p.Decl.DeclName() != nil {
			// `requires(_Fn&& __fn, _Args&&... __args)`: a parameter pack,
			// one parameter per element, so `std::forward<_Args>(__args)...`
			// expands over it.
			name := NameString(p.Decl.DeclName(), a.unit)
			ps := &PackSymbol{SymName: name, SymScope: scope}
			for _, elem := range pack.Elems {
				ps.Elems = append(ps.Elems, &VarSymbol{SymName: name, SymType: substitutePack(t, elem.Type), SymScope: scope, IsParam: true})
			}
			scope.Insert(ps)
			continue
		}
		if t == nil || isDependentType(t) {
			return false, fmt.Errorf("a parameter of the requires-expression has no type under this binding")
		}
		name := ""
		if p.Decl != nil && p.Decl.DeclName() != nil {
			name = NameString(p.Decl.DeclName(), a.unit)
		}
		if name != "" {
			scope.Insert(&VarSymbol{SymName: name, SymType: t, SymScope: scope, IsParam: true})
		}
	}
	a.curScope = scope

	for _, req := range re.Reqs {
		ok, err := a.satisfiesRequirement(req)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func (a *Analyzer) satisfiesRequirement(req ast.Node) (bool, error) {
	switch r := req.(type) {
	case *ast.SimpleReq:
		// Simple requirement: expression must be valid.
		_, ok := a.wellFormedExpr(r.X)
		return ok, nil

	case *ast.TypeReq:
		// Type requirement: name must denote a valid type.
		ndiags := len(a.diags)
		spec := &ast.NamedTypeSpec{Span: r.Span, Typename: r.Typename, Name: r.Name}
		info := BuildDeclSpecs(&ast.DeclSpecs{Span: r.Span, List: []ast.DeclSpec{spec}}, a.curScope, a.unit)
		a.diags = a.diags[:ndiags]
		return info.Type != nil && info.Unresolved == "" && !isDependentType(info.Type), nil

	case *ast.CompoundReq:
		// Compound requirement: expression must be valid, noexcept if specified,
		// and satisfy any return-type-requirement.
		info, ok := a.wellFormedExpr(r.X)
		if !ok {
			return false, nil
		}
		if r.Noexcept.IsValid() && !a.isNoexcept(r.X) {
			return false, nil
		}
		if r.Ret == nil {
			return true, nil
		}
		return a.satisfiesTypeConstraint(r.Ret, a.decltypeParenthesized(info))

	case *ast.NestedReq:
		// Nested requirement: constraint expression must evaluate to true.
		ndiags := len(a.diags)
		v, err := a.NewConstContext().Eval(r.X)
		a.diags = a.diags[:ndiags]
		if err != nil {
			return false, fmt.Errorf("the nested requirement cannot be decided: %v", err)
		}
		return v.ToBool(), nil
	}
	return false, fmt.Errorf("unknown requirement %T", req)
}

// wellFormedExpr checks an expression and reports whether checking it said
// nothing -- which is what a requirement asks -- discarding what it said.
func (a *Analyzer) wellFormedExpr(e ast.Expr) (ExprInfo, bool) {
	ndiags := len(a.diags)
	// [expr.prim.req.general]/2: the expressions of a requires-expression
	// are unevaluated operands. `static_cast<_To>(std::declval<_From>())` in
	// convertible_to names declval without instantiating its body -- which
	// is a static_assert that fails, by design, when it is.
	a.unevaluated++
	info := a.CheckExpr(e)
	a.unevaluated--
	if len(a.diags) > ndiags {
		a.diags = a.diags[:ndiags]
		return info, false
	}
	return info, info.Type != nil && !isDependentExpr(info)
}

// decltypeParenthesized computes decltype((e)), reflecting value category as a reference.
func (a *Analyzer) decltypeParenthesized(info ExprInfo) types.Type {
	switch info.ValCat {
	case LValue:
		return types.AddLValueReference(info.Type)
	case XValue:
		return &types.RValueReference{Elem: info.Type}
	}
	return info.Type
}

// satisfiesTypeConstraint checks return-type-requirements (-> C<Args...>).
func (a *Analyzer) satisfiesTypeConstraint(constraint ast.Expr, first types.Type) (bool, error) {
	var name ast.Name
	var extra []ast.Node
	switch c := constraint.(type) {
	case *ast.TemplateName:
		name, extra = c.Name, c.Args
	case *ast.QualifiedName:
		name = c
		if tn, isTemplate := c.Name.(*ast.TemplateName); isTemplate {
			extra = tn.Args
			q := *c
			q.Name = tn.Name
			name = &q
		}
	case *ast.Ident:
		name = c
	default:
		return false, fmt.Errorf("the return-type-requirement is not a type-constraint")
	}
	var concept *ConceptSymbol
	var syms []Symbol
	if qn, qualified := name.(*ast.QualifiedName); qualified {
		syms = ResolveQualifiedName(qn, a.curScope, a.globalScope, a.unit)
	} else {
		syms = LookupUnqualified(a.curScope, NameString(name, a.unit))
	}
	for _, sym := range syms {
		if cs, isConcept := sym.(*ConceptSymbol); isConcept {
			concept = cs
			break
		}
	}
	if concept == nil {
		return false, fmt.Errorf("%s in the return-type-requirement does not name a concept", NameString(name, a.unit))
	}
	rest, err := a.conceptArgTypes(extra)
	if err != nil {
		return false, err
	}
	ndiags := len(a.diags)
	v, err := a.satisfyConcept(a.NewConstContext(), concept, append([]types.Type{first}, rest...))
	a.diags = a.diags[:ndiags]
	if err != nil {
		return false, err
	}
	return v.ToBool(), nil
}

// isNoexcept reports whether an expression is potentially-throwing.
func (a *Analyzer) isNoexcept(e ast.Expr) bool {
	if a.info == nil {
		return false
	}
	throwing := false
	ast.Inspect(e, func(n ast.Node) bool {
		if throwing {
			return false
		}
		var fn *FuncSymbol
		switch x := n.(type) {
		case *ast.CallExpr:
			fn = a.info.Calls[x]
		case *ast.NewExpr:
			fn = a.info.Allocs[x]
		case ast.Expr:
			fn = a.info.Operators[x]
		}
		if fn != nil && fn.FuncType != nil && !fn.FuncType.Noexcept {
			throwing = true
		}
		return true
	})
	return !throwing
}
