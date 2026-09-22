package lower

// GNU's variadic builtins, which <stdarg.h> spells va_start, va_arg,
// va_end and va_copy with. VIR's verbs take the list's address: where the
// calling convention makes va_list an array -- SysV x86-64's
// __va_list_tag[1] -- the expression decays to it; where it is a pointer
// -- Apple's arm64, Windows -- it is the variable's own address.

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/types"
)

// vaList is the address of the va_list an argument names.
func (fl *fn) vaList(e ast.Expr) (ir.Ptr, bool) {
	t := types.Unqualify(types.RemoveReference(fl.typeOf(e)))
	if _, isArr := t.(*types.Array); isArr {
		p, ok := fl.expr(e).(ir.Ptr)
		return p, ok
	}
	if _, isPtr := t.(*types.Pointer); isPtr && vaListIsArray(fl) {
		// Where va_list is an array, one of pointer type is a parameter,
		// adjusted from the array to a pointer to it ([dcl.fct]/5).
		p, ok := fl.expr(e).(ir.Ptr)
		return p, ok
	}
	p, _, ok := fl.lvalue(e)
	return p, ok
}

// vaListIsArray reports whether the calling convention's va_list is an
// array type.
func vaListIsArray(fl *fn) bool {
	return fl.u.model.VaList == types.VaListX86_64 || fl.u.model.VaList == types.VaListAArch64
}

// vaArg is __builtin_va_arg(ap, T): the next argument, read as a T.
func (fl *fn) vaArg(e *ast.VaArgExpr) ir.Value {
	ap, ok := fl.vaList(e.X)
	if !ok {
		return nil
	}
	t := fl.typeOf(e)
	b := fl.blk
	switch fl.u.regType(t) {
	case ir.TypeI32:
		return b.I32.VaArg(ap)
	case ir.TypeI64:
		return b.I64.VaArg(ap)
	case ir.TypeF64:
		return b.F64.VaArg(ap)
	case ir.TypePtr:
		if classOf(t) == nil {
			return b.Ptr.VaArg(ap)
		}
	}
	fl.u.errorf(e.Pos(), "va_arg of %s is not lowered yet", t)
	return nil
}
