package lower

import (
	"fmt"
	"github.com/vertex-language/vcx/types"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/literal"
)

// stringLit lowers a string literal into a read-only global in static storage.
// Duplicate string literals in the translation unit share the global.
func (fl *fn) stringLit(e *ast.StringLit) ir.Value {
	g, ok := fl.u.stringGlobal(e)
	if !ok {
		return nil
	}
	return fl.blk.Ptr.GetAddr(g)
}

// stringGlobal is the global holding a literal's code units, made on
// first use: bytes for a narrow literal, and the target's wchar_t, or
// char16_t or char32_t, for the others.
func (u *unit) stringGlobal(e *ast.StringLit) (*ir.Global, bool) {
	s, err := literal.Decode(u.unit, e)
	if err != nil {
		u.errorf(e.Pos(), "%v", err)
		return nil, false
	}
	key := fmt.Sprintf("%d:%v", s.Enc, s.Units)
	if g, seen := u.strings[key]; seen {
		return g, true
	}
	u.nstrings++
	name := u.symbolName(fmt.Sprintf("__vcx_str_%d", u.nstrings))
	n := uint64(len(s.Units) + 1)
	var g *ir.Global
	width, _ := u.model.Sizeof(types.Typ(s.ElemKind()))
	switch width {
	case 1:
		g = u.mod.Global(name, ir.RO, ir.Array(n, ir.StoreI8.FType()))
		g.Init(ir.Str(string(s.Bytes()) + string(rune(0))))
	default:
		elem := ir.StoreI16.FType()
		if width == 4 {
			elem = ir.StoreI32.FType()
		}
		g = u.mod.Global(name, ir.RO, ir.Array(n, elem))
		units := make([]ir.Init, 0, n)
		for _, cu := range s.Units {
			units = append(units, ir.Lit(ir.Int(int64(cu))))
		}
		units = append(units, ir.Lit(ir.Int(0)))
		g.Init(ir.List(units...))
	}
	g.Internal()
	g.Align(uint64(width))
	u.strings[key] = g
	return g, true
}

// initArrayFromString initializes an array from a string literal, copying
// characters and null-terminating, zeroing any remaining capacity.
func (fl *fn) initArrayFromString(dst ir.Ptr, arr *types.Array, e *ast.StringLit) bool {
	g, ok := fl.u.stringGlobal(e)
	if !ok {
		return false
	}
	s, _ := literal.Decode(fl.u.unit, e)
	width, _ := fl.u.model.Sizeof(types.Unqualify(arr.Elem))
	have := int64(len(s.Units)+1) * width
	room := arr.Len * width
	copied := have
	if copied > room {
		copied = room
	}
	fl.blk.MemCpy(dst, fl.blk.Ptr.GetAddr(g), fl.blk.I64.Const(copied))
	if copied < room {
		rest := fl.blk.Ptr.Add(dst, fl.blk.I64.Const(copied))
		fl.blk.MemSet(rest, fl.blk.I32.Const(0), fl.blk.I64.Const(room-copied))
	}
	return true
}
