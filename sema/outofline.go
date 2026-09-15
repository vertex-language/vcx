package sema

import (
	"fmt"
	"strings"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/types"
)

// outOfLineMember recognizes out-of-line member function definitions (e.g. `void Widget::set(...) { ... }`).
// It returns the enclosing record symbol and the member's unqualified name.
func (a *Analyzer) outOfLineMember(d *ast.FuncDecl) (*RecordSymbol, string) {
	if d.Decl == nil {
		return nil, ""
	}
	if qn, isQualified := d.Decl.DeclName().(*ast.QualifiedName); isQualified {
		rs := a.qualifierRecord(qn)
		if rs == nil {
			return nil, ""
		}
		return rs, NameString(qn.Name, a.unit)
	}
	if d.Decl.DeclName() != nil {
		return nil, ""
	}
	for _, spec := range d.Specs.List {
		nt, isNamed := spec.(*ast.NamedTypeSpec)
		if !isNamed {
			continue
		}
		qn, isQualified := nt.Name.(*ast.QualifiedName)
		if !isQualified || len(qn.Qual) == 0 {
			return nil, ""
		}
		// `Widget::Widget` is the constructor and `Widget::~Widget` the
		// destructor; anything else is a qualified type with an unnamed
		// declarator, which is not a definition of anything.
		last := NameString(qn.Name, a.unit)
		class := NameString(qn.Qual[len(qn.Qual)-1], a.unit)
		if last != class && last != "~"+class {
			return nil, ""
		}
		rs := a.qualifierRecord(qn)
		if rs == nil {
			return nil, ""
		}
		return rs, last
	}
	return nil, ""
}

// qualifierRecord is the class a qualified name's qualifier denotes.
func (a *Analyzer) qualifierRecord(qn *ast.QualifiedName) *RecordSymbol {
	if len(qn.Qual) == 0 {
		return nil
	}
	class := &ast.QualifiedName{
		Span:   qn.Span,
		Global: qn.Global,
		Qual:   qn.Qual[:len(qn.Qual)-1],
		Name:   qn.Qual[len(qn.Qual)-1],
	}
	var syms []Symbol
	if len(class.Qual) == 0 && !class.Global.IsValid() {
		syms = LookupUnqualified(a.curScope, NameString(class.Name, a.unit))
	} else {
		syms = ResolveQualifiedName(class, a.curScope, a.globalScope, a.unit)
	}
	// `V<bool, A>::clear` is a member of the partial specialization whose
	// pattern is written `V<bool, A>`, not of the primary template: the
	// qualifier's arguments, read under the definition's template-head,
	// spell the pattern the specialization was declared with.
	if tn, isTemplate := class.Name.(*ast.TemplateName); isTemplate {
		for _, s := range syms {
			rs, isRec := s.(*RecordSymbol)
			if !isRec || rs.ClassTemplate == nil {
				continue
			}
			written := templateArgs(tn, a.curScope, a.unit)
			key := argsKey(written)
			if es := rs.ClassTemplate.ExplicitSpecs[a.specializationKey(rs, written, tn.Pos())]; es != nil && es.Sym != nil {
				return es.Sym
			}
			for _, ps := range rs.ClassTemplate.Partials {
				if ps.Sym != nil && argsKey(ps.Args) == key {
					return ps.Sym
				}
			}
		}
	}
	for _, s := range syms {
		if rs, isRec := s.(*RecordSymbol); isRec && rs.ClassScope != nil {
			return rs
		}
	}
	return nil
}

