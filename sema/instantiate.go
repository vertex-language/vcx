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

	// HeadKey spells the template-head as written, each parameter by its
	// position: two declarations with the same key declare one template.
	HeadKey string

	// Specs and Declarator are a declaration's, for a template declared and
	// not defined -- libc++'s `static decltype(std::__invoke(...))
	// __try_call(int);` -- whose signature is rebuilt under the arguments.
	Specs      *ast.DeclSpecs
	Declarator ast.Declarator

	// DefParams are the parameters as an out-of-line definition names them,
	// for rebuilding the definition's signature and body; Params stay the
	// declaration's, which deduction, defaults, and constraints use.
	DefParams []*TemplateParamSymbol

	// Pattern is the function type as written, its parameters still open:
	// what partial ordering compares ([temp.func.order]). Candidates deduced
	// for a call carry specialized types and share this info.
	Pattern *types.Func
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

	// ExplicitSpecs are the explicit specializations as written, by their
	// arguments, for the out-of-line definitions of their members:
	// `int char_traits<char16_t>::compare(...)`.
	ExplicitSpecs map[string]*PartialSpec

	// Incomplete are the specializations named while the template had no
	// definition, by their arguments. Each is the record its uses hold, and
	// the instantiation made once there is a definition fills it in.
	Incomplete map[string]*RecordSymbol

	// Guides are the template's deduction guides ([temp.deduct.guide]).
	Guides []*DeductionGuide
}

// A DeductionGuide is `template <class _T1, class _T2> pair(_T1, _T2) ->
// pair<_T1, _T2>;`: its template parameters and its signature, whose return
// type is the specialization it deduces.
type DeductionGuide struct {
	Params []*TemplateParamSymbol
	Func   *types.Func
}

