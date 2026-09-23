package sema

import "sort"

import "github.com/vertex-language/vcx/types"

// HasExternalLinkage reports whether a namespace-scope variable has external linkage.
// Non-extern const non-volatile variables have internal linkage.
func HasExternalLinkage(v *VarSymbol) bool {
	if v.InClass != nil {
		return true
	}
	if v.Storage == StorageExtern {
		return true
	}
	if v.Storage == StorageStatic {
		return false
	}
	// An array inherits the cv-qualifiers of its element type.
	t := v.SymType
	for {
		arr, isArr := t.(*types.Array)
		if !isArr {
			break
		}
		t = arr.Elem
	}
	if q, isQ := t.(*types.Qualified); isQ && q.Q&types.QConst != 0 && q.Q&types.QVolatile == 0 {
		return false
	}
	return true
}

// StaticMembers returns every static data member defined in this unit.
//
// A class template's pattern holds no objects: its static members are
// declarations waiting for arguments, and it is the specializations that
// have members to emit. Those are not in any scope -- an instantiation
// keeps them in the template's own table -- so each template's instances
// are walked as well as the classes a scope names directly.
func StaticMembers(res *Result) []*VarSymbol {
	if res.GlobalScope == nil {
		return nil
	}
	var out []*VarSymbol
	seen := map[*VarSymbol]bool{}
	scopes := map[*Scope]bool{}
	var walk func(scope *Scope)

	take := func(rs *RecordSymbol) {
		if rs == nil || rs.ClassScope == nil {
			return
		}
		for _, member := range sortedNames(rs.ClassScope.Symbols) {
			for _, m := range rs.ClassScope.Symbols[member] {
				v, isVar := m.(*VarSymbol)
				if !isVar || v.InClass == nil || !v.Defined || seen[v] {
					continue
				}
				seen[v] = true
				out = append(out, v)
			}
		}
	}

	walk = func(scope *Scope) {
		if scope == nil || scopes[scope] {
			return
		}
		scopes[scope] = true
		// In name order: a scope is a map, and a module built from the
		// same source should come out the same.
		for _, name := range sortedNames(scope.Symbols) {
			for _, sym := range scope.Symbols[name] {
				switch s := sym.(type) {
				case *NamespaceSymbol:
					// `std::__1::numeric_limits<int>` is as much a class
					// as one at the top.
					walk(s.InnerScope)
				case *RecordSymbol:
					if s.ClassTemplate != nil {
						for _, key := range sortedInstanceKeys(s.ClassTemplate.Instances) {
							take(s.ClassTemplate.Instances[key])
						}
						if s.Record == nil || s.Record.TemplateArgs == nil {
							continue
						}
					}
					take(s)
				}
			}
		}
	}
	walk(res.GlobalScope)
	return out
}

// sortedInstanceKeys orders a template's instances, so that the same
// source is the same module every time.
func sortedInstanceKeys(m map[string]*RecordSymbol) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ConstructedClasses returns every class instantiated in this unit whose
// virtual function tables need to be emitted.
func ConstructedClasses(res *Result) []*types.Record {
	seen := map[*types.Record]bool{}
	var out []*types.Record
	add := func(r *types.Record) {
		if r == nil || seen[r] || !r.Complete {
			return
		}
		seen[r] = true
		out = append(out, r)
		// Constructing a derived object constructs its bases, and each
		// base's constructor installs that base's own table.
		for _, b := range r.Bases {
			if br := types.AsRecord(types.Unqualify(b.Type)); br != nil && !seen[br] {
				seen[br] = true
				out = append(out, br)
			}
		}
	}
	for _, fn := range res.Functions {
		if fn.Body != nil && fn.InClass != nil && fn.SymName == fn.InClass.Name {
			add(fn.InClass)
		}
	}
	if res.Info != nil {
		for _, sym := range res.Info.Defs {
			if v, isVar := sym.(*VarSymbol); isVar && !v.IsParam {
				add(types.AsRecord(types.Unqualify(v.SymType)))
			}
		}
	}
	return out
}

// MethodOf finds the member function corresponding to a virtual table slot.
func MethodOf(res *Result, definer *types.Record, slot types.VSlot) *FuncSymbol {
	if definer == nil {
		return nil
	}
	name := slot.Name
	if name == "{dtor}" {
		name = "~" + definer.Name
	}
	for _, fn := range res.Functions {
		if fn.InClass == definer && fn.SymName == name && types.SameSignature(fn.FuncType, slot.Signature) {
			return fn
		}
	}
	return nil
}

// sortedNames is a scope's names in order.
func sortedNames(symbols map[string][]Symbol) []string {
	names := make([]string, 0, len(symbols))
	for name := range symbols {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