// checkOutOfLineMember attaches a definition to the member it defines and
// checks the body in the class's scope, where `x` means `this->x` and the
// parameter types are read as the declaration read them.
func (a *Analyzer) checkOutOfLineMember(d *ast.FuncDecl, rs *RecordSymbol, name string) {
	rec := rs.Record

	// The declared type is built in the class's scope, since a parameter
	// may name a nested type; the constructor and destructor get the
	// return type the class gives them, none.
	oldScope := a.curScope
	// A member template defined out of line has a template-head of its own
	// -- `template <class T> template <class I> void C<T>::f(I)` -- whose
	// parameters the class's scope has never heard of. They are carried
	// into a scope under the class's, so the signature and the body read
	// them.
	memberScope := rs.ClassScope
	if heads := enclosingTemplateParams(oldScope); len(heads) > 0 {
		memberScope = NewScope(rs.ClassScope, TemplateParamScope, nil)
		for _, sym := range heads {
			memberScope.Insert(sym)
		}
	}
	a.curScope = memberScope
	declInfo := BuildDeclSpecs(d.Specs, a.curScope, a.unit)
	declared, _ := BuildDeclarator(d.Decl, declInfo.Type, a.curScope, a.unit).(*types.Func)
	a.curScope = oldScope
	if declared == nil {
		a.errorAt(d.Pos(), "function definition with non-function type")
		return
	}
	if name == rec.Name || name == "~"+rec.Name {
		declared.Ret = types.Typ(types.Void)
	}
	if target := a.conversionTarget(d.Decl.DeclName(), rs.ClassScope); target != nil {
		declared.Ret = target
	}

	var fnSym *FuncSymbol
	for _, s := range rs.ClassScope.LookupLocal(name) {
		fn, isFn := s.(*FuncSymbol)
		if !isFn || fn.InClass != rec || !sameDeclaration(name, fn.FuncType, declared) {
			continue
		}
		// Two member templates can share a signature and differ only in
		// their template-heads -- vector<bool>'s constructors from an input
		// and from a forward iterator pair. Which one a definition defines
		// is told by its head, whose parameters are spelled as the
		// declaration's. Without that, the first declaration still waiting
		// for its definition is the one, so that the second definition of a
		// pair is not taken for a redefinition of the first.
		if sameTemplateHead(fn, a.curTemplateParams) {
			fnSym = fn
			break
		}
		if fnSym == nil || fnSym.Body != nil && fn.Body == nil {
			fnSym = fn
		}
	}
	if fnSym == nil {
		a.errorAt(d.Pos(), fmt.Sprintf("%s::%s does not match any declaration in the class", rec.Name, name))
		return
	}

	// A member of a class template as written is a member of every
	// specialization of it: the definition is kept for the instantiations,
	// and given to the ones already made.
	if fnSym.Template != nil && fnSym.Template.Decl != nil && fnSym.Template.Decl.Body == nil {
		fnSym.Template.Decl = d
		if n := len(fnSym.Template.Params); n <= len(a.curTemplateParams) {
			fnSym.Template.DefParams = adoptParams(fnSym.Template.Params, a.curTemplateParams[len(a.curTemplateParams)-n:])
		}
	}
	if a.curTemplateParams != nil && !a.instantiating && rs.Spec != nil {
		def := outOfLineDef{decl: d, name: name, memberTemplate: fnSym.Template != nil}
		if fnSym.Template != nil && len(fnSym.Template.Params) <= len(a.curTemplateParams) {
			def.headKey = a.templateHeadKey(a.curTemplateParams[len(a.curTemplateParams)-len(fnSym.Template.Params):])
			def.params = a.curTemplateParams[len(a.curTemplateParams)-len(fnSym.Template.Params):]
		}
		if a.outOfLine == nil {
			a.outOfLine = map[*ast.ClassSpec][]outOfLineDef{}
		}
		a.outOfLine[rs.Spec] = append(a.outOfLine[rs.Spec], def)
		for _, made := range a.instances {
			if made.pattern == rs.Spec {
				a.completeInstanceMember(made.inst, def, made.scope)
			}
		}
	}
	if fnSym.Body != nil {
		a.errorAt(d.Pos(), fmt.Sprintf("redefinition of %s::%s", rec.Name, name))
		return
	}

	// The definition supplies what the declaration could not: the
	// parameter names, the body, the mem-initializers. Default arguments
	// are the declaration's -- a definition cannot repeat them -- so which
	// parameters have one is kept.
	for i := range declared.Params {
		if i < len(fnSym.FuncType.Params) && fnSym.FuncType.Params[i].HasDefault {
			declared.Params[i].HasDefault = true
		}
	}
	fnSym.FuncType.Params = declared.Params
	fnSym.Decl = d
	if defs := extractDefaults(d.Decl); len(defs) > 0 {
		if len(fnSym.Defaults) < len(defs) {
			newDef := make([]ast.Expr, len(defs))
			copy(newDef, fnSym.Defaults)
			fnSym.Defaults = newDef
		}
		for i, def := range defs {
			if i < len(fnSym.Defaults) && fnSym.Defaults[i] == nil {
				fnSym.Defaults[i] = def
			}
		}
	}
	if comp, isComp := d.Body.(*ast.CompoundStmt); isComp {
		fnSym.Body = comp
	}
	if a.curTemplateParams == nil {
		a.functions = append(a.functions, fnSym)
	}

	fnScope := NewScope(memberScope, FunctionScope, fnSym)
	oldFunc, oldRecord := a.curFunc, a.curRecord
	a.curScope, a.curFunc, a.curRecord = fnScope, fnSym, rec

	fnSym.Params = nil
	for _, p := range declared.Params {
		pVar := &VarSymbol{SymName: p.Name, SymType: p.Type, SymScope: fnScope, IsParam: true}
		if p.Name != "" {
			fnScope.Insert(pVar)
		}
		fnSym.Params = append(fnSym.Params, pVar)
	}
	a.checkMemInits(d)
	a.CheckStmt(d.Body)

	a.curScope, a.curFunc, a.curRecord = oldScope, oldFunc, oldRecord
}

// sameDeclaration reports whether a definition's type matches a declaration's,
// including the target return type for conversion functions.
func sameDeclaration(name string, declared, defined *types.Func) bool {
	if !types.SameSignature(declared, defined) {
		return false
	}
	if strings.HasPrefix(name, "operator ") {
		return declared.Ret != nil && defined.Ret != nil && declared.Ret.Equal(defined.Ret)
	}
	return true
}

