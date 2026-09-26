package sema

import (
	"fmt"
	"github.com/vertex-language/vcx/ast"
	"strings"

	"github.com/vertex-language/vcx/types"
)

// ScopeKind distinguishes the kind of lexical or semantic scope.
type ScopeKind uint8

const (
	GlobalScope ScopeKind = iota
	NamespaceScope
	ClassScope
	BlockScope
	FunctionScope
	PrototypeScope
	TemplateParamScope
)

func (k ScopeKind) String() string {
	switch k {
	case GlobalScope:
		return "global"
	case NamespaceScope:
		return "namespace"
	case ClassScope:
		return "class"
	case BlockScope:
		return "block"
	case FunctionScope:
		return "function"
	case PrototypeScope:
		return "prototype"
	case TemplateParamScope:
		return "template-param"
	}
	return "unknown"
}

// Scope represents a C++ lexical or semantic scope.
type Scope struct {
	Kind   ScopeKind
	Parent *Scope
	Entity any // *NamespaceSymbol, *types.Record, *FuncSymbol, etc.

	Symbols         map[string][]Symbol
	UsingNamespaces []*Scope

	// Instantiation marks the scope a template instance is declared in,
	// which stands between the instance's body and the scope the
	// template was declared in. The instance is the only thing in it,
	// and it is not a scope of the program: a name the body looks up
	// that finds the instance here goes on to the template's own scope,
	// where the instance's overloads are, and takes them all.
	Instantiation bool

	// InlineNamespaces are the inline namespaces declared directly in this scope.
	InlineNamespaces []*Scope

	// AnonRecords maps unnamed class-specifiers to their types on the root scope.
	AnonRecords map[*ast.ClassSpec]*types.Record
	UsingDecls  map[string][]Symbol

	// Decltype evaluates decltype(e) within this scope.
	Decltype func(e ast.Expr, scope *Scope) types.Type

	// Instantiate instantiates a class template specialization.
	Instantiate func(tmpl *RecordSymbol, args []types.TemplateArg, at ast.Tok) types.Type

	// AliasInstantiate instantiates an alias template.
	AliasInstantiate func(alias *TypeSymbol, args []types.TemplateArg, at ast.Tok) types.Type
	EvalConst        func(e ast.Expr, scope *Scope) (int64, bool)

	// ExpandValues expands a non-type pack expansion into concrete values.
	ExpandValues func(pe *ast.PackExpansion, scope *Scope) ([]int64, bool)

	// RecordSymbols maps record types to their defining symbols on the root scope.
	RecordSymbols map[*types.Record]*RecordSymbol

	// ObjC is an Objective-C++ unit's classes and protocols (objc.go),
	// on the root scope.
	ObjC *ObjCTables
}

// remove takes a symbol out of the scope it was inserted into.
func (s *Scope) remove(sym Symbol) {
	syms := s.Symbols[sym.Name()]
	for i, cur := range syms {
		if cur == sym {
			s.Symbols[sym.Name()] = append(syms[:i:i], syms[i+1:]...)
			return
		}
	}
}

// noteRecord registers a class's symbol on the root.
func (s *Scope) noteRecord(rs *RecordSymbol) {
	r := s.root()
	if r.RecordSymbols == nil {
		r.RecordSymbols = map[*types.Record]*RecordSymbol{}
	}
	if rs.Record != nil {
		r.RecordSymbols[rs.Record] = rs
	}
	// An unnamed class is reachable only from its own class-specifier:
	// `struct { int a; } x;` names a member of a type nothing else can
	// name, and the type builder finds it by the specifier.
	if rs.SymName == "" && rs.Spec != nil {
		if r.AnonRecords == nil {
			r.AnonRecords = map[*ast.ClassSpec]*types.Record{}
		}
		r.AnonRecords[rs.Spec] = rs.Record
	}
}

// anonymousRecord is the class an unnamed class-specifier declared.
func (s *Scope) anonymousRecord(spec *ast.ClassSpec) *types.Record {
	return s.root().AnonRecords[spec]
}

// recordSymbol finds the symbol a class was declared by, or nil.
func (s *Scope) recordSymbol(rec *types.Record) *RecordSymbol {
	return s.root().RecordSymbols[rec]
}

// root is the scope every resolver is installed on.
func (s *Scope) root() *Scope {
	cur := s
	for cur.Parent != nil {
		cur = cur.Parent
	}
	return cur
}

