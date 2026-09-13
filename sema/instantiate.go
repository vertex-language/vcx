package sema

import (
	"fmt"
	"os"
	"strings"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/types"
)

// TemplateInfo is what a function template keeps for its instantiations.
type TemplateInfo struct {
	Params []*TemplateParamSymbol
	Decl   *ast.FuncDecl

	// Scope is where the template's name lives, which is where an
	// instantiation's lookups begin after the parameters.
	Scope *Scope

	// Instances are the specializations made so far, by their arguments.
	Instances map[string]*FuncSymbol
}

// ClassTemplateInfo is the same for a class template.
type ClassTemplateInfo struct {
	Params    []*TemplateParamSymbol
	Spec      *ast.ClassSpec
	Scope     *Scope
	Instances map[string]*RecordSymbol

	// Explicit full specializations and Partials partial specializations.
	Explicit map[string]*ast.ClassSpec
	Partials []*PartialSpec
}

// A PartialSpec is `template <class T> struct X<T *, 4>`: its own
// parameters, the argument pattern, and the body.
type PartialSpec struct {
	Params []*TemplateParamSymbol
	Args   []types.TemplateArg
	Spec   *ast.ClassSpec
	Scope  *Scope
}

// instantiateFunction makes -- or finds -- the specialization of a
// function template for one binding of its parameters.
func (a *Analyzer) instantiateFunction(tmpl *FuncSymbol, b Binding, at ast.Tok) *FuncSymbol {
	args, err := a.templateArgsFor(tmpl, b)
	if err != nil {
		a.errorAt(at, err.Error())
		return nil
	}
	return a.instantiateWithArgs(tmpl, args, at)
}

// templateArgsFor builds template arguments from parameter bindings and defaults.
func (a *Analyzer) templateArgsFor(tmpl *FuncSymbol, b Binding) ([]types.TemplateArg, error) {
	info := tmpl.Template
	args := make([]types.TemplateArg, len(info.Params))
	for i, p := range info.Params {
		t, bound := b[p.SymName]
		switch {
		case p.IsPack && !bound:
			// Empty pack.
			args[i] = types.TemplateArg{IsType: true, Type: &types.Pack{}}
		case bound && t != nil && p.IsType:
			args[i] = types.TemplateArg{IsType: true, Type: t}
		case bound && t != nil:
			vb, isVal := t.(*valueBound)
			if !isVal {
				return nil, fmt.Errorf("cannot instantiate %s: template parameter %s is a value; a type was deduced", tmpl.SymName, p.SymName)
			}
			args[i] = types.TemplateArg{Val: vb.Val, ValType: a.valueParamType(p, info.Params[:i], args[:i], info.Scope)}
		case p.Default != nil:
			arg, ok := a.defaultTemplateArg(p, info.Params[:i], args[:i], info.Scope)
			if !ok {
				return nil, fmt.Errorf("cannot instantiate %s: the default argument of template parameter %d does not substitute", tmpl.SymName, i+1)
			}
			args[i] = arg
		case !p.IsType:
			return nil, fmt.Errorf("cannot instantiate %s: non-type template parameter %s is not deduced yet", tmpl.SymName, p.SymName)
		default:
			return nil, fmt.Errorf("cannot instantiate %s: template parameter %s was not deduced", tmpl.SymName, p.SymName)
		}
	}
	return args, nil
}