// conversionTarget extracts the target type from a conversion-function-id (operator T).
func (a *Analyzer) conversionTarget(name ast.Name, scope *Scope) types.Type {
	if qn, isQualified := name.(*ast.QualifiedName); isQualified {
		name = qn.Name
	}
	cn, isConv := name.(*ast.ConversionName)
	if !isConv || cn.Type == nil {
		return nil
	}
	info := BuildDeclSpecs(cn.Type.Specs, scope, a.unit)
	return BuildDeclarator(cn.Type.Decl, info.Type, scope, a.unit)
}

// enclosingTemplateParams is every template parameter the template-heads
// around scope declare, innermost head first.
func enclosingTemplateParams(scope *Scope) []Symbol {
	var out []Symbol
	for s := scope; s != nil && s.Kind == TemplateParamScope; s = s.Parent {
		for _, syms := range s.Symbols {
			for _, sym := range syms {
				if _, isParam := sym.(*TemplateParamSymbol); isParam {
					out = append(out, sym)
				}
			}
		}
	}
	return out
}

// An outOfLineDef is a member definition written outside its class template.
type outOfLineDef struct {
	decl *ast.FuncDecl
	name string
	// memberTemplate is set when the member is a template of its own, whose
	// instantiations read the definition rather than a body checked once.
	memberTemplate bool
	// headKey is a member template's own template-head as the definition
	// writes it (see templateHeadKey), which tells it from another member
	// template of the same name and parameter count.
	headKey string
	// params are that head's parameters: the definition's names for them --
	// `_ForwardIterator` where the class declared `_Iterator` -- are the ones
	// its signature and body use.
	params []*TemplateParamSymbol
}

// An instanceMade is a class specialization, the pattern it came from, and
// the scope its template arguments are bound in.
type instanceMade struct {
	inst    *RecordSymbol
	pattern *ast.ClassSpec
	scope   *Scope
}

// completeInstanceMember gives a specialization the definition its pattern's
// member has out of line: `template <class T> void V<T>::clear() {}` is
// V<int>::clear's body, checked with T bound, and the definition of a member
// template is what V<int>::each<I> instantiates.
func (a *Analyzer) completeInstanceMember(inst *RecordSymbol, def outOfLineDef, scope *Scope) {
	if inst == nil || inst.ClassScope == nil {
		return
	}
	d := ast.Clone(def.decl)
	if def.memberTemplate {
		want := paramCount(d)
		var fallback *FuncSymbol
		for _, s := range inst.ClassScope.LookupLocal(def.name) {
			fn, isFn := s.(*FuncSymbol)
			if !isFn || fn.Template == nil || fn.Template.Decl != nil && fn.Template.Decl.Body != nil {
				continue
			}
			if fn.Template.Decl != nil && paramCount(fn.Template.Decl) != want {
				continue
			}
			// basic_string's two __init templates both take two parameters;
			// the head -- which enable_if it spells -- says which this is.
			if def.headKey != "" && fn.Template.HeadKey == def.headKey {
				fn.Template.Decl = d
				fn.Template.DefParams = adoptParams(fn.Template.Params, def.params)
				a.defineAwaiting(fn)
				return
			}
			if fallback == nil {
				fallback = fn
			}
		}
		if fallback != nil && (def.headKey == "" || fallback.Template.HeadKey == "") {
			fallback.Template.Decl = d
			fallback.Template.DefParams = adoptParams(fallback.Template.Params, def.params)
			a.defineAwaiting(fallback)
		}
		return
	}
	saved := a.enterInstantiation(scope)
	wasInstantiating := a.instantiating
	a.instantiating = true
	a.checkOutOfLineMember(d, inst, def.name)
	a.instantiating = wasInstantiating
	a.leaveInstantiation(saved)
}

// paramCount is how many parameters a function declaration lists, or -1.
func paramCount(d *ast.FuncDecl) int {
	if d == nil {
		return -1
	}
	fd := funcDeclaratorOf(d.Decl)
	if fd == nil {
		return -1
	}
	return len(fd.Params)
}

// sameTemplateHead reports whether a member template's parameters are
// spelled as a definition's innermost template-head spells them.
func sameTemplateHead(fn *FuncSymbol, head []*TemplateParamSymbol) bool {
	// The heads in scope run outermost first -- the class's, then the
	// member's -- and the member template's own are the innermost.
	if fn.Template == nil || len(fn.Template.Params) > len(head) {
		return false
	}
	inner := head[len(head)-len(fn.Template.Params):]
	for i, p := range fn.Template.Params {
		if p.SymName != inner[i].SymName || p.IsType != inner[i].IsType {
			return false
		}
	}
	return true
}

// adoptParams is a member template's parameters as its out-of-line definition
// names them, with the default arguments the declaration gave. A definition
// whose head does not line up with the declaration's leaves them as they are.
func adoptParams(declared, defined []*TemplateParamSymbol) []*TemplateParamSymbol {
	if len(defined) != len(declared) {
		return declared
	}
	out := make([]*TemplateParamSymbol, len(defined))
	for i, p := range defined {
		c := *p
		if c.Default == nil {
			c.Default = declared[i].Default
		}
		out[i] = &c
	}
	return out
}
