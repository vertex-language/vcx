package sema

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
func StaticMembers(res *Result) []*VarSymbol {
	var out []*VarSymbol
	if res.GlobalScope == nil {
		return nil
	}
	for _, syms := range res.GlobalScope.Symbols {
		for _, sym := range syms {
			rs, ok := sym.(*RecordSymbol)
			if !ok || rs.ClassScope == nil {
				continue
			}
			for _, members := range rs.ClassScope.Symbols {
				for _, m := range members {
					if v, isVar := m.(*VarSymbol); isVar && v.InClass != nil && v.Defined {
						out = append(out, v)
					}
				}
			}
		}
	}
	return out
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
