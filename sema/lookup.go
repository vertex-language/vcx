package sema

import (
	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/types"
)

// LookupUnqualified performs C++ unqualified name lookup starting from scope.
func LookupUnqualified(scope *Scope, name string) []Symbol {
	if scope == nil || name == "" {
		return nil
	}

	for cur := scope; cur != nil; cur = cur.Parent {
		// 1. Direct local declarations in cur
		if syms := cur.LookupLocal(name); len(syms) > 0 {
			return syms
		}

		// 2. Class scope: look in base classes
		if cur.Kind == ClassScope {
			if r, ok := cur.Entity.(*types.Record); ok {
				// Injected-class-name of the base class.
				if rs := injectedBaseName(cur, r, name, map[*types.Record]bool{}); rs != nil {
					return []Symbol{rs}
				}
				if syms := lookupRecordMember(r, name, make(map[*types.Record]bool)); len(syms) > 0 {
					return syms
				}
			}
		}

		// 3. Nominated namespaces (using namespace N;), and the inline
		// namespaces, which nominate themselves in their parent.
		if len(cur.UsingNamespaces) > 0 {
			var gathered []Symbol
			for _, ns := range cur.UsingNamespaces {
				if syms := ns.LookupNamespaceMember(name); len(syms) > 0 {
					gathered = append(gathered, syms...)
				}
			}
			if len(gathered) > 0 {
				return deduplicateSymbols(gathered)
			}
		}
	}

	return nil
}

// LookupQualified performs C++ qualified lookup (e.g. N::name or Class::name).
func LookupQualified(target any, name string) []Symbol {
	if target == nil || name == "" {
		return nil
	}

	switch t := target.(type) {
	case *Scope:
		if t == nil {
			return nil
		}
		if t.Kind == ClassScope {
			if r, ok := t.Entity.(*types.Record); ok {
				return lookupRecordMember(r, name, make(map[*types.Record]bool))
			}
		}
		if t.Kind == NamespaceScope || t.Parent == nil {
			return t.LookupNamespaceMember(name)
		}
		return t.LookupLocal(name)

	case *NamespaceSymbol:
		if t.InnerScope != nil {
			return t.InnerScope.LookupNamespaceMember(name)
		}

	case *RecordSymbol:
		// The class's own scope first. It holds the symbols the analysis
		// built while walking the body -- a static data member with its
		// initializer, a member alias, a nested type -- and those carry
		// more than the layout does: types.Record records a field's name
		// and type, which is what a layout needs and not what a constant
		// expression needs. `S::count` in a static_assert has to reach the
		// initializer, and only the scope has it.
		if t.ClassScope != nil {
			if syms := t.ClassScope.LookupLocal(name); len(syms) > 0 {
				return syms
			}
			// Check base scopes for inherited static members and aliases.
			if syms := lookupInheritedScopes(t.ClassScope, t.Record, name, map[*types.Record]bool{t.Record: true}); len(syms) > 0 {
				return syms
			}
		}
		return lookupRecordMember(t.Record, name, make(map[*types.Record]bool))

	case *types.Record:
		return lookupRecordMember(t, name, make(map[*types.Record]bool))

	case *EnumSymbol:
		return lookupEnumMember(t.Enum, name)

	case *types.Enum:
		return lookupEnumMember(t, name)
	}

	return nil
}