// instantiate finds the class-template resolver on the root of this tree.
func (s *Scope) instantiate() func(*RecordSymbol, []types.TemplateArg, ast.Tok) types.Type {
	return s.root().Instantiate
}

// decltype finds the resolver installed on the root of this scope tree.
func (s *Scope) decltype() func(ast.Expr, *Scope) types.Type {
	for cur := s; cur != nil; cur = cur.Parent {
		if cur.Decltype != nil {
			return cur.Decltype
		}
	}
	return nil
}

// NewScope creates a child scope under parent.
func NewScope(parent *Scope, kind ScopeKind, entity any) *Scope {
	return &Scope{
		Kind:            kind,
		Parent:          parent,
		Entity:          entity,
		Symbols:         make(map[string][]Symbol),
		UsingDecls:      make(map[string][]Symbol),
		UsingNamespaces: nil,
	}
}

// Insert inserts a symbol into this scope.
func (s *Scope) Insert(sym Symbol) error {
	_, err := s.insert(sym)
	return err
}

// InsertFunc inserts a function and returns the symbol that ends up in the
// scope, which is not always the one passed in: a definition merges into the
// declaration that preceded it, and the declaration is the object everything
// checked before it is still pointing at.
func (s *Scope) InsertFunc(fn *FuncSymbol) (*FuncSymbol, error) {
	sym, err := s.insert(fn)
	if surviving, ok := sym.(*FuncSymbol); ok {
		return surviving, err
	}
	return fn, err
}