// instanceSignature returns the substituted function signature for overload ranking.
func (a *Analyzer) instanceSignature(tmpl *FuncSymbol, args []types.TemplateArg) (*types.Func, bool) {
	info := tmpl.Template
	if inst, done := info.Instances[argsKey(args)]; done {
		return inst.FuncType, true
	}
	decl := info.Decl
	if decl == nil {
		// Declared and never defined -- `template <class T> void f(T);`
		// -- the signature is the declaration's with the arguments put
		// in for the parameters.
		b := Binding{}
		for i, p := range info.Params {
			if i < len(args) && args[i].IsType {
				b[p.SymName] = args[i].Type
			}
		}
		ft, isFunc := substitute(tmpl.FuncType, b).(*types.Func)
		if !isFunc || isDependent(ft) {
			return nil, false
		}
		return ft, true
	}
	paramScope := a.bindTemplateArgs(info.Scope, info.Params, args)
	declInfo := BuildDeclSpecs(decl.Specs, paramScope, a.unit)
	if declInfo.Unresolved != "" {
		return nil, false
	}
	ft, isFunc := BuildDeclarator(decl.Decl, declInfo.Type, paramScope, a.unit).(*types.Func)
	if !isFunc || isDependent(ft) {
		return nil, false
	}
	if tmpl.InClass != nil && (tmpl.SymName == tmpl.InClass.Name || tmpl.SymName == "~"+tmpl.InClass.Name) {
		ft.Ret = types.Typ(types.Void)
	}
	return ft, true
}

// instantiateWithArgs makes -- or finds -- the specialization of a
// function template for an argument list.
func (a *Analyzer) instantiateWithArgs(tmpl *FuncSymbol, args []types.TemplateArg, at ast.Tok) *FuncSymbol {
	info := tmpl.Template
	key := argsKey(args)
	if inst, done := info.Instances[key]; done {
		return inst
	}
	if a.instantiationDepth > 256 {
		a.errorAt(at, fmt.Sprintf("template instantiation depth exceeded while instantiating %s", tmpl.SymName))
		return nil
	}
	a.instantiationDepth++
	defer func() { a.instantiationDepth-- }()

	if info.Decl == nil || info.Decl.Body == nil {
		if a.unevaluated > 0 {
			// In unevaluated operands, declarations without definitions suffice.
			if ft, ok := a.instanceSignature(tmpl, args); ok {
				inst := &FuncSymbol{SymName: tmpl.SymName, FuncType: ft, SymPos: tmpl.SymPos, SymScope: tmpl.SymScope, InClass: tmpl.InClass, Access: tmpl.Access, Static: tmpl.Static, Inline: true, TemplateArgs: args, TemplateOf: tmpl}
				info.Instances[key] = inst
				return inst
			}
		}
		a.errorAt(at, fmt.Sprintf("%s<%s> is used but the template is declared and not defined", tmpl.SymName, key))
		return nil
	}

	paramScope := a.bindTemplateArgs(info.Scope, info.Params, args)
	instScope := NewScope(paramScope, BlockScope, nil)

	saved := a.enterInstantiation(instScope)
	if tmpl.InClass != nil {
		a.curRecord = tmpl.InClass
		a.curAccess = tmpl.Access
	}
	decl := ast.Clone(info.Decl)
	body := decl.Body
	decl.Body = nil
	a.checkFuncDecl(decl)

	var inst *FuncSymbol
	for _, s := range instScope.LookupLocal(tmpl.SymName) {
		if fn, isFn := s.(*FuncSymbol); isFn {
			inst = fn
			break
		}
	}
	if inst == nil {
		a.leaveInstantiation(saved)
		a.errorAt(at, fmt.Sprintf("instantiating %s produced no function", tmpl.SymName))
		return nil
	}
	inst.TemplateArgs = args
	inst.TemplateOf = tmpl
	// Specializations have vague linkage (inline).
	inst.Inline = true
	info.Instances[key] = inst

	if body != nil {
		decl.Body = body
		if a.unevaluated > 0 {
			if a.pendingInstances == nil {
				a.pendingInstances = map[*FuncSymbol]*pendingInstance{}
			}
			a.pendingInstances[inst] = &pendingInstance{decl: decl, scope: instScope, tmpl: tmpl}
		} else {
			a.checkFunctionBody(inst, decl)
		}
	}
	a.leaveInstantiation(saved)
	return inst
}