// ResolveQualifiedName resolves a qualified name (A::B::c, ::c, etc.) through scopes.
func ResolveQualifiedName(q *ast.QualifiedName, curScope *Scope, globalScope *Scope, u ast.Unit) []Symbol {
	if q == nil {
		return nil
	}

	var curTarget any
	if q.Global.IsValid() {
		if globalScope == nil && curScope != nil {
			globalScope = curScope.root()
		}
		curTarget = globalScope
	}

	for i, qual := range q.Qual {
		// A qualifier that is a template-id names a specialization:
		// `Fact<5>::value`, `true_type::value` through an alias. It is
		// built as a type -- instantiated when its arguments are
		// settled -- and the lookup continues in the class that came
		// back, through the symbol the class was declared by so that a
		// static member's initializer is reachable.
		if tn, isTemplate := qual.(*ast.TemplateName); isTemplate {
			// The template is found where the qualifiers so far lead --
			// `std::is_pointer<int>::value` finds is_pointer in std --
			// and its arguments mean what they mean where the name was
			// written, so the specialization is built as the qualified
			// template-id `std::is_pointer<int>` in the current scope.
			var name ast.Name = tn
			if i > 0 || q.Global.IsValid() {
				name = &ast.QualifiedName{Span: ast.Span{Lo: q.Pos(), Hi: tn.End()}, Global: q.Global, Qual: q.Qual[:i], Name: tn}
			}
			specs := &ast.DeclSpecs{Span: tn.Span, List: []ast.DeclSpec{&ast.NamedTypeSpec{Span: tn.Span, Typename: ast.NoTok, Name: name}}}
			info := BuildDeclSpecs(specs, curScope, u)
			if info.Type == nil || isDependentType(info.Type) {
				return []Symbol{&DependentSymbol{SymName: NameString(q, u), SymPos: q.Pos()}}
			}
			rec := types.AsRecord(types.Unqualify(info.Type))
			if rec == nil {
				return nil
			}
			curTarget = recordTarget(curScope, rec)
			continue
		}

		qualName := NameString(qual, u)
		var syms []Symbol
		if curTarget == nil {
			syms = LookupUnqualified(curScope, qualName)
		} else {
			syms = LookupQualified(curTarget, qualName)
		}
		if len(syms) == 0 {
			return nil
		}
		targetSym := syms[0]
		for _, s := range syms {
			if _, isRec := s.(*RecordSymbol); isRec {
				targetSym = s
				break
			}
			if _, isNs := s.(*NamespaceSymbol); isNs {
				targetSym = s
				break
			}
		}
		switch t := targetSym.(type) {
		case *FuncSymbol:
			if t.InClass != nil {
				curTarget = t.InClass
			} else {
				curTarget = t
			}
		case *TypeSymbol:
			// An alias for a class -- `using true_type = ...` -- or for
			// something dependent, in which case the whole name is.
			if isDependentType(t.SymType) {
				return []Symbol{&DependentSymbol{SymName: NameString(q, u), SymPos: q.Pos()}}
			}
			if rec := types.AsRecord(types.Unqualify(t.SymType)); rec != nil {
				curTarget = recordTarget(curScope, rec)
			} else if e, isEnum := types.Unqualify(t.SymType).(*types.Enum); isEnum {
				curTarget = e
			} else {
				curTarget = t
			}
		case *TemplateParamSymbol:
			return []Symbol{&DependentSymbol{SymName: NameString(q, u), SymPos: q.Pos()}}
		case *DependentSymbol:
			return []Symbol{t}
		case *RecordSymbol:
			curTarget = recordTarget(curScope, t.Record)
		default:
			curTarget = targetSym
		}
	}

	finalName := ""
	if q.Name != nil {
		finalName = NameString(q.Name, u)
	}

	if curTarget == nil {
		return LookupUnqualified(curScope, finalName)
	}
	return LookupQualified(curTarget, finalName)
}

func lookupEnumMember(e *types.Enum, name string) []Symbol {
	if e == nil {
		return nil
	}
	for _, item := range e.Enumerators {
		if item.Name == name {
			return []Symbol{&EnumeratorSymbol{
				SymName: item.Name,
				Enum:    e,
				Val:     item.Val,
			}}
		}
	}
	return nil
}

func lookupRecordMember(r *types.Record, name string, visited map[*types.Record]bool) []Symbol {
	if r == nil || visited[r] {
		return nil
	}
	visited[r] = true

	var results []Symbol

	// 1. Fields (including anonymous union members).
	for _, f := range r.Fields {
		if f.Name == name {
			results = append(results, &VarSymbol{
				SymName: f.Name,
				SymType: f.Type,
			})
		}
		if f.Name == "" {
			if inner := types.AsRecord(types.Unqualify(f.Type)); inner != nil {
				for _, g := range inner.Fields {
					if g.Name == name {
						results = append(results, &VarSymbol{SymName: g.Name, SymType: g.Type})
					}
				}
			}
		}
	}

	// 2. Methods
	for _, m := range r.Methods {
		if m.Name == name {
			results = append(results, &FuncSymbol{
				SymName:     m.Name,
				FuncType:    m.Func,
				Virtual:     m.Virtual,
				PureVirtual: m.PureVirtual,
				Static:      m.Static,
				Explicit:    m.Explicit,
				Friend:      m.Friend,
				InClass:     r,
				Method:      m,
			})
		}
	}

	if len(results) > 0 {
		return results
	}

	// 3. Base classes
	for _, b := range r.Bases {
		if bRec, ok := types.Unqualify(b.Type).(*types.Record); ok {
			baseSyms := lookupRecordMember(bRec, name, visited)
			results = append(results, baseSyms...)
		}
	}

	return deduplicateSymbols(results)
}