func (s *Scope) insert(sym Symbol) (Symbol, error) {
	name := sym.Name()
	if name == "" {
		return sym, nil
	}

	existing := s.Symbols[name]
	// Function overloading: multiple functions with the same name are allowed if signatures differ.
	if fn, ok := sym.(*FuncSymbol); ok {
		for _, ex := range existing {
			exFn, isFn := ex.(*FuncSymbol)
			if !isFn {
				// A class name can be hidden by a function or object of the same name.
				if _, isRec := ex.(*RecordSymbol); isRec {
					continue
				}
				return nil, fmt.Errorf("redeclaration of %q as different kind of symbol", name)
			}
			if !exFn.FuncType.Equal(fn.FuncType) {
				continue
			}
			// Overloads can differ by trailing requires-clauses.
			if !sameConstraints(exFn, fn) {
				continue
			}
			// And by the concepts their constrained placeholders named,
			// which is what an abbreviated function template has in place
			// of a template-head: `show(std::integral auto)` beside
			// `show(std::floating_point auto)` is two templates.
			if !sameAutoConcepts(exFn, fn) {
				continue
			}
			// And function templates by their template-heads: basic_string's
			// __init from an input and from a forward iterator pair have one
			// signature and two heads, each with its own enable_if.
			if !sameTemplateParams(exFn, fn) {
				continue
			}
			if len(fn.Defaults) > 0 {
				if len(exFn.Defaults) < len(fn.Defaults) {
					newDef := make([]ast.Expr, len(fn.Defaults))
					copy(newDef, exFn.Defaults)
					exFn.Defaults = newDef
				}
				for i, def := range fn.Defaults {
					if i < len(exFn.Defaults) && exFn.Defaults[i] == nil {
						exFn.Defaults[i] = def
					}
				}
			}
			if len(exFn.Defaults) > len(fn.Defaults) {
				newDef := make([]ast.Expr, len(exFn.Defaults))
				copy(newDef, fn.Defaults)
				fn.Defaults = newDef
				for i, def := range exFn.Defaults {
					if i < len(fn.Defaults) && fn.Defaults[i] == nil {
						fn.Defaults[i] = def
					}
				}
			}
			for i := range exFn.FuncType.Params {
				if i < len(fn.FuncType.Params) {
					if fn.FuncType.Params[i].HasDefault {
						exFn.FuncType.Params[i].HasDefault = true
					}
					if exFn.FuncType.Params[i].HasDefault {
						fn.FuncType.Params[i].HasDefault = true
					}
				}
			}
			// Definition following a declaration fills in the existing symbol.
			if exFn.Body == nil && fn.Body != nil {
				// [dcl.fct]/8 -- parameter names are not part of the type,
				// and the body sees the definition's: `int f(int);` then
				// `int f(int b) { return b; }` declares b.
				for i := range exFn.FuncType.Params {
					if i < len(fn.FuncType.Params) {
						exFn.FuncType.Params[i].Name = fn.FuncType.Params[i].Name
					}
				}
				exFn.Body = fn.Body
				exFn.Decl = fn.Decl
				exFn.Params = fn.Params
				exFn.Inline = exFn.Inline || fn.Inline
				exFn.Constexpr = exFn.Constexpr || fn.Constexpr
				exFn.Consteval = exFn.Consteval || fn.Consteval
				return exFn, nil
			}
			if exFn.Body != nil && fn.Body != nil {
				return nil, fmt.Errorf("redefinition of function %q with same signature", name)
			}
			return exFn, nil
		}
		if s.Kind != ClassScope {
			// Place the function ahead of any class it hides.
			s.Symbols[name] = append([]Symbol{sym}, existing...)
			return sym, nil
		}
		s.Symbols[name] = append(existing, sym)
		return sym, nil
	}

	// [class.mem]: a non-static data member may share its class's name when
	// the class declares no constructor, as Darwin's
	// `struct ip_opts { char ip_opts[40]; }` does. The member hides the
	// injected-class-name for ordinary lookup, so it goes first.
	if v, isVar := sym.(*VarSymbol); isVar && s.Kind == ClassScope && len(existing) == 1 {
		if rs, isRec := existing[0].(*RecordSymbol); isRec && rs.Record != nil && rs.Record == s.Entity {
			s.Symbols[name] = []Symbol{v, rs}
			return v, nil
		}
	}

	// Non-function symbols cannot be redefined in the same scope
	if len(existing) > 0 {
		if v, isVar := sym.(*VarSymbol); isVar {
			for _, ex := range existing {
				exV, isVar := ex.(*VarSymbol)
				if !isVar || !exV.SymType.Equal(v.SymType) && !isDependentType(v.SymType) && !isDependentType(exV.SymType) {
					continue
				}
				// A definition completes a prior non-defining declaration.
				switch {
				case !exV.Defined && v.Defined:
					// [dcl.stc]/7: linkage is the first declaration's, so
					// `extern const T x; const T x = {...};` stays external
					// -- the definition does not make it internal the way a
					// const at namespace scope alone would be.
					exV.Init, exV.BracedInit, exV.Defined = v.Init, v.BracedInit, true
					if exV.Storage != StorageExtern {
						exV.Storage = v.Storage
					}
					exV.Constexpr = exV.Constexpr || v.Constexpr
					if exV.AsmLabel == "" {
						exV.AsmLabel = v.AsmLabel
					}
					return exV, nil
				case !v.Defined:
					if exV.AsmLabel == "" {
						exV.AsmLabel = v.AsmLabel
					}
					return exV, nil
				}
			}
		}
		if ts, ok := sym.(*TypeSymbol); ok {
			for _, ex := range existing {
				if rs, isRec := ex.(*RecordSymbol); isRec {
					if rec := types.AsRecord(types.Unqualify(ts.SymType)); rec == rs.Record {
						return ex, nil
					}
				}
				if exTs, ok := ex.(*TypeSymbol); ok {
					if exTs.SymType != nil && ts.SymType != nil && exTs.SymType.Equal(ts.SymType) {
						return ex, nil
					}
				}
			}
		}
		return nil, fmt.Errorf("redefinition of %q", name)
	}

	s.Symbols[name] = []Symbol{sym}
	return sym, nil
}

// LookupLocal searches for a name only in this exact scope (ignoring parent and using directives).
//
// A using-declaration's symbols join what the scope declares under the same
// name rather than being shadowed by it: `using Base::f` beside a derived
// `f` makes one overload set of both ([namespace.udecl]/13). The exception
// is a function the scope declares with the same parameters, which hides
// the one brought in instead of colliding with it ([namespace.udecl]/14).
func (s *Scope) LookupLocal(name string) []Symbol {
	syms := s.Symbols[name]
	using := s.UsingDecls[name]
	switch {
	case len(using) == 0:
		return syms
	case len(syms) == 0:
		return using
	}
	out := make([]Symbol, 0, len(syms)+len(using))
	out = append(out, syms...)
	for _, u := range using {
		if !hiddenByLocal(u, syms) {
			out = append(out, u)
		}
	}
	return out
}

