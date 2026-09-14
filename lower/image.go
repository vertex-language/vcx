package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/literal"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// Constant initialization of an aggregate with static storage duration:
// a braced initializer whose every leaf is a constant forms the object's image.

// constantInit is the initializer for a global of type t from its braced
// list, or false when something in it has to run.
func (u *unit) constantInit(t types.Type, init ast.Node) (ir.Init, bool) {
	t = types.Unqualify(t)
	// A list whose braces were elided is laid out as the one with them.
	if list, isList := init.(*ast.InitList); isList {
		if braced := u.res.Info.Braced[list]; braced != nil {
			init = braced
		}
	}
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
		if lit, isLit := unparen(e).(*ast.BasicLit); isLit && lit.Kind == token.NULLPTR {
			return ir.ZeroInit, true
		}
		// A string literal is an array with static storage duration, so a
		// pointer to its first character is an address constant.
		if lit, isLit := unparen(e).(*ast.StringLit); isLit {
			if g, ok := u.stringGlobal(lit); ok {
				return ir.RelocInit(g), true
			}
		}
		if n, err := u.evalInt(e); err == nil && n == 0 {
			return ir.ZeroInit, true
		}
		// [expr.const]/13 -- the address of an object with static storage
		// duration or of a function is a constant the linker fills in, so
		// it is data rather than code run before main.
		if addr, ok := u.addressConstant(e); ok {
			return addr, true
		}
	}
	return ir.Init{}, false
}

// addressConstant is a pointer-valued constant expression as a relocation:
// `&global`, `&global.member`, `&array[2]`, a function's name, or an
// array's name decaying to its first element.
func (u *unit) addressConstant(e ast.Expr) (ir.Init, bool) {
	e = unparen(e)
	switch x := e.(type) {
	case *ast.UnaryExpr:
		if x.Op == token.AND {
			return u.addressOf(unparen(x.X))
		}
	case *ast.Ident, *ast.QualifiedName:
		switch sym := u.res.Info.Uses[x].(type) {
		case *sema.FuncSymbol:
			return u.addressOf(x)
		case *sema.VarSymbol:
			if _, isArr := types.Unqualify(sym.SymType).(*types.Array); isArr {
				return u.addressOf(x)
			}
		}
	}
	return ir.Init{}, false
}

// addressOf is where a static object or function is, as a relocation.
func (u *unit) addressOf(e ast.Expr) (ir.Init, bool) {
	switch x := e.(type) {
	case *ast.ParenExpr:
		return u.addressOf(x.X)
	case *ast.Ident, *ast.QualifiedName:
		switch sym := u.res.Info.Uses[x].(type) {
		case *sema.FuncSymbol:
			callee, isSym := u.callee(sym).(ir.Symbol)
			if !isSym {
				return ir.Init{}, false
			}
			return ir.RelocInit(callee), true
		case *sema.VarSymbol:
			if isReference(sym.SymType) {
				return ir.Init{}, false
			}
			g, known := u.globalFor(sym)
			if !known {
				return ir.Init{}, false
			}
			return ir.RelocInit(g), true
		}
	case *ast.MemberExpr:
		if x.Op != token.PERIOD {
			return ir.Init{}, false
		}
		base, ok := u.addressOf(unparen(x.X))
		if !ok {
			return ir.Init{}, false
		}
		rec := classOf(types.RemoveReference(u.res.Info.Types[x.X]))
		if rec == nil {
			return ir.Init{}, false
		}
		off, _, ok := u.fieldOffset(rec, sema.NameString(x.Sel, u.unit))
		if !ok {
			return ir.Init{}, false
		}
		return base.Plus(ir.Int(off)), true
	case *ast.IndexExpr:
		base, ok := u.addressOf(unparen(x.X))
		if !ok {
			return ir.Init{}, false
		}
		arr, isArr := types.Unqualify(types.RemoveReference(u.res.Info.Types[x.X])).(*types.Array)
		if !isArr {
			return ir.Init{}, false
		}
		if len(x.Args) != 1 {
			return ir.Init{}, false
		}
		i, err := u.evalInt(x.Args[0])
		if err != nil {
			return ir.Init{}, false
		}
		size, _ := u.sizeAlign(arr.Elem)
		return base.Plus(ir.Int(i * size)), true
	}
	return ir.Init{}, false
}