// LookupADL performs Argument-Dependent Lookup (Koenig lookup) for function calls.
func LookupADL(funcName string, argTypes []types.Type) []*FuncSymbol {
	if funcName == "" || len(argTypes) == 0 {
		return nil
	}

	var candidateFuncs []*FuncSymbol
	seenNamespaces := make(map[*Scope]bool)

	for _, argT := range argTypes {
		collectAssociatedNamespaces(argT, seenNamespaces)
	}

	for nsScope := range seenNamespaces {
		syms := nsScope.LookupAssociated(funcName)
		for _, s := range syms {
			if fn, ok := s.(*FuncSymbol); ok {
				candidateFuncs = append(candidateFuncs, fn)
			}
		}
	}

	return candidateFuncs
}

func collectAssociatedNamespaces(t types.Type, nsMap map[*Scope]bool) {
	if t == nil {
		return
	}
	u := types.Unqualify(types.RemoveReference(t))

	switch v := u.(type) {
	case *types.Pointer:
		collectAssociatedNamespaces(v.Elem, nsMap)
	case *types.Record:
		// Record's enclosing scope
		// (In a full translation unit, the symbol's scope would be recorded)
	case *types.Enum:
		// Enum's enclosing scope
	case *types.TemplateSpecialization:
		for _, arg := range v.Args {
			if arg.IsType {
				collectAssociatedNamespaces(arg.Type, nsMap)
			}
		}
	}
}

func deduplicateSymbols(syms []Symbol) []Symbol {
	if len(syms) <= 1 {
		return syms
	}
	out := make([]Symbol, 0, len(syms))
	seen := make(map[Symbol]bool)
	for _, s := range syms {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// recordTarget is the best thing to look a member up in for a class: the
// symbol it was declared by, with its scope, when the tree has one; the
// bare record otherwise.
func recordTarget(scope *Scope, rec *types.Record) any {
	if scope != nil {
		if rs := scope.recordSymbol(rec); rs != nil {
			return rs
		}
	}
	return rec
}

// lookupInheritedScopes looks a name up in the class scopes of a class's
// bases, depth first, through the symbols the bases were declared by.
func lookupInheritedScopes(scope *Scope, rec *types.Record, name string, visited map[*types.Record]bool) []Symbol {
	for _, b := range rec.Bases {
		br := types.AsRecord(types.Unqualify(b.Type))
		if br == nil || visited[br] {
			continue
		}
		visited[br] = true
		if rs := scope.recordSymbol(br); rs != nil && rs.ClassScope != nil {
			if syms := rs.ClassScope.LookupLocal(name); len(syms) > 0 {
				return syms
			}
		}
		if syms := lookupInheritedScopes(scope, br, name, visited); len(syms) > 0 {
			return syms
		}
	}
	return nil
}

// injectedBaseName is the symbol of the base class of r, at any depth,
// whose name is name -- what that name means inside r.
func injectedBaseName(scope *Scope, r *types.Record, name string, visited map[*types.Record]bool) Symbol {
	for _, b := range r.Bases {
		br := types.AsRecord(types.Unqualify(b.Type))
		if br == nil || visited[br] {
			continue
		}
		visited[br] = true
		if br.Name == name {
			if rs := scope.recordSymbol(br); rs != nil {
				return rs
			}
			return &RecordSymbol{SymName: name, Record: br}
		}
		if rs := injectedBaseName(scope, br, name, visited); rs != nil {
			return rs
		}
	}
	return nil
}