// hiddenByLocal reports whether a using-declared symbol is a function the
// scope already declares with the same parameters.
func hiddenByLocal(brought Symbol, declared []Symbol) bool {
	fn, isFn := brought.(*FuncSymbol)
	if !isFn {
		return false
	}
	for _, d := range declared {
		if df, ok := d.(*FuncSymbol); ok && types.SameSignature(df.FuncType, fn.FuncType) {
			return true
		}
	}
	return false
}

// AddUsingNamespace imports symbols from ns into unqualified lookups in this scope.
func (s *Scope) AddUsingNamespace(ns *Scope) {
	if ns == nil {
		return
	}
	for _, existing := range s.UsingNamespaces {
		if existing == ns {
			return
		}
	}
	s.UsingNamespaces = append(s.UsingNamespaces, ns)
}

// AddInlineNamespace records ns as an inline namespace of this one.
func (s *Scope) AddInlineNamespace(ns *Scope) {
	for _, existing := range s.InlineNamespaces {
		if existing == ns {
			return
		}
	}
	s.InlineNamespaces = append(s.InlineNamespaces, ns)
	s.AddUsingNamespace(ns)
}

// LookupNamespaceMember performs qualified lookup into a namespace, its inline
// namespaces, and nominated using-namespaces.
func (s *Scope) LookupNamespaceMember(name string) []Symbol {
	return s.lookupNamespaceMember(name, map[*Scope]bool{})
}

func (s *Scope) lookupNamespaceMember(name string, seen map[*Scope]bool) []Symbol {
	if s == nil || seen[s] {
		return nil
	}
	seen[s] = true
	var found []Symbol
	var inlineSet func(*Scope)
	inlineSet = func(ns *Scope) {
		found = append(found, ns.LookupLocal(name)...)
		for _, in := range ns.InlineNamespaces {
			if !seen[in] {
				seen[in] = true
				inlineSet(in)
			}
		}
	}
	inlineSet(s)
	if len(found) > 0 {
		return deduplicateSymbols(found)
	}
	for _, used := range s.UsingNamespaces {
		found = append(found, used.lookupNamespaceMember(name, seen)...)
	}
	return deduplicateSymbols(found)
}

// LookupAssociated performs argument-dependent lookup (ADL) on an associated
// namespace, including inline enclosing/nested namespaces.
func (s *Scope) LookupAssociated(name string) []Symbol {
	if s == nil {
		return nil
	}
	top := s
	for top.Parent != nil && top.Parent.hasInline(top) {
		top = top.Parent
	}
	var found []Symbol
	seen := map[*Scope]bool{}
	var walk func(*Scope)
	walk = func(ns *Scope) {
		if seen[ns] {
			return
		}
		seen[ns] = true
		found = append(found, ns.LookupLocal(name)...)
		for _, in := range ns.InlineNamespaces {
			walk(in)
		}
	}
	walk(top)
	return deduplicateSymbols(found)
}

func (s *Scope) hasInline(ns *Scope) bool {
	for _, in := range s.InlineNamespaces {
		if in == ns {
			return true
		}
	}
	return false
}

// AddUsingDecl brings a specific symbol into this scope under name.
func (s *Scope) AddUsingDecl(name string, sym Symbol) {
	s.UsingDecls[name] = append(s.UsingDecls[name], sym)
}

// InnermostRecord finds the closest enclosing class/struct scope, if any.
func (s *Scope) InnermostRecord() *types.Record {
	for cur := s; cur != nil; cur = cur.Parent {
		if cur.Kind == ClassScope {
			if r, ok := cur.Entity.(*types.Record); ok {
				return r
			}
		}
	}
	return nil
}

// InnermostFunc finds the closest enclosing function scope, if any.
func (s *Scope) InnermostFunc() *FuncSymbol {
	for cur := s; cur != nil; cur = cur.Parent {
		if cur.Kind == FunctionScope {
			if fn, ok := cur.Entity.(*FuncSymbol); ok {
				return fn
			}
		}
	}
	return nil
}

// Path returns the names of the namespaces and classes enclosing this scope,
// ordered outermost first.
func (s *Scope) Path() []string {
	var rev []string
	for cur := s; cur != nil; cur = cur.Parent {
		switch e := cur.Entity.(type) {
		case *NamespaceSymbol:
			rev = append(rev, e.SymName)
		case *types.Record:
			if e.Name != "" {
				rev = append(rev, e.Name)
			}
		}
	}
	path := make([]string, len(rev))
	for i, n := range rev {
		path[len(rev)-1-i] = n
	}
	return path
}