// instantiateClass makes -- or finds -- the specialization of a class
// template for a list of arguments.
func (a *Analyzer) instantiateClass(tmpl *RecordSymbol, args []types.TemplateArg, at ast.Tok) *RecordSymbol {
	if a.instantiationDepth > 256 {
		a.errorAt(at, fmt.Sprintf("template instantiation depth exceeded while instantiating %s", tmpl.SymName))
		return nil
	}
	a.instantiationDepth++
	defer func() { a.instantiationDepth-- }()

	info := tmpl.ClassTemplate
	args, ok := a.matchTemplateArgs(tmpl.SymName, info.Params, args, info.Scope, at)
	if !ok {
		return nil
	}
	key := argsKey(args)
	if inst, done := info.Instances[key]; done {
		return inst
	}

	// Select matching specialization: explicit specialization, then partial, then primary.
	pattern, params, scope := info.Spec, info.Params, info.Scope
	var bound []types.TemplateArg = args
	if explicit, has := info.Explicit[key]; has {
		pattern, params, bound = explicit, nil, nil
	} else if ps, b, matched := a.matchPartial(info, args); matched {
		pattern, params, scope, bound = ps.Spec, ps.Params, ps.Scope, b
	}

	if pattern == nil {
		a.errorAt(at, fmt.Sprintf("%s<%s> is incomplete: the template is declared and not defined, and no specialization matches", tmpl.SymName, key))
		return nil
	}

	paramScope := a.bindTemplateArgs(scope, params, bound)
	instScope := NewScope(paramScope, BlockScope, nil)

	saved := a.enterInstantiation(instScope)
	wasInstantiating, wasPrimary, wasArgs := a.instantiating, a.primary, a.instArgs
	a.instantiating, a.primary, a.instArgs = true, tmpl, args
	spec := ast.Clone(pattern)
	a.checkClassSpec(spec)
	a.instantiating, a.primary, a.instArgs = wasInstantiating, wasPrimary, wasArgs
	a.leaveInstantiation(saved)

	var inst *RecordSymbol
	for _, s := range instScope.LookupLocal(tmpl.Record.Name) {
		if rs, isRec := s.(*RecordSymbol); isRec {
			inst = rs
			break
		}
	}
	if inst == nil || inst.Record == nil {
		a.errorAt(at, fmt.Sprintf("instantiating %s produced no class", tmpl.SymName))
		return nil
	}
	inst.Record.TemplateArgs = args
	inst.Record.Scopes = tmpl.Record.Scopes
	primaryRecords[inst.Record] = tmpl.Record
	info.Instances[key] = inst
	return inst
}

// instantiateAlias instantiates an alias template specialization.
func (a *Analyzer) instantiateAlias(alias *TypeSymbol, args []types.TemplateArg, at ast.Tok) types.Type {
	info := alias.Alias
	if info.Builtin != "" {
		return a.instantiateBuiltinTemplate(info.Builtin, args, at)
	}
	args, ok := a.matchTemplateArgs(alias.SymName, info.Params, args, info.Scope, at)
	if !ok {
		return nil
	}
	paramScope := a.bindTemplateArgs(info.Scope, info.Params, args)
	id := ast.Clone(info.Type)
	specs := BuildDeclSpecs(id.Specs, paramScope, a.unit)
	if specs.Unresolved != "" {
		return nil
	}
	return BuildDeclarator(id.Decl, specs.Type, paramScope, a.unit)
}

