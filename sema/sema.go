package sema

import (
	"fmt"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/constexpr"
	"github.com/vertex-language/vcx/preprocessor"
	"github.com/vertex-language/vcx/sema/cfg"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// Diagnostic represents an error or warning emitted during semantic analysis.
type Diagnostic struct {
	Severity token.Severity
	Pos      ast.Tok
	Site     preprocessor.Site
	Message  string
}

func (d Diagnostic) String() string {
	if d.Site.Origin != nil && d.Site.Origin.File != nil {
		p := d.Site.Origin.File.Position(d.Site.Pos)
		return fmt.Sprintf("%s:%d:%d: %s: %s", p.Filename, p.Line, p.Column, d.Severity, d.Message)
	}
	return fmt.Sprintf("%s: %s", d.Severity, d.Message)
}

// Result holds the analyzed symbol table and AST.
type Result struct {
	GlobalScope *Scope
	File        *ast.File
	Model       types.Model
	Functions   []*FuncSymbol

	// Declared is every function declared in the translation unit in order.
	Declared []*FuncSymbol

	// Info contains side-table data produced by semantic analysis.
	Info *Info
}

// Info records semantic analysis results: expression types, name resolutions,
// and overload resolutions for lowering.
type Info struct {
	Types map[ast.Expr]types.Type
	Uses  map[ast.Expr]Symbol
	Calls map[*ast.CallExpr]*FuncSymbol

	// Ctors maps class declarators to their resolved constructors.
	Ctors map[*ast.InitDeclarator]*FuncSymbol

	// Temporaries maps functional cast CallExprs to resolved constructors.
	Temporaries map[*ast.CallExpr]*FuncSymbol

	// News maps NewExprs to constructors; Deletes maps DeleteExprs to destructors.
	News    map[*ast.NewExpr]*FuncSymbol
	Deletes map[*ast.DeleteExpr]*FuncSymbol

	// Operators maps expressions with class operands to resolved operator functions.
	Operators map[ast.Expr]*FuncSymbol

	// BindingInits maps structured bindings to hidden variable declarators.
	BindingInits map[*ast.StructuredBinding]*ast.InitDeclarator

	// Allocs maps placement new-expressions to resolved allocation functions.
	Allocs map[*ast.NewExpr]*FuncSymbol

	// Casts maps FunctionalCastExprs on classes to resolved constructors.
	Casts map[*ast.FunctionalCastExpr]*FuncSymbol

	// Conversions maps argument expressions to converting constructors.
	Conversions map[ast.Expr]*FuncSymbol

	// MemInits maps member initializers to resolved constructors.
	MemInits map[*ast.MemInit]*FuncSymbol

	// Ranges maps range-based for loops over classes to resolved range protocols.
	Ranges map[*ast.RangeForStmt]*RangeProtocol

	// MemberInits maps classes to default member initializers by member name.
	MemberInits map[*types.Record]map[string]*ast.InitDeclarator

	// MemberPointers maps pointer-to-member expressions (&C::m) to named members.
	MemberPointers map[*ast.UnaryExpr]*MemberRef

	// Lambdas maps lambda expressions to their closure records.
	Lambdas map[*ast.LambdaExpr]*LambdaInfo

	// Arrows maps class member access (x->m) to chained operator-> functions.
	Arrows map[*ast.MemberExpr][]*FuncSymbol

	// Rewrites maps expressions to rewritten comparison forms (e.g. != into !(==)).
	Rewrites map[ast.Expr]ast.Expr

	// TypeIds maps type-ids in expression context to their built types.
	TypeIds map[*ast.TypeId]types.Type

	// Consts records evaluated constant values of expressions.
	Consts map[ast.Expr]int64

	// Defs maps AST declaration nodes to the symbols they introduce.
	Defs map[ast.Node]Symbol
}

func newInfo() *Info {
	return &Info{
		Types:        map[ast.Expr]types.Type{},
		Uses:         map[ast.Expr]Symbol{},
		Calls:        map[*ast.CallExpr]*FuncSymbol{},
		Defs:         map[ast.Node]Symbol{},
		Ctors:        map[*ast.InitDeclarator]*FuncSymbol{},
		Temporaries:  map[*ast.CallExpr]*FuncSymbol{},
		News:         map[*ast.NewExpr]*FuncSymbol{},
		Allocs:       map[*ast.NewExpr]*FuncSymbol{},
		Operators:    map[ast.Expr]*FuncSymbol{},
		BindingInits: map[*ast.StructuredBinding]*ast.InitDeclarator{},
		Deletes:      map[*ast.DeleteExpr]*FuncSymbol{},
		Casts:        map[*ast.FunctionalCastExpr]*FuncSymbol{},
		Conversions:  map[ast.Expr]*FuncSymbol{},
		MemInits:     map[*ast.MemInit]*FuncSymbol{},
		Rewrites:     map[ast.Expr]ast.Expr{},
		Arrows:       map[*ast.MemberExpr][]*FuncSymbol{},
		Lambdas:      map[*ast.LambdaExpr]*LambdaInfo{},
		MemberPointers: map[*ast.UnaryExpr]*MemberRef{},
		MemberInits:  map[*types.Record]map[string]*ast.InitDeclarator{},
		Ranges:       map[*ast.RangeForStmt]*RangeProtocol{},
		TypeIds:      map[*ast.TypeId]types.Type{},
		Consts:       map[ast.Expr]int64{},
	}
}

// record notes what a name resolved to.
func (a *Analyzer) record(expr ast.Expr, sym Symbol) {
	if a.info != nil && sym != nil {
		a.info.Uses[expr] = sym
	}
}

// recordDef notes the declaration a declarator introduced.
func (a *Analyzer) recordDef(node ast.Node, sym Symbol) {
	if a.info != nil && node != nil && sym != nil {
		a.info.Defs[node] = sym
	}
}

// Analyzer maintains state during semantic analysis of a translation unit.
type Analyzer struct {
	unit        ast.Unit
	file        *ast.File
	model       types.Model
	globalScope *Scope
	curScope    *Scope
	curFunc     *FuncSymbol
	curRecord   *types.Record
	curAccess   types.Access
	loopDepth   int

	// externC indicates whether the current scope has extern "C" linkage.
	externC bool

	// Template instantiation state (see instantiate.go).
	instantiating bool
	primary       *RecordSymbol
	// instArgs are the arguments of the class specialization being
	// instantiated, for checkClassSpec to register it by (see there).
	instArgs []types.TemplateArg
	pending       map[*types.Method]*pendingBody

	// rangeElem is the deduced element type for range-based for loops.
	rangeElem types.Type

	// curTemplateParams holds active template parameters for concepts and nested declarations.
	curTemplateParams []*TemplateParamSymbol

	// methodSyms maps methods to their corresponding FuncSymbols.
	methodSyms map[*types.Method]*FuncSymbol

	// nestedBodies holds delayed member bodies of nested classes.
	nestedBodies []func()

	// definingFriend indicates friend function definition inside a class body.
	definingFriend bool

	// prechecked caches expanded pack arguments (see expandPackArgs).
	prechecked map[ast.Expr]ExprInfo

	// lastAnonymousRecord holds the record for an anonymous union/struct.
	lastAnonymousRecord *types.Record

	// unevaluated tracks depth of unevaluated operand contexts (decltype, sizeof, noexcept).
	unevaluated int

	// pendingInstances tracks template specializations waiting for definition.
	pendingInstances map[*FuncSymbol]*pendingInstance

	// curTemplateRequires holds the requires-clause of the active template-declaration.
	curTemplateRequires ast.Expr

	// instantiationDepth tracks recursive template instantiation depth to prevent stack overflow.
	instantiationDepth int

	// templateOwner is the declaration enclosed by the template-declaration.
	templateOwner ast.Decl

	// info holds semantic analysis results.
	info *Info

	// nlambdas numbers the unit's closure types, `<lambda_1>` onward as
	// cl spells them; lambdas is the stack of the lambdas whose bodies
	// are being checked, innermost last, for implicit captures.
	nlambdas int
	lambdas  []*LambdaInfo

	functions []*FuncSymbol
	declared  []*FuncSymbol
	seenDecl  map[*FuncSymbol]bool
	diags     []Diagnostic
}

// NewAnalyzer creates an Analyzer for unit under the given target model.
func NewAnalyzer(u ast.Unit, model types.Model) *Analyzer {
	global := NewScope(nil, GlobalScope, nil)
	a := &Analyzer{
		unit:        u,
		model:       model,
		globalScope: global,
		curScope:    global,
		curAccess:   types.AccessPublic,
		info:        newInfo(),
	}
	a.declareBuiltinTypes()
	a.declareLibraryBuiltins()
	a.declareBuiltinTemplates()
	global.Decltype = func(e ast.Expr, scope *Scope) types.Type {
		// The operand means what it means where the type was written --
		// a trailing return type sees the parameters -- so it is checked
		// there.
		saved := a.curScope
		if scope != nil {
			a.curScope = scope
		}
		a.unevaluated++
		defer func() { a.curScope = saved; a.unevaluated-- }()
		return a.decltypeOf(e)
	}
	global.Instantiate = func(tmpl *RecordSymbol, args []types.TemplateArg, at ast.Tok) types.Type {
		if args == nil || a.dependentContext() {
			return nil
		}
		inst := a.instantiateClass(tmpl, args, at)
		if inst == nil {
			return nil
		}
		return inst.Record
	}
	global.AliasInstantiate = func(alias *TypeSymbol, args []types.TemplateArg, at ast.Tok) types.Type {
		if args == nil || a.dependentContext() {
			return nil
		}
		return a.instantiateAlias(alias, args, at)
	}
	global.EvalConst = func(e ast.Expr, scope *Scope) (int64, bool) {
		// The argument means what it means where the template-id was
		// written -- an alias template's B is bound in a scope of its
		// own -- so the evaluation runs there.
		saved := a.curScope
		if scope != nil {
			a.curScope = scope
		}
		defer func() { a.curScope = saved }()
		if a.dependentContext() && isDependentExpr(a.CheckExpr(e)) {
			return 0, false
		}
		n, err := a.NewConstContext().EvalInt(e)
		return n, err == nil
	}
	global.ExpandValues = func(pe *ast.PackExpansion, scope *Scope) ([]int64, bool) {
		saved := a.curScope
		if scope != nil {
			a.curScope = scope
		}
		defer func() { a.curScope = saved }()
		elems, ok := a.expandOne(pe)
		if !ok {
			return nil, false
		}
		// Each element was checked in the scope binding its packs, so its
		// value is read from that check rather than evaluated again here,
		// where the pack names are unbound.
		out := make([]int64, 0, len(elems))
		for _, e := range elems {
			info := a.prechecked[e]
			if !info.IsConst {
				return nil, false
			}
			out = append(out, info.ConstVal)
		}
		return out, true
	}
	return a
}

// NewConstContext creates an evaluation context connected to the analyzer's symbols.
func (a *Analyzer) NewConstContext() *constexpr.Context {
	ctx := constexpr.NewContext(a.unit, a.model)
	ctx.ResolveVar = func(name string) (constexpr.Value, error) {
		syms := LookupUnqualified(a.curScope, name)
		for _, s := range syms {
			if vs, ok := s.(*VarSymbol); ok && vs.HasKnownValue {
				return constexpr.NewInt(vs.KnownValue, vs.SymType, a.model), nil
			}
			if vs, ok := s.(*VarSymbol); ok && vs.Init != nil {
				return a.evalInScope(ctx, vs.Init, vs.SymScope)
			}
			if vs, ok := s.(*VarSymbol); ok && vs.BracedInit != nil && (vs.Constexpr || types.IsConst(vs.SymType)) {
				// Aggregate elements left out by the list are value-initialized.
				v, err := a.evalInScope(ctx, vs.BracedInit, vs.SymScope)
				if err != nil {
					return nil, err
				}
				if arr, isArr := types.Unqualify(vs.SymType).(*types.Array); isArr {
					if av, isAV := v.(constexpr.ArrayValue); isAV && arr.Len > int64(len(av.Elements)) {
						elems := make([]constexpr.Value, arr.Len)
						copy(elems, av.Elements)
						for i := int64(len(av.Elements)); i < arr.Len; i++ {
							elems[i] = constexpr.NewInt(0, types.Unqualify(arr.Elem), a.model)
						}
						v = constexpr.NewArray(elems, types.Unqualify(arr.Elem))
					}
				}
				return v, nil
			}
			if es, ok := s.(*EnumeratorSymbol); ok {
				return constexpr.NewInt(es.Val, es.Type(), a.model), nil
			}
		}
		return nil, fmt.Errorf("variable %q not found or not constant", name)
	}
	ctx.ResolveTrait = a.resolveTrait
	ctx.ExpandFold = func(f *ast.FoldExpr) (constexpr.Value, error) {
		return a.expandFold(ctx, f)
	}
	ctx.Folded = func(e ast.Expr) (int64, bool) {
		if a.info != nil {
			if n, known := a.info.Consts[e]; known {
				return n, true
			}
		}
		if s, isSizeof := e.(*ast.SizeofExpr); isSizeof && s.Ellipsis.IsValid() {
			return a.packLength(s.X)
		}
		return 0, false
	}
	ctx.TypeOfExpr = func(e ast.Expr) types.Type {
		if a.info == nil {
			return nil
		}
		if t, known := a.info.Types[e]; known && !isDependentType(t) {
			return t
		}
		// Check unevaluated operand on demand for constant evaluation.
		if a.dependentContext() {
			return nil
		}
		ndiags := len(a.diags)
		a.unevaluated++
		info := a.CheckExpr(e)
		a.unevaluated--
		if len(a.diags) > ndiags || info.Type == nil || isDependentType(info.Type) {
			a.diags = a.diags[:ndiags]
			return nil
		}
		return info.Type
	}
	ctx.ResolveConcept = func(tn *ast.TemplateName) (constexpr.Value, bool, error) {
		name := NameString(tn.Name, a.unit)
		var concept *ConceptSymbol
		for _, sym := range LookupUnqualified(a.curScope, name) {
			if cs, ok := sym.(*ConceptSymbol); ok {
				concept = cs
				break
			}
		}
		if concept == nil {
			return a.resolveVarTemplate(tn, ctx)
		}
		// Evaluate concept constraint with bound template arguments.
		args, err := a.conceptArgTypes(tn.Args)
		if err != nil {
			return nil, false, err
		}
		v, err := a.satisfyConcept(ctx, concept, args)
		if err != nil {
			return nil, false, err
		}
		return v, true, nil
	}
	ctx.ResolveRequires = a.satisfiesRequires
	ctx.ResolveQualified = func(qn *ast.QualifiedName) (constexpr.Value, error) {
		syms := ResolveQualifiedName(qn, a.curScope, a.globalScope, a.unit)
		// `std::is_void_v<int>` -- a template-id at the end names a
		// variable template's instance, not the template.
		if tn, isTemplate := qn.Name.(*ast.TemplateName); isTemplate {
			v, found, err := a.resolveVarTemplateAmong(tn, syms, ctx)
			if err != nil {
				return nil, err
			}
			if found {
				return v, nil
			}
		}
		for _, sym := range syms {
			switch s := sym.(type) {
			case *EnumeratorSymbol:
				return constexpr.NewInt(s.Val, s.Type(), a.model), nil
			case *VarSymbol:
				if s.HasKnownValue {
					return constexpr.NewInt(s.KnownValue, s.SymType, a.model), nil
				}
				if s.Init != nil {
					// The initializer means what it meant where it was
					// written: `value = V` in a specialization reads V
					// from the scope that bound it, not from here.
					return a.evalInScope(ctx, s.Init, s.SymScope)
				}
			}
		}
		return nil, fmt.Errorf("qualified name %q is not a constant", NameString(qn, a.unit))
	}
	ctx.ResolveType = func(id *ast.TypeId) (types.Type, bool) {
		if id == nil {
			return nil, false
		}
		info := BuildDeclSpecs(id.Specs, a.curScope, a.unit)
		if info.Unresolved != "" {
			return nil, false
		}
		t := BuildDeclarator(id.Decl, info.Type, a.curScope, a.unit)
		return t, t != nil
	}
	ctx.ResolveTypeExpr = func(e ast.Expr) (types.Type, bool) {
		return a.calleeNamesAType(e)
	}
	funcInfo := func(fs *FuncSymbol) *constexpr.FuncInfo {
		paramNames := make([]string, len(fs.Params))
		paramTypes := make([]types.Type, len(fs.Params))
		for i, p := range fs.Params {
			paramNames[i] = p.SymName
			paramTypes[i] = p.SymType
		}
		return &constexpr.FuncInfo{
			Unit:       a.unit,
			Name:       fs.SymName,
			ParamNames: paramNames,
			ParamTypes: paramTypes,
			Body:       fs.Body,
			RetType:    fs.FuncType.Ret,
		}
	}
	ctx.ResolveCall = func(c *ast.CallExpr) (*constexpr.FuncInfo, bool) {
		// The function the analysis chose for this call -- by overload
		// resolution, with its constraints, as the specialization it
		// instantiated -- which a lookup by name would not find again.
		if a.info == nil {
			return nil, false
		}
		fs := a.info.Calls[c]
		if fs == nil || !(fs.Constexpr || fs.Consteval) || fs.Body == nil || fs.InClass != nil {
			return nil, false
		}
		return funcInfo(fs), true
	}
	ctx.ResolveFunc = func(name string) (*constexpr.FuncInfo, error) {
		syms := LookupUnqualified(a.curScope, name)
		for _, s := range syms {
			if fs, ok := s.(*FuncSymbol); ok && (fs.Constexpr || fs.Consteval) && fs.Body != nil {
				return funcInfo(fs), nil
			}
		}
		return nil, fmt.Errorf("constexpr function %q not found", name)
	}
	return ctx
}

func (a *Analyzer) errorAt(pos ast.Tok, msg string) {
	site := preprocessor.Site{}
	if a.unit != nil && pos.IsValid() {
		if u, ok := a.unit.(interface {
			Site(ast.Tok) preprocessor.Site
		}); ok {
			site = u.Site(pos)
		}
	}
	a.diags = append(a.diags, Diagnostic{
		Severity: token.Error,
		Pos:      pos,
		Site:     site,
		Message:  msg,
	})
}

func (a *Analyzer) warnAt(pos ast.Tok, msg string) {
	site := preprocessor.Site{}
	if a.unit != nil && pos.IsValid() {
		if u, ok := a.unit.(interface {
			Site(ast.Tok) preprocessor.Site
		}); ok {
			site = u.Site(pos)
		}
	}
	a.diags = append(a.diags, Diagnostic{
		Severity: token.Warn,
		Pos:      pos,
		Site:     site,
		Message:  msg,
	})
}

// Analyze performs semantic analysis and type checking over file.
func Analyze(file *ast.File, model types.Model) (*Result, []Diagnostic) {
	if file == nil {
		return nil, nil
	}

	a := NewAnalyzer(file.Unit, model)
	a.file = file

	for _, decl := range file.Decls {
		a.CheckDecl(decl)
	}

	for _, fn := range a.functions {
		if fn.Body != nil {
			funcCFG := cfg.Build(fn.Body, a.unit)
			retDiags := cfg.CheckReturns(funcCFG, fn.Name(), fn.FuncType.Ret)
			for _, rd := range retDiags {
				a.errorAt(fn.Pos(), rd)
			}
			unreachable := cfg.CheckUnreachable(funcCFG)
			for _, uPos := range unreachable {
				a.warnAt(uPos, "unreachable code")
			}
		}
	}

	res := &Result{
		GlobalScope: a.globalScope,
		Info:        a.info,
		File:        file,
		Model:       model,
		Functions:   a.functions,
		Declared:    a.declared,
	}

	return res, a.diags
}

// dependentContext reports whether the analysis is inside a template
// being checked as written -- where an argument may be a T, and a
// specialization deduced from it would be a specialization on nothing.
func (a *Analyzer) dependentContext() bool {
	return a.curTemplateParams != nil
}

// decltypeOf returns the decltype of an expression, considering declared entity type
// for unparenthesized names or adjusting by value category otherwise.
func (a *Analyzer) decltypeOf(e ast.Expr) types.Type {
	info := a.CheckExpr(e)
	if info.Type == nil {
		return types.Typ(types.AutoKind)
	}
	switch e.(type) {
	case *ast.Ident, *ast.QualifiedName:
		// The declared type (read directly off the symbol).
		if v, isVar := a.info.Uses[e].(*VarSymbol); isVar {
			return v.SymType
		}
		return info.Type
	case *ast.MemberExpr:
		return info.Type
	}
	if _, isRef := info.Type.(*types.LValueReference); isRef {
		return info.Type
	}
	if _, isRef := info.Type.(*types.RValueReference); isRef {
		return info.Type
	}
	switch info.ValCat {
	case LValue:
		return &types.LValueReference{Elem: info.Type}
	case XValue:
		return &types.RValueReference{Elem: info.Type}
	}
	return info.Type
}

// noteDeclared records a function symbol the unit declared, once.
func (a *Analyzer) noteDeclared(fn *FuncSymbol) {
	if a.seenDecl == nil {
		a.seenDecl = map[*FuncSymbol]bool{}
	}
	if fn == nil || a.seenDecl[fn] {
		return
	}
	a.seenDecl[fn] = true
	a.declared = append(a.declared, fn)
}

// evalInScope evaluates an initializer with the analysis's scope set to
// where the initializer was written, so that the names in it resolve as
// they did then.
func (a *Analyzer) evalInScope(ctx *constexpr.Context, e ast.Expr, scope *Scope) (constexpr.Value, error) {
	if scope == nil || scope == a.curScope {
		return ctx.Eval(e)
	}
	saved := a.curScope
	a.curScope = scope
	defer func() { a.curScope = saved }()
	return ctx.Eval(e)
}

// resolveVarTemplate answers a template-id in a constant expression that
// names a variable template's instance: `is_void_v<int>` is the value of
// that instance's initializer, evaluated where its parameters are bound.
func (a *Analyzer) resolveVarTemplate(tn *ast.TemplateName, ctx *constexpr.Context) (constexpr.Value, bool, error) {
	var syms []Symbol
	switch n := tn.Name.(type) {
	case *ast.QualifiedName:
		syms = ResolveQualifiedName(n, a.curScope, a.globalScope, a.unit)
	default:
		syms = LookupUnqualified(a.curScope, NameString(tn.Name, a.unit))
	}
	return a.resolveVarTemplateAmong(tn, syms, ctx)
}

// resolveVarTemplateAmong is resolveVarTemplate for a template-id whose
// template was already looked up -- `std::is_void_v<int>`, where the
// qualified name found it.
func (a *Analyzer) resolveVarTemplateAmong(tn *ast.TemplateName, syms []Symbol, ctx *constexpr.Context) (constexpr.Value, bool, error) {
	for _, sym := range syms {
		v, isVar := sym.(*VarSymbol)
		if !isVar || v.Template == nil {
			continue
		}
		args := templateArgs(tn, a.curScope, a.unit)
		if args == nil || argsDependent(args) {
			return nil, false, fmt.Errorf("%w: %s depends on a template parameter", constexpr.ErrNonConstexpr, NameString(tn, a.unit))
		}
		inst := a.instantiateVar(v, args, tn.Pos())
		if inst == nil || inst.Init == nil {
			return nil, false, fmt.Errorf("%w: %s has no instance", constexpr.ErrNonConstexpr, NameString(tn, a.unit))
		}
		val, err := a.evalInScope(ctx, inst.Init, inst.SymScope)
		return val, true, err
	}
	return nil, false, nil
}

// expandFold expands and evaluates fold expressions over parameter packs.
func (a *Analyzer) expandFold(ctx *constexpr.Context, f *ast.FoldExpr) (constexpr.Value, error) {
	// The pattern is the operand that mentions a pack; the other, if
	// there is one, is the initial value.
	pattern, init := f.Left, f.Right
	packs := a.packsIn(pattern)
	leftFold := false // (... op E): combine from the left
	if len(packs) == 0 {
		pattern, init = f.Right, f.Left
		packs = a.packsIn(pattern)
		leftFold = true
	}
	if len(packs) == 0 {
		return nil, fmt.Errorf("%w: a fold expression's pattern names no parameter pack", constexpr.ErrNonConstexpr)
	}
	if f.Left != nil && f.Right != nil {
		// A binary fold: the init sits on the side the ellipsis leans
		// away from, and the combining runs from the init outwards.
		leftFold = init == f.Left
	}
	n := -1
	for _, p := range packs {
		if n >= 0 && n != len(p.pack.Elems) {
			return nil, fmt.Errorf("%w: the packs in a fold expression differ in length", constexpr.ErrNonConstexpr)
		}
		n = len(p.pack.Elems)
	}

	var values []constexpr.Value
	for i := 0; i < n; i++ {
		bound := NewScope(a.curScope, BlockScope, nil)
		for _, p := range packs {
			elem := p.pack.Elems[i]
			if elem.IsType {
				bound.Insert(&TypeSymbol{SymName: p.name, SymType: elem.Type, SymScope: bound})
			} else {
				bound.Insert(&VarSymbol{SymName: p.name, SymType: elem.ValType, SymScope: bound, Constexpr: true, KnownValue: elem.Val, HasKnownValue: true})
			}
		}
		v, err := a.evalInScope(ctx, ast.Clone(pattern), bound)
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	if init != nil {
		v, err := ctx.Eval(init)
		if err != nil {
			return nil, err
		}
		if leftFold {
			values = append([]constexpr.Value{v}, values...)
		} else {
			values = append(values, v)
		}
	}
	if len(values) == 0 {
		switch f.Op {
		case token.LAND:
			return constexpr.NewBool(true), nil
		case token.LOR:
			return constexpr.NewBool(false), nil
		case token.COMMA:
			return constexpr.VoidValue{}, nil
		}
		return nil, fmt.Errorf("%w: a fold of an empty pack over %s has no value", constexpr.ErrNonConstexpr, f.Op)
	}
	if leftFold {
		acc := values[0]
		for _, v := range values[1:] {
			var err error
			if acc, err = ctx.Combine(f.Op, acc, v); err != nil {
				return nil, err
			}
		}
		return acc, nil
	}
	acc := values[len(values)-1]
	for i := len(values) - 2; i >= 0; i-- {
		var err error
		if acc, err = ctx.Combine(f.Op, values[i], acc); err != nil {
			return nil, err
		}
	}
	return acc, nil
}

// A boundPack is a parameter pack's name in the current scope and what
// it is bound to.
type boundPack struct {
	name string
	pack *types.Pack
}

// packsIn finds the parameter packs an expression mentions: every name
// in it that resolves to a pack bound in the current scope.
func (a *Analyzer) packsIn(e ast.Expr) []boundPack {
	if e == nil {
		return nil
	}
	var packs []boundPack
	seen := map[string]bool{}
	ast.Inspect(e, func(n ast.Node) bool {
		id, isIdent := n.(*ast.Ident)
		if !isIdent {
			return true
		}
		name := id.Text(a.unit)
		if seen[name] {
			return true
		}
		for _, sym := range LookupUnqualified(a.curScope, name) {
			if ts, isType := sym.(*TypeSymbol); isType {
				if pack, isPack := ts.SymType.(*types.Pack); isPack {
					seen[name] = true
					packs = append(packs, boundPack{name, pack})
				}
			}
			break
		}
		return true
	})
	return packs
}

// declareBuiltinTypes enters the type names a compiler declares before the
// first line of any unit: the ones gcc and clang predeclare and system
// headers use without declaring, which are reserved names and so cost a
// program nothing to have everywhere. __builtin_va_list is the calling
// convention's va_list, and the 128-bit integers exist where a pointer is
// sixty-four bits.
func (a *Analyzer) declareBuiltinTypes() {
	g := a.globalScope
	g.Insert(&TypeSymbol{SymName: "__builtin_va_list", SymType: a.model.BuiltinVaList(), SymScope: g})
	if a.model.SizePtr == 8 {
		g.Insert(&TypeSymbol{SymName: "__int128_t", SymType: types.Typ(types.Int128), SymScope: g})
		g.Insert(&TypeSymbol{SymName: "__uint128_t", SymType: types.Typ(types.UInt128), SymScope: g})
	}
}