// A PartialSpec is `template <class T> struct X<T *, 4>`: its own
// parameters, the argument pattern, and the body.
type PartialSpec struct {
	Params []*TemplateParamSymbol
	Args   []types.TemplateArg
	Spec   *ast.ClassSpec
	Scope  *Scope

	// Sym is the specialization as written, whose class scope holds the
	// members an out-of-line definition -- `void V<bool, A>::clear()` --
	// completes.
	Sym *RecordSymbol

	// Constraints are the template-head's requires-clause: libc++'s
	// `__move_iter_category_base<_Iter>` differs from its primary only by
	// `requires requires { typename iterator_traits<_Iter>::iterator_category; }`.
	Constraints []ast.Expr
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
	if inst, done := info.Instances[instanceKey(args)]; done {
		return inst.FuncType, true
	}
	decl := info.Decl
	if decl == nil && info.Specs != nil && info.Declarator != nil {
		specs, declarator := ast.Clone(info.Specs), ast.Clone(info.Declarator)
		paramScope := a.bindTemplateArgs(info.Scope, info.Params, args)
		declInfo := BuildDeclSpecs(specs, paramScope, a.unit)
		if declInfo.Unresolved != "" {
			return nil, false
		}
		ft, isFunc := BuildDeclarator(declarator, declInfo.Type, paramScope, a.unit).(*types.Func)
		if !isFunc || isDependent(ft) {
			return nil, false
		}
		if tmpl.InClass != nil && (tmpl.SymName == tmpl.InClass.Name || tmpl.SymName == "~"+tmpl.InClass.Name) {
			ft.Ret = types.Typ(types.Void)
		}
		return ft, true
	}
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
	// A copy: checking the signature expands the packs in it in place --
	// `decltype(std::declval<_Fp>()(std::declval<_Args>()...))` -- and the
	// template as written must keep its expansions for the next arguments.
	decl = ast.Clone(decl)
	paramScope := a.bindTemplateArgs(info.Scope, info.declParams(), args)
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
	ikey := instanceKey(args)
	if inst, done := info.Instances[ikey]; done {
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
				info.Instances[ikey] = inst
				return inst
			}
		}
		// A member template of a class template whose definition is written
		// after the class -- basic_string::__init, called from constructors
		// in the class body -- is defined by the time the translation unit
		// ends, which is where C++ instantiates it. The specialization is
		// declared from its signature now and defined when the definition
		// arrives (see defineAwaiting); one that never does is reported
		// when the analysis ends.
		if tmpl.InClass != nil {
			if ft, ok := a.instanceSignature(tmpl, args); ok {
				inst := &FuncSymbol{SymName: tmpl.SymName, FuncType: ft, SymPos: tmpl.SymPos, SymScope: tmpl.SymScope, InClass: tmpl.InClass, Access: tmpl.Access, Static: tmpl.Static, Inline: true, TemplateArgs: args, TemplateOf: tmpl}
				info.Instances[ikey] = inst
				a.awaiting = append(a.awaiting, &awaitingInstance{tmpl: tmpl, args: args, inst: inst, at: at})
				return inst
			}
		}
		a.errorAt(at, fmt.Sprintf("%s<%s> is used but the template is declared and not defined", tmpl.SymName, key))
		return nil
	}

	paramScope := a.bindTemplateArgs(info.Scope, info.declParams(), args)
	instScope := NewScope(paramScope, BlockScope, nil)
	instScope.Instantiation = true

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
	info.Instances[ikey] = inst

	if body != nil {
		decl.Body = body
		if a.unevaluated > 0 && inst.FuncType != nil && mentionsAuto(inst.FuncType.Ret) {
			// [dcl.spec.auto.general]/12: a placeholder return type is
			// deduced by instantiating the definition, even when the call
			// is an unevaluated operand -- decltype(ranges::iter_move(i))
			// has no type until the body says what it returns.
			wasUnevaluated := a.unevaluated
			a.unevaluated = 0
			a.checkFunctionBody(inst, decl)
			a.unevaluated = wasUnevaluated
		} else if a.unevaluated > 0 {
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
	ikey := instanceKey(args)
	if inst, done := info.Instances[ikey]; done {
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
		// [temp.inst]/2: naming a specialization does not instantiate it.
		// `__is_identity<reference_wrapper<__identity>>` names one of a
		// template declared and not yet defined, and that is fine until
		// something needs the class complete -- which says so itself. It is
		// not kept in Instances: a definition later in the unit makes the
		// real one.
		if held := info.Incomplete[ikey]; held != nil {
			return held
		}
		rec := &types.Record{Tag: tmpl.Record.Tag, Name: tmpl.Record.Name, Scopes: tmpl.Record.Scopes, TemplateArgs: args}
		primaryRecords[rec] = tmpl.Record
		held := &RecordSymbol{SymName: tmpl.SymName, Record: rec, SymPos: tmpl.SymPos, SymScope: tmpl.SymScope, TemplateOf: tmpl}
		if info.Incomplete == nil {
			info.Incomplete = map[string]*RecordSymbol{}
		}
		info.Incomplete[ikey] = held
		if a.placeholders == nil {
			a.placeholders = map[*types.Record]*RecordSymbol{}
		}
		a.placeholders[rec] = tmpl
		return held
	}

	paramScope := a.bindTemplateArgs(scope, params, bound)
	instScope := NewScope(paramScope, BlockScope, nil)

	saved := a.enterInstantiation(instScope)
	wasInstantiating, wasPrimary, wasArgs := a.instantiating, a.primary, a.instArgs
	a.instantiating, a.primary, a.instArgs = true, tmpl, args
	spec := ast.Clone(pattern)
	if held := info.Incomplete[ikey]; held != nil {
		a.reuseRecord = held.Record
		delete(info.Incomplete, ikey)
		delete(a.placeholders, held.Record)
	}
	a.checkClassSpec(spec)
	a.reuseRecord = nil
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
	info.Instances[ikey] = inst

	// The members its pattern defines out of line, now that there is a
	// class with its arguments bound to give them to.
	a.instances = append(a.instances, instanceMade{inst: inst, pattern: pattern, scope: paramScope})
	for _, def := range a.outOfLine[pattern] {
		a.completeInstanceMember(inst, def, paramScope)
	}
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
		// `__enable_if_t<false, int>` is `typename enable_if<false, int>::type`,
		// which is no type: a substitution failure for whoever named it, and
		// said so rather than leaving the template-id standing for a type.
		return substitutionFailure
	}
	return BuildDeclarator(id.Decl, specs.Type, paramScope, a.unit)
}