// instantiateVar instantiates a variable template specialization.
func (a *Analyzer) instantiateVar(v *VarSymbol, args []types.TemplateArg, at ast.Tok) *VarSymbol {
	info := v.Template
	args, ok := a.matchTemplateArgs(v.SymName, info.Params, args, info.Scope, at)
	if !ok {
		return nil
	}
	key := argsKey(args)
	if inst, done := info.Instances[key]; done {
		return inst
	}

	// Match explicit, partial, or primary template.
	pattern, params, scope := info.Decl, info.Params, info.Scope
	bound := args
	if explicit, has := info.Explicit[key]; has {
		pattern, params, bound = explicit, nil, nil
	} else {
		for _, ps := range info.Partials {
			if b, ok := matchPattern(ps.Params, ps.Args, args); ok {
				pattern, params, scope, bound = ps.Decl, ps.Params, ps.Scope, b
				break
			}
		}
	}

	paramScope := a.bindTemplateArgs(scope, params, bound)
	instScope := NewScope(paramScope, BlockScope, nil)

	saved := a.enterInstantiation(instScope)
	wasInstantiating := a.instantiating
	a.instantiating = true
	a.checkSimpleDecl(ast.Clone(pattern))
	a.instantiating = wasInstantiating
	a.leaveInstantiation(saved)

	var inst *VarSymbol
	for _, sym := range instScope.LookupLocal(v.SymName) {
		if vs, isVar := sym.(*VarSymbol); isVar {
			inst = vs
			break
		}
	}
	if inst == nil {
		return nil
	}
	instScope.remove(inst)
	inst.TemplateArgs = args
	if info.Instances == nil {
		info.Instances = map[string]*VarSymbol{}
	}
	info.Instances[key] = inst
	return inst
}

// matchTemplateArgs checks and converts template arguments against parameters.
func (a *Analyzer) matchTemplateArgs(name string, params []*TemplateParamSymbol, args []types.TemplateArg, scope *Scope, at ast.Tok) ([]types.TemplateArg, bool) {
	out := make([]types.TemplateArg, len(params))
	for i, p := range params {
		var arg types.TemplateArg
		switch {
		case p.IsPack:
			// Pack parameter absorbs remaining arguments.
			var rest []types.TemplateArg
			if i < len(args) {
				rest = args[i:]
			}
			elems := append([]types.TemplateArg(nil), rest...)
			for k, r := range elems {
				if r.IsType && (r.Type == nil || isDependentType(r.Type)) {
					return nil, false
				}
				if !r.IsType && !p.IsType && r.ValType == nil {
					// A value in a non-type pack has the pack's element
					// type: `T... Vs` with T bound before it.
					elems[k].ValType = a.valueParamType(p, params[:i], out[:i], scope)
				}
			}
			out[i] = types.TemplateArg{IsType: true, Type: &types.Pack{Elems: elems}}
			continue
		case i < len(args):
			arg = args[i]
		case p.Default != nil:
			// Substitute default template argument.
			var ok bool
			arg, ok = a.defaultTemplateArg(p, params[:i], out[:i], scope)
			if !ok {
				return nil, false
			}
		default:
			a.errorAt(at, fmt.Sprintf("%s takes %d template arguments, %d given", name, len(params), len(args)))
			return nil, false
		}
		switch {
		case p.IsType && !arg.IsType:
			a.errorAt(at, fmt.Sprintf("template parameter %s of %s is a type; the argument is not", p.SymName, name))
			return nil, false
		case !p.IsType && arg.IsType:
			a.errorAt(at, fmt.Sprintf("template parameter %s of %s is a value; the argument is a type", p.SymName, name))
			return nil, false
		case p.IsType && (arg.Type == nil || isDependentType(arg.Type)):
			return nil, false
		case !p.IsType:
			arg.ValType = a.valueParamType(p, params[:i], out[:i], scope)
		}
		out[i] = arg
	}
	if len(args) > len(params) && (len(params) == 0 || !params[len(params)-1].IsPack) {
		a.errorAt(at, fmt.Sprintf("%s takes %d template arguments, %d given", name, len(params), len(args)))
		return nil, false
	}
	return out, true
}

// defaultTemplateArg evaluates a template parameter's default argument in scope.
func (a *Analyzer) defaultTemplateArg(p *TemplateParamSymbol, before []*TemplateParamSymbol, bound []types.TemplateArg, scope *Scope) (types.TemplateArg, bool) {
	if scope == nil {
		scope = a.curScope
	}
	paramScope := a.bindTemplateArgs(scope, before, bound)
	if p.IsType {
		id, isTypeId := p.Default.(*ast.TypeId)
		if !isTypeId {
			return types.TemplateArg{}, false
		}
		specs := BuildDeclSpecs(id.Specs, paramScope, a.unit)
		if specs.Type == nil || specs.Unresolved != "" {
			return types.TemplateArg{}, false
		}
		t := BuildDeclarator(id.Decl, specs.Type, paramScope, a.unit)
		if t == nil || isDependentType(t) {
			return types.TemplateArg{}, false
		}
		return types.TemplateArg{IsType: true, Type: t}, true
	}
	e, isExpr := p.Default.(ast.Expr)
	if !isExpr {
		return types.TemplateArg{}, false
	}
	n, ok := a.globalScope.EvalConst(e, paramScope)
	if !ok {
		return types.TemplateArg{}, false
	}
	return types.TemplateArg{Val: n, ValType: a.valueParamType(p, before, bound, scope)}, true
}

