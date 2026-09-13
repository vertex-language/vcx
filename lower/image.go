package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/literal"
	"github.com/vertex-language/vcx/types"
)

// Constant initialization of an aggregate with static storage duration:
// a braced initializer whose every leaf is a constant forms the object's image.

// constantInit is the initializer for a global of type t from its braced
// list, or false when something in it has to run.
func (u *unit) constantInit(t types.Type, init ast.Node) (ir.Init, bool) {
	t = types.Unqualify(t)
	switch x := t.(type) {
	case *types.Array:
		if x.Incomplete {
			return ir.Init{}, false
		}
		elems := make([]ir.Init, x.Len)
		for i := range elems {
			elems[i] = ir.ZeroInit
		}
		if lit, isLit := init.(*ast.StringLit); isLit {
			// Character array from a string literal.
			s, err := literal.Decode(u.unit, lit)
			if err != nil || int64(len(s.Units)) > x.Len {
				return ir.Init{}, false
			}
			for i, cu := range s.Units {
				elems[i] = ir.Lit(ir.Int(int64(cu)))
			}
			return ir.List(elems...), true
		}
		list, isList := init.(*ast.InitList)
		if !isList || int64(len(list.Items)) > x.Len {
			return ir.Init{}, false
		}
		for i, item := range list.Items {
			e, ok := u.constantInit(x.Elem, item)
			if !ok {
				return ir.Init{}, false
			}
			elems[i] = e
		}
		return ir.List(elems...), true

	case *types.Record:
		list, isList := init.(*ast.InitList)
		if !isList {
			return ir.Init{}, false
		}
		for _, m := range x.Methods {
			if m.Name == x.Name && !m.Defaulted {
				return ir.Init{}, false // a constructor runs
			}
		}
		if u.model.VTables(x) != nil {
			return ir.Init{}, false // a table pointer to store
		}
		var vals []ir.FieldVal
		items := list.Items
		for _, b := range x.Bases {
			if b.Virtual {
				return ir.Init{}, false
			}
			if len(items) == 0 {
				break
			}
			br := classOf(b.Type)
			if br == nil {
				return ir.Init{}, false
			}
			e, ok := u.constantInit(b.Type, items[0])
			if !ok {
				return ir.Init{}, false
			}
			vals = append(vals, ir.Val("base_"+identOf(br.Name), e))
			items = items[1:]
		}
		for i, f := range x.Fields {
			if f.BitField || f.Name == "" {
				return ir.Init{}, false
			}
			if i < len(items) {
				e, ok := u.constantInit(f.Type, items[i])
				if !ok {
					return ir.Init{}, false
				}
				vals = append(vals, ir.Val(identOf(f.Name), e))
				continue
			}
			if f.HasInit {
				return ir.Init{}, false // a default member initializer to run
			}
		}
		if x.Tag == types.TagUnion && len(vals) > 1 {
			return ir.Init{}, false
		}
		return ir.Fields(vals...), true
	}

	// A scalar, possibly written in braces.
	if list, isList := init.(*ast.InitList); isList {
		switch len(list.Items) {
		case 0:
			return ir.ZeroInit, true
		case 1:
			return u.constantInit(t, list.Items[0])
		}
		return ir.Init{}, false
	}
	e, isExpr := init.(ast.Expr)
	if !isExpr {
		return ir.Init{}, false
	}
	switch {
	case types.IsFloat(t):
		f, err := u.evalFloat(e)
		if err != nil {
			return ir.Init{}, false
		}
		return ir.Lit(ir.Float(f)), true
	case types.IsInteger(t) || types.IsEnum(t) || types.IsBool(t):
		n, err := u.evalInt(e)
		if err != nil {
			return ir.Init{}, false
		}
		return ir.Lit(ir.Int(n)), true
	case types.IsPointer(t):
		// A null pointer constant is the one address that is a number.
		if n, err := u.evalInt(e); err == nil && n == 0 {
			return ir.ZeroInit, true
		}
	}
	return ir.Init{}, false
}