// sameConstraints reports whether two declarations carry the same
// requires-clauses, spelled alike: the same count, and each the same
// tokens.
func sameConstraints(x, y *FuncSymbol) bool {
	if len(x.Constraints) != len(y.Constraints) {
		return false
	}
	// What they require, where both declarations recorded it. Two
	// constraints of the same length written in different places are not
	// the same constraint: `Int` and `Flt` are both three characters, and
	// comparing extents merged the two overloads into one.
	if len(x.ConstraintKeys) == len(x.Constraints) && len(y.ConstraintKeys) == len(y.Constraints) {
		for i := range x.ConstraintKeys {
			if x.ConstraintKeys[i] != y.ConstraintKeys[i] {
				return false
			}
		}
		return true
	}
	for i := range x.Constraints {
		a, b := x.Constraints[i], y.Constraints[i]
		if a.Pos() != b.Pos() && (a.End()-a.Pos() != b.End()-b.Pos()) {
			return false
		}
	}
	return true
}

// sameTemplateParams reports whether two function declarations have
// equivalent template-heads: the same kinds of parameter, and non-type
// parameters of the same type once each parameter is read by its position.
// Parameter names do not count -- `template <class T> void swap(T&, T&);`
// and `template <class _Tp> void swap(_Tp&, _Tp&) {}` are one template --
// but what a non-type parameter's type says does: basic_string's __init from
// an input and from a forward iterator pair differ only in the enable_if
// condition their second parameter spells.
func sameTemplateParams(a, b *FuncSymbol) bool {
	// A declaration whose template-head was not recorded when it was
	// inserted -- a function template declared ahead of its definition in
	// another header -- says nothing against the merge.
	if a.Template == nil || b.Template == nil {
		return true
	}
	if len(a.Template.Params) != len(b.Template.Params) {
		return false
	}
	// The head as written decides when both declarations recorded it: the
	// types a dependent non-type parameter resolves to all look alike.
	if a.Template.HeadKey != "" && b.Template.HeadKey != "" {
		return a.Template.HeadKey == b.Template.HeadKey
	}
	for i, p := range a.Template.Params {
		q := b.Template.Params[i]
		if p.IsType != q.IsType || p.IsPack != q.IsPack || p.IsTemplate != q.IsTemplate {
			return false
		}
		// A parameter invented for a constrained placeholder carries the
		// concept rather than an expression, so the heads are told apart
		// by it: two `show(C auto)` with different C are two templates.
		if p.AutoConceptKey != q.AutoConceptKey {
			return false
		}
		if !p.IsType && canonicalParamType(p.SymType, a.Template.Params) != canonicalParamType(q.SymType, b.Template.Params) {
			return false
		}
	}
	return true
}

// canonicalParamType spells a non-type template parameter's type with every
// parameter of its head replaced by that parameter's position.
func canonicalParamType(t types.Type, head []*TemplateParamSymbol) string {
	if t == nil {
		return ""
	}
	text := t.String()
	for i, p := range head {
		if p.SymName == "" {
			continue
		}
		text = replaceIdentifier(text, p.SymName, fmt.Sprintf("$%d", i))
	}
	return text
}

// replaceIdentifier replaces whole-identifier occurrences of name in text.
func replaceIdentifier(text, name, with string) string {
	isIdent := func(c byte) bool {
		return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
	}
	var out []byte
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], name) && (i == 0 || !isIdent(text[i-1])) &&
			(i+len(name) == len(text) || !isIdent(text[i+len(name)])) {
			out = append(out, with...)
			i += len(name)
			continue
		}
		out = append(out, text[i])
		i++
	}
	return string(out)
}

// sameAutoConcepts reports whether two declarations' constrained
// placeholders name the same concepts.
func sameAutoConcepts(x, y *FuncSymbol) bool {
	if len(x.AutoConceptKeys) != len(y.AutoConceptKeys) {
		return false
	}
	for i := range x.AutoConceptKeys {
		if x.AutoConceptKeys[i] != y.AutoConceptKeys[i] {
			return false
		}
	}
	return true
}