// valueParamType is a non-type template parameter's type with the
// parameters before it bound to their arguments.
//
// The type a parameter was declared with may be written in terms of those
// parameters, and then it is not a type until they are: the enable_if idiom
// declares `enable_if_t<is_int_v<T>, int> = 0`, which is int for T = int
// and nothing at all otherwise. A mangled name spells the argument's
// type, so the bound one is the one recorded. Where it still does not
// resolve, the declared type is kept, as before.
func (a *Analyzer) valueParamType(p *TemplateParamSymbol, before []*TemplateParamSymbol, bound []types.TemplateArg, scope *Scope) types.Type {
	if p.Decl == nil || p.SymType == nil || !isDependentType(p.SymType) {
		return p.SymType
	}
	if scope == nil {
		scope = a.curScope
	}
	paramScope := a.bindTemplateArgs(scope, before, bound)
	info := BuildDeclSpecs(p.Decl.Specs, paramScope, a.unit)
	if info.Type == nil || info.Unresolved != "" {
		return p.SymType
	}
	t := BuildDeclarator(p.Decl.Decl, info.Type, paramScope, a.unit)
	if t == nil || isDependentType(t) {
		return p.SymType
	}
	return t
}

// bindTemplateArgs binds template parameters to arguments in a new scope.
func (a *Analyzer) bindTemplateArgs(enclosing *Scope, params []*TemplateParamSymbol, args []types.TemplateArg) *Scope {
	paramScope := NewScope(enclosing, TemplateParamScope, nil)
	for i, p := range params {
		if i >= len(args) {
			break
		}
		if p.IsTemplate {
			// The argument of a template template parameter names a class
			// template: as a TemplateRef, or -- where it was written as a
			// plain name and read as a type -- as the template's own
			// record, which is the same template.
			var primary *RecordSymbol
			switch t := args[i].Type.(type) {
			case *types.TemplateRef:
				primary = paramScope.recordSymbol(t.Primary)
			case *types.Record:
				if rs := paramScope.recordSymbol(t); rs != nil && rs.ClassTemplate != nil {
					primary = rs
				}
			}
			if primary != nil {
				alias := *primary
				alias.SymName = p.SymName
				paramScope.Insert(&alias)
				continue
			}
		}
		if p.IsType || p.IsPack {
			paramScope.Insert(&TypeSymbol{SymName: p.SymName, SymType: args[i].Type, SymPos: p.SymPos, SymScope: paramScope})
			continue
		}
		paramScope.Insert(&VarSymbol{
			SymName: p.SymName, SymType: p.SymType, SymPos: p.SymPos, SymScope: paramScope,
			Constexpr: true, KnownValue: args[i].Val, HasKnownValue: true,
		})
	}
	return paramScope
}

// instantiationState saves analysis state during template instantiation.
type instantiationState struct {
	scope    *Scope
	fn       *FuncSymbol
	record   *types.Record
	params   []*TemplateParamSymbol
	access   types.Access
	externC  bool
	loops    int
	owner    ast.Decl
	requires ast.Expr
	nested   []func()
}