// declParams are the parameters the template's definition is rebuilt with:
// the definition's own names when it was written out of line.
func (info *TemplateInfo) declParams() []*TemplateParamSymbol {
	if len(info.DefParams) == len(info.Params) && info.DefParams != nil {
		return info.DefParams
	}
	return info.Params
}

// substitutionFailure is what an alias template's specialization is when its
// target names nothing under the arguments.
var substitutionFailure = &types.DependentType{Name: "<substitution failure>"}

// instantiateVar instantiates a variable template specialization.
func (a *Analyzer) instantiateVar(v *VarSymbol, args []types.TemplateArg, at ast.Tok) *VarSymbol {
	info := v.Template
	args, ok := a.matchTemplateArgs(v.SymName, info.Params, args, info.Scope, at)
	if !ok {
		return nil
	}
	key := argsKey(args)
	ikey := instanceKey(args)
	if inst, done := info.Instances[ikey]; done {
		return inst
	}

	// Match explicit, partial, or primary template.
	pattern, params, scope := info.Decl, info.Params, info.Scope
	bound := args
	if explicit, has := info.Explicit[key]; has {
		pattern, params, bound = explicit, nil, nil
	} else {
		for _, ps := range info.Partials {
			b, ok := matchPattern(ps.Params, ps.Args, args)
			if !ok {
				continue
			}
			if tn := varPartialName(ps); tn != nil && !a.substitutedArgsMatch(tn, ps.Params, ps.Args, ps.Scope, b, args) {
				continue
			}
			pattern, params, scope, bound = ps.Decl, ps.Params, ps.Scope, b
			break
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
	info.Instances[ikey] = inst
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
		case !p.IsType && arg.IsType && (arg.Type == nil || isDependentType(arg.Type)):
			// `_If<__has_random_access_iterator_category<_Iter>::value, ...>`
			// in a template as written: the value depends on a parameter, and
			// the name was read as the dependent type it could also be. The
			// specialization is made when the instantiation says which.
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
	if !a.nonTypeParamSubstitutes(p, before, bound, scope) {
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
				// An alias template -- `__pointer_member` passed to
				// __detector's `template <class...> class _Op` -- is bound as
				// the alias, so `_Op<_Args...>` instantiates it.
				if ts, isAlias := t.Alias.(*TypeSymbol); isAlias && ts != nil {
					bound := *ts
					bound.SymName = p.SymName
					paramScope.Insert(&bound)
					continue
				}
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
	// lambdas are the lambdas whose bodies were being read: a template
	// instantiated from inside one -- std::move, called in a lambda -- is not
	// inside it.
	lambdas []*LambdaInfo
}

func (a *Analyzer) enterInstantiation(scope *Scope) instantiationState {
	saved := instantiationState{a.curScope, a.curFunc, a.curRecord, a.curTemplateParams, a.curAccess, a.externC, a.loopDepth, a.templateOwner, a.curTemplateRequires, a.nestedBodies, a.lambdas}
	a.lambdas = nil
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
	a.lambdas = s.lambdas
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

// wantKey names a member for wantedBodies by identity: two members of one
// signature, told apart by their requires-clauses, are two members.
func wantKey(m *types.Method) string {
	return fmt.Sprintf("%p", m)
}

// bodyWanted reports whether a use already asked for a member's body.
func (a *Analyzer) bodyWanted(d *ast.FuncDecl, rec *types.Record) bool {
	if len(a.wantedBodies) == 0 {
		return false
	}
	sym := a.methodSymbolFor(d)
	return sym != nil && sym.Method != nil && a.wantedBodies[wantKey(sym.Method)]
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
	// The member by identity where the symbol knows it: two members of one
	// signature -- `step() const` and `step() const requires Big<T>` -- are
	// two bodies. Its signature only when nothing is the member itself.
	match := func(m *types.Method) bool { return sym.Method != nil && m == sym.Method }
	found := false
	for _, m := range rec.Methods {
		if match(m) {
			found = true
			break
		}
	}
	if !found {
		match = func(m *types.Method) bool {
			return m.Name == sym.SymName && (m.Func == sym.FuncType || m.Func != nil && sym.FuncType != nil && m.Func.Equal(sym.FuncType))
		}
	}
	for _, m := range rec.Methods {
		if match(m) {
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
	if fn.Method != nil && fn.InClass != nil && fn.Body == nil {
		if _, waiting := a.pending[fn.Method]; !waiting {
			if a.wantedBodies == nil {
				a.wantedBodies = map[string]bool{}
			}
			a.wantedBodies[wantKey(fn.Method)] = true
		}
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

// deduceMemberReturn checks, ahead of the rest of its class's bodies, the body
// of an in-class member whose placeholder return type a use needs now.
func (a *Analyzer) deduceMemberReturn(fn *FuncSymbol) {
	pb := a.earlyBodies[fn]
	if pb == nil || a.dependentContext() {
		return
	}
	delete(a.earlyBodies, fn)
	if a.bodyChecked == nil {
		a.bodyChecked = map[*ast.FuncDecl]bool{}
	}
	a.bodyChecked[pb.decl] = true
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
		if ok && a.spellingMatches(ps, bound, args) && a.partialConstraintsHold(ps, bound) {
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
	return a.substitutedArgsMatch(tn, ps.Params, ps.Args, ps.Scope, bound, args)
}

// substitutedArgsMatch checks the pattern arguments that deduction could not:
// an alias template's spelling, or a type a dependent expression decides --
// `decltype((void)std::declval<_Alloc&>().max_size())`. Each is read again
// with the deduced parameters bound, and has to come out as the argument.
//
// [temp.deduct]/8: a substitution that makes an invalid type or expression
// is a failure to match, not an error in the program. That is how libc++
// detects a member: the specialization for an allocator without max_size
// does not match, and the primary's `false` stands.
func (a *Analyzer) substitutedArgsMatch(tn *ast.TemplateName, params []*TemplateParamSymbol, pattern []types.TemplateArg, scope *Scope, bound, args []types.TemplateArg) bool {
	var rebuilt []types.TemplateArg
	for i, pat := range pattern {
		if !pat.IsType || i >= len(args) || !args[i].IsType {
			continue
		}
		// `decltype((void)expr)` is void whatever expr is, so its type says
		// nothing about whether expr is valid for these arguments; the
		// spelling is what has to be substituted into.
		if !needsSubstitution(pat.Type) && !(i < len(tn.Args) && writtenWithDecltype(tn.Args[i])) {
			continue
		}
		if rebuilt == nil {
			ndiags := len(a.diags)
			paramScope := a.bindTemplateArgs(scope, params, bound)
			rebuilt = templateArgs(tn, paramScope, a.unit)
			failed := len(a.diags) > ndiags
			a.diags = a.diags[:ndiags]
			if failed || len(rebuilt) != len(pattern) {
				return false
			}
		}
		if !rebuilt[i].IsType || rebuilt[i].Type == nil || isDependent(rebuilt[i].Type) || !rebuilt[i].Type.Equal(args[i].Type) {
			return false
		}
	}
	return true
}

// needsSubstitution reports whether a pattern argument is matched by
// substituting into it rather than by deduction: an alias template's
// spelling, or a type still waiting on a dependent expression or name.
func needsSubstitution(t types.Type) bool {
	if isAliasSpelling(t) {
		return true
	}
	_, isDep := types.Unqualify(t).(*types.DependentType)
	return isDep
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

// partialConstraintsHold reports whether a partial specialization's
// constraints are satisfied by the arguments its pattern deduced
// ([temp.spec.partial.match]/2).
func (a *Analyzer) partialConstraintsHold(ps *PartialSpec, bound []types.TemplateArg) bool {
	if len(ps.Constraints) == 0 {
		return true
	}
	if argsDependent(bound) {
		return false
	}
	scope := a.bindTemplateArgs(ps.Scope, ps.Params, bound)
	for _, e := range ps.Constraints {
		ndiags := len(a.diags)
		v, err := a.evalInScope(a.NewConstContext(), e, scope)
		if len(a.diags) > ndiags {
			a.diags = a.diags[:ndiags]
			return false
		}
		if err != nil || !v.ToBool() {
			return false
		}
	}
	return true
}

// noteSpecialization records a full or partial specialization of a class template.
func (a *Analyzer) noteSpecialization(s *ast.ClassSpec, params []*TemplateParamSymbol) *PartialSpec {
	tn, isTemplate := s.Name.(*ast.TemplateName)
	if !isTemplate {
		return nil
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
		return nil
	}
	args := templateArgs(tn, a.curScope, a.unit)
	if len(params) == 0 {
		// Keyed as instantiateClass keys what it looks up: by the arguments
		// matched against the template's parameters, where `tuple<>` is an
		// empty pack rather than no arguments at all.
		key := a.specializationKey(primary, args, tn.Pos())
		if primary.ClassTemplate.Explicit == nil {
			primary.ClassTemplate.Explicit = map[string]*ast.ClassSpec{}
		}
		primary.ClassTemplate.Explicit[key] = s
		if primary.ClassTemplate.ExplicitSpecs == nil {
			primary.ClassTemplate.ExplicitSpecs = map[string]*PartialSpec{}
		}
		es := &PartialSpec{Args: args, Spec: s, Scope: a.declScope()}
		primary.ClassTemplate.ExplicitSpecs[key] = es
		return es
	}
	// `template <class _Iter> requires ... struct __move_iter_category_base<_Iter>`
	// of `template <class _Iter, class = void>`: the arguments the pattern
	// leaves off are the primary's defaults, as they are for every use.
	if info := primary.ClassTemplate; len(args) < len(info.Params) && (len(info.Params) == 0 || !info.Params[len(info.Params)-1].IsPack) {
		for i := len(args); i < len(info.Params); i++ {
			if info.Params[i].Default == nil {
				break
			}
			arg, ok := a.defaultTemplateArg(info.Params[i], info.Params[:i], args[:i], info.Scope)
			if !ok {
				break
			}
			args = append(args, arg)
		}
	}
	ps := &PartialSpec{Params: params, Args: args, Spec: s, Scope: a.declScope()}
	for _, p := range params {
		if p.TypeConstraint != nil {
			ps.Constraints = append(ps.Constraints, p.TypeConstraint)
		}
	}
	if a.curTemplateRequires != nil {
		ps.Constraints = append(ps.Constraints, a.curTemplateRequires)
	}
	primary.ClassTemplate.Partials = append(primary.ClassTemplate.Partials, ps)
	return ps
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
	a.noteVarSpecializationAmong(d, tn, params, LookupUnqualified(a.curScope, NameString(tn.Name, a.unit)))
}

// noteVarSpecializationAmong is noteVarSpecialization for a template already
// looked up -- `ranges::enable_view<basic_string_view<C, T>> = true`, whose
// template is found through its qualifier.
func (a *Analyzer) noteVarSpecializationAmong(d *ast.SimpleDecl, tn *ast.TemplateName, params []*TemplateParamSymbol, syms []Symbol) {
	name := NameString(tn.Name, a.unit)
	var primary *VarSymbol
	for _, sym := range syms {
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

// completeNamedSpecialization instantiates a specialization that was named
// before its template was defined, once something needs the class complete.
//
// `using string = basic_string<char>;` in <__fwd/string.h> holds the record
// such a name made; nothing names basic_string<char> again to instantiate
// it, so a member access, a constructor call or a member lookup asks here.
// The instantiation fills that same record in.
func (a *Analyzer) completeNamedSpecialization(rec *types.Record, at ast.Tok) {
	if rec == nil || rec.Complete {
		return
	}
	tmpl := a.placeholders[rec]
	if tmpl == nil || tmpl.ClassTemplate == nil || tmpl.ClassTemplate.Spec == nil {
		return
	}
	a.instantiateClass(tmpl, rec.TemplateArgs, at)
}

// varPartialName is the template-id a variable template's partial
// specialization is declared by: `__has_max_size_v<_Alloc, decltype(...)>`,
// written plainly or behind a qualifier.
func varPartialName(ps *VarPartial) *ast.TemplateName {
	if ps.Decl == nil || len(ps.Decl.Inits) == 0 || ps.Decl.Inits[0].Decl == nil {
		return nil
	}
	switch n := ps.Decl.Inits[0].Decl.DeclName().(type) {
	case *ast.TemplateName:
		return n
	case *ast.QualifiedName:
		if tn, isTemplate := n.Name.(*ast.TemplateName); isTemplate {
			return tn
		}
	}
	return nil
}

// writtenWithDecltype reports whether a template argument is spelled with a
// decltype-specifier anywhere in it.
func writtenWithDecltype(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(m ast.Node) bool {
		if _, isDecltype := m.(*ast.DecltypeSpec); isDecltype {
			found = true
		}
		return !found
	})
	return found
}

// nonTypeParamSubstitutes reports whether a non-type template parameter's
// declared type is a type once the parameters before it are bound.
//
// The enable_if idiom declares `__enable_if_t<__has_forward_iterator_category<
// _ForwardIterator>::value, int> = 0`: int for a forward iterator, and for
// anything else no type at all -- enable_if<false, int> has no member type.
// [temp.deduct]/8 makes that a failure to deduce, which removes the
// candidate; it is not an error, and what it said is not reported.
func (a *Analyzer) nonTypeParamSubstitutes(p *TemplateParamSymbol, before []*TemplateParamSymbol, bound []types.TemplateArg, scope *Scope) bool {
	if p.Decl == nil || p.SymType == nil || !isDependentType(p.SymType) || argsDependent(bound) {
		return true
	}
	if scope == nil {
		scope = a.curScope
	}
	ndiags := len(a.diags)
	paramScope := a.bindTemplateArgs(scope, before, bound)
	info := BuildDeclSpecs(p.Decl.Specs, paramScope, a.unit)
	a.diags = a.diags[:ndiags]
	if info.Type != nil && info.Unresolved == "" && !isDependentType(info.Type) {
		return true
	}
	// Only a definite failure removes the candidate. A condition this
	// analysis could not evaluate -- a value argument left as "<value>" --
	// says nothing about the program, and the candidate stays.
	return info.Type != nil && mentionsUnevaluatedValue(info.Type)
}

// mentionsUnevaluatedValue reports whether a type carries a template value
// argument that could not be evaluated where the type was built.
func mentionsUnevaluatedValue(t types.Type) bool {
	switch x := types.Unqualify(t).(type) {
	case *types.DependentType:
		return x.Name == "<value>"
	case *types.TemplateSpecialization:
		for _, arg := range x.Args {
			if arg.IsType && arg.Type != nil && mentionsUnevaluatedValue(arg.Type) {
				return true
			}
		}
		return x.Type != nil && x.Type != t && mentionsUnevaluatedValue(x.Type)
	}
	return false
}

// specializationKey is the key a class template's specialization for args is
// found by: the arguments matched against the template's parameters, with
// defaults filled in and packs gathered, or the arguments as written where
// they do not match.
func (a *Analyzer) specializationKey(tmpl *RecordSymbol, args []types.TemplateArg, at ast.Tok) string {
	info := tmpl.ClassTemplate
	if info == nil || info.Params == nil {
		return argsKey(args)
	}
	ndiags := len(a.diags)
	matched, ok := a.matchTemplateArgs(tmpl.SymName, info.Params, args, info.Scope, at)
	a.diags = a.diags[:ndiags]
	if !ok {
		return argsKey(args)
	}
	return argsKey(matched)
}

// instanceKey is what a specialization is cached by: its arguments as
// argsKey spells them, and the identity of every class among them. Two
// classes nested in different specializations share a spelling --
// basic_string<char>::__rep and basic_string<wchar_t>::__rep are both
// `union __rep` -- and a std::move made for one is not the other's.
func instanceKey(args []types.TemplateArg) string {
	key := argsKey(args)
	var ids []string
	for _, arg := range args {
		if arg.IsType {
			recordIdentities(arg.Type, &ids)
		}
	}
	if len(ids) == 0 {
		return key
	}
	return key + "#" + strings.Join(ids, ",")
}

// recordIdentities appends the identity of each class a type is made of.
func recordIdentities(t types.Type, out *[]string) {
	switch x := t.(type) {
	case *types.Record:
		*out = append(*out, fmt.Sprintf("%p", x))
	case *types.Qualified:
		recordIdentities(x.T, out)
	case *types.Pointer:
		recordIdentities(x.Elem, out)
	case *types.LValueReference:
		recordIdentities(x.Elem, out)
	case *types.RValueReference:
		recordIdentities(x.Elem, out)
	case *types.Array:
		recordIdentities(x.Elem, out)
	case *types.MemberPointer:
		recordIdentities(x.Class, out)
		recordIdentities(x.Elem, out)
	case *types.Func:
		recordIdentities(x.Ret, out)
		for _, p := range x.Params {
			recordIdentities(p.Type, out)
		}
	case *types.TemplateSpecialization:
		for _, a := range x.Args {
			if a.IsType {
				recordIdentities(a.Type, out)
			}
		}
	case *types.Pack:
		for _, e := range x.Elems {
			if e.IsType {
				recordIdentities(e.Type, out)
			}
		}
	}
}

// An awaitingInstance is a member template's specialization used before the
// member template was defined.
type awaitingInstance struct {
	tmpl *FuncSymbol
	args []types.TemplateArg
	inst *FuncSymbol
	at   ast.Tok
	done bool
}

// defineAwaiting instantiates the specializations of tmpl that were used
// before it had a definition, now that it has one, into the symbols the
// calls already resolved to.
func (a *Analyzer) defineAwaiting(tmpl *FuncSymbol) {
	for _, w := range a.awaiting {
		if w.done || w.tmpl != tmpl {
			continue
		}
		w.done = true
		info := tmpl.Template
		ikey := instanceKey(w.args)
		delete(info.Instances, ikey)
		fresh := a.instantiateWithArgs(tmpl, w.args, w.at)
		if fresh == nil || fresh == w.inst {
			continue
		}
		old := w.inst
		old.FuncType = fresh.FuncType
		old.Body = fresh.Body
		old.Decl = fresh.Decl
		old.Params = fresh.Params
		old.Constexpr = fresh.Constexpr
		old.Consteval = fresh.Consteval
		old.Method = fresh.Method
		info.Instances[ikey] = old
		for i, fn := range a.functions {
			if fn == fresh {
				a.functions[i] = old
			}
		}
	}
}

// reportUndefined reports the specializations that waited for a definition
// the translation unit never gave.
func (a *Analyzer) reportUndefined() {
	for _, w := range a.awaiting {
		if !w.done {
			a.errorAt(w.at, fmt.Sprintf("%s<%s> is used but the template is declared and not defined", w.tmpl.SymName, argsKey(w.args)))
		}
	}
}