func (a *Analyzer) enterInstantiation(scope *Scope) instantiationState {
	saved := instantiationState{a.curScope, a.curFunc, a.curRecord, a.curTemplateParams, a.curAccess, a.externC, a.loopDepth, a.templateOwner, a.curTemplateRequires, a.nestedBodies}
	a.curScope, a.curFunc, a.curRecord, a.curTemplateParams, a.externC, a.loopDepth = scope, nil, nil, nil, false, 0
	a.templateOwner, a.curTemplateRequires = nil, nil
	// The nested-class bodies waiting on whatever class was being
	// defined are that class's to run, not this instantiation's.
	a.nestedBodies = nil
	a.curAccess = types.AccessPublic
	return saved
}

func (a *Analyzer) leaveInstantiation(s instantiationState) {
	a.curScope, a.curFunc, a.curRecord, a.curTemplateParams, a.curAccess, a.externC, a.loopDepth = s.scope, s.fn, s.record, s.params, s.access, s.externC, s.loops
	a.templateOwner, a.curTemplateRequires = s.owner, s.requires
	a.nestedBodies = append(s.nested, a.nestedBodies...)
}

// argsKey tells one argument list from another.
func argsKey(args []types.TemplateArg) string {
	parts := make([]string, len(args))
	for i, t := range args {
		parts[i] = t.String()
	}
	return strings.Join(parts, ",")
}

// typeArgsOf is a list of type arguments as template arguments.
func typeArgsOf(ts []types.Type) []types.TemplateArg {
	out := make([]types.TemplateArg, len(ts))
	for i, t := range ts {
		out[i] = types.TemplateArg{IsType: true, Type: t}
	}
	return out
}

// A pendingBody is a member of a class template's specialization whose
// body waits for a use.
type pendingBody struct {
	decl  *ast.FuncDecl
	rec   *types.Record
	scope *Scope // the class scope, where the body is checked
}

// usedByExistence reports whether a member is instantiated with its class
// rather than on use: a constructor, a destructor, or a virtual function.
func (a *Analyzer) usedByExistence(d *ast.FuncDecl, rec *types.Record) bool {
	name := ""
	if d.Decl != nil && d.Decl.DeclName() != nil {
		name = NameString(d.Decl.DeclName(), a.unit)
	}
	if name == "" || name == rec.Name || name == "~"+rec.Name {
		return true
	}
	for _, m := range rec.Methods {
		if m.Name == name && m.Virtual {
			return true
		}
	}
	return false
}

// deferBody records a member body for its first use.
func (a *Analyzer) deferBody(d *ast.FuncDecl, rec *types.Record) {
	sym := a.methodSymbolFor(d)
	if sym == nil {
		return
	}
	if a.pending == nil {
		a.pending = map[*types.Method]*pendingBody{}
	}
	for _, m := range rec.Methods {
		if m.Name == sym.SymName && m.Func == sym.FuncType {
			a.pending[m] = &pendingBody{decl: d, rec: rec, scope: a.curScope}
			return
		}
	}
}

// ensureInstantiated checks the body of a member a use just resolved to,
// if it is one whose body was left for this moment.
func (a *Analyzer) ensureInstantiated(fn *FuncSymbol) {
	if fn == nil {
		return
	}
	if pi, waiting := a.pendingInstances[fn]; waiting && a.unevaluated == 0 {
		// The body of a specialization declared in an unevaluated
		// operand, checked now that something calls it: the definition
		// merges into the declaration (see Scope.InsertFunc).
		delete(a.pendingInstances, fn)
		saved := a.enterInstantiation(pi.scope)
		if pi.tmpl.InClass != nil {
			a.curRecord = pi.tmpl.InClass
			a.curAccess = pi.tmpl.Access
		}
		a.checkFuncDecl(pi.decl)
		a.leaveInstantiation(saved)
	}
	if fn.Method == nil || a.pending == nil {
		return
	}
	pb, waiting := a.pending[fn.Method]
	if !waiting {
		return
	}
	delete(a.pending, fn.Method)
	saved := a.enterInstantiation(pb.scope)
	a.curRecord = pb.rec
	a.checkMethodBody(pb.decl)
	a.leaveInstantiation(saved)
}

// A pendingInstance is a function template's specialization declared in an unevaluated operand.
type pendingInstance struct {
	decl  *ast.FuncDecl
	scope *Scope
	tmpl  *FuncSymbol
}

// matchPartial finds a matching partial specialization.
func (a *Analyzer) matchPartial(info *ClassTemplateInfo, args []types.TemplateArg) (*PartialSpec, []types.TemplateArg, bool) {
	for _, ps := range info.Partials {
		bound, ok := matchPattern(ps.Params, ps.Args, args)
		if os.Getenv("VCX_DEBUG_MATCH") != "" {
			fmt.Fprintf(os.Stderr, "matchPartial pattern=%s args=%s -> %v\n", argsKey(ps.Args), argsKey(args), ok)
			for _, x := range ps.Args {
				fmt.Fprintf(os.Stderr, "   pat %T %v\n", x.Type, x.Type)
			}
		}
		if ok && a.spellingMatches(ps, bound, args) {
			return ps, bound, true
		}
	}
	return nil, nil, false
}

// spellingMatches validates non-deduced alias template specialization arguments
// after substituting deduced template bindings into the pattern.
func (a *Analyzer) spellingMatches(ps *PartialSpec, bound, args []types.TemplateArg) bool {
	tn, isTemplate := ps.Spec.Name.(*ast.TemplateName)
	if !isTemplate {
		return true
	}
	var rebuilt []types.TemplateArg
	for i, pat := range ps.Args {
		if !pat.IsType || !isAliasSpelling(pat.Type) || i >= len(args) || !args[i].IsType {
			continue
		}
		if rebuilt == nil {
			paramScope := a.bindTemplateArgs(ps.Scope, ps.Params, bound)
			rebuilt = templateArgs(tn, paramScope, a.unit)
			if len(rebuilt) != len(ps.Args) {
				return false
			}
		}
		if !rebuilt[i].IsType || rebuilt[i].Type == nil || isDependent(rebuilt[i].Type) || !rebuilt[i].Type.Equal(args[i].Type) {
			return false
		}
	}
	return true
}

// isAliasSpelling reports whether a pattern type is an alias template's
// specialization left as spelled because its arguments were dependent.
func isAliasSpelling(t types.Type) bool {
	ts, isSpec := types.Unqualify(t).(*types.TemplateSpecialization)
	if !isSpec {
		return false
	}
	switch ts.Type.(type) {
	case *types.Record, *types.TemplateParam:
		return false
	}
	return true
}

// matchPattern deduces partial specialization parameters from pattern arguments.
func matchPattern(params []*TemplateParamSymbol, pattern, args []types.TemplateArg) ([]types.TemplateArg, bool) {
	b := Binding{}
	args = flattenPacks(args)
	// A pack at the end of the pattern takes remaining arguments.
	if n := len(pattern); n > 0 && isPackParam(pattern[n-1]) {
		if len(args) < n-1 {
			return nil, false
		}
		tp := pattern[n-1].Type.(*types.TemplateParam)
		b[tp.Name] = &types.Pack{Elems: append([]types.TemplateArg(nil), args[n-1:]...)}
		pattern, args = pattern[:n-1], args[:n-1]
	}
	if len(pattern) != len(args) {
		return nil, false
	}
	for i, pat := range pattern {
		arg := args[i]
		switch {
		case pat.IsType && arg.IsType:
			if !matchType(pat.Type, arg.Type, b) {
				return nil, false
			}
		case pat.IsType && !arg.IsType:
			tp, isParam := pat.Type.(*types.TemplateParam)
			if !isParam || tp.IsType {
				return nil, false
			}
			if prev, seen := b[tp.Name]; seen {
				if vb, isVal := prev.(*valueBound); !isVal || vb.Val != arg.Val {
					return nil, false
				}
			} else {
				b[tp.Name] = &valueBound{Type: arg.ValType, Val: arg.Val}
			}
		case !pat.IsType && !arg.IsType:
			if pat.Val != arg.Val {
				return nil, false
			}
		default:
			return nil, false
		}
	}
	return boundArgs(params, b)
}

// noteSpecialization records a full or partial specialization of a class template.
func (a *Analyzer) noteSpecialization(s *ast.ClassSpec, params []*TemplateParamSymbol) {
	tn, isTemplate := s.Name.(*ast.TemplateName)
	if !isTemplate {
		return
	}
	var primary *RecordSymbol
	for _, sym := range LookupUnqualified(a.curScope, NameString(tn.Name, a.unit)) {
		if rs, isRec := sym.(*RecordSymbol); isRec && rs.ClassTemplate != nil {
			primary = rs
			break
		}
	}
	if primary == nil {
		a.errorAt(s.Pos(), fmt.Sprintf("%s is not a class template", NameString(tn.Name, a.unit)))
		return
	}
	args := templateArgs(tn, a.curScope, a.unit)
	if len(params) == 0 {
		if primary.ClassTemplate.Explicit == nil {
			primary.ClassTemplate.Explicit = map[string]*ast.ClassSpec{}
		}
		primary.ClassTemplate.Explicit[argsKey(args)] = s
		return
	}
	primary.ClassTemplate.Partials = append(primary.ClassTemplate.Partials, &PartialSpec{
		Params: params, Args: args, Spec: s, Scope: a.declScope(),
	})
}

// boundArgs converts deduction bindings into an argument list in parameter order.
func boundArgs(params []*TemplateParamSymbol, b Binding) ([]types.TemplateArg, bool) {
	bound := make([]types.TemplateArg, len(params))
	for i, p := range params {
		t, has := b[p.SymName]
		if !has {
			return nil, false
		}
		vb, isVal := t.(*valueBound)
		_, isPack := t.(*types.Pack)
		_, isRef := t.(*types.TemplateRef)
		switch {
		case p.IsTemplate != isRef:
			return nil, false
		case p.IsPack != isPack:
			return nil, false
		case p.IsType && !isVal:
			bound[i] = types.TemplateArg{IsType: true, Type: t}
		case !p.IsType && isVal:
			bound[i] = types.TemplateArg{Val: vb.Val, ValType: p.SymType}
		default:
			return nil, false
		}
	}
	return bound, true
}

// noteVarSpecialization records a full or partial specialization of a variable template.
func (a *Analyzer) noteVarSpecialization(d *ast.SimpleDecl, tn *ast.TemplateName, params []*TemplateParamSymbol) {
	name := NameString(tn.Name, a.unit)
	var primary *VarSymbol
	for _, sym := range LookupUnqualified(a.curScope, name) {
		if vs, isVar := sym.(*VarSymbol); isVar && vs.Template != nil {
			primary = vs
			break
		}
	}
	if primary == nil {
		a.errorAt(d.Pos(), fmt.Sprintf("%s is not a variable template", name))
		return
	}
	args := templateArgs(tn, a.curScope, a.unit)
	info := primary.Template
	if len(params) == 0 {
		if info.Explicit == nil {
			info.Explicit = map[string]*ast.SimpleDecl{}
		}
		matched, ok := a.matchTemplateArgs(name, info.Params, args, info.Scope, tn.Pos())
		if !ok {
			return
		}
		info.Explicit[argsKey(matched)] = d
		return
	}
	info.Partials = append(info.Partials, &VarPartial{Params: params, Args: args, Decl: d, Scope: a.declScope()})
}

// isPackParam reports whether a pattern argument is a parameter pack
// still open: the `_Rest...` of a partial specialization's pattern.
func isPackParam(arg types.TemplateArg) bool {
	if !arg.IsType {
		return false
	}
	tp, isParam := arg.Type.(*types.TemplateParam)
	return isParam && tp.IsPack
}

// flattenPacks spreads every Pack in an argument list into its elements.
func flattenPacks(args []types.TemplateArg) []types.TemplateArg {
	flat := make([]types.TemplateArg, 0, len(args))
	for _, arg := range args {
		if pack, isPack := arg.Type.(*types.Pack); arg.IsType && isPack {
			flat = append(flat, pack.Elems...)
			continue
		}
		flat = append(flat, arg)
	}
	return flat
}
