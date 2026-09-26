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
	if rec := classOf(t); rec != nil && fl.u.model.VaList == types.VaListPointer && fl.u.model.ABI == types.ItaniumAppleARM64 {
		return fl.vaArgAppleARM64(ap, rec)
	}
	fl.u.errorf(e.Pos(), "va_arg of %s is not lowered yet", t)
	return nil
}

// vaArgAppleARM64 reads a class argument from Apple's arm64 va_list, a
// pointer into the stack where every variadic argument has 8-byte slots
// of its own: a class of up to 16 bytes is there by value, in as many
// slots as it fills, and a larger one by the address of a copy the caller
// made. What is read is copied into a temporary, the va_arg's value.
func (fl *fn) vaArgAppleARM64(ap ir.Ptr, rec *types.Record) ir.Value {
	b := fl.blk
	size, _ := fl.u.sizeAlign(rec)
	cur := b.Ptr.Load(ap)
	src, step := cur, roundUp(size, 8)
	if size > 16 {
		src, step = b.Ptr.Load(cur), 8
	}
	b.Ptr.Store(b.Ptr.Add(cur, b.I64.Const(step)), ap)
	tmp := fl.alloc(rec, "va_arg")
	b.MemCpy(tmp, src, b.I64.Const(size))
	return tmp
}

// variadicClass is a class argument passed past a variadic function's
// parameters, on Apple's arm64: where every variadic argument has stack
// slots of its own, a class of up to 16 bytes is its bytes as 8-byte
// words, one slot each, and a larger one the address of a copy -- what
// vaArgAppleARM64 reads back. False on any other target.
func (fl *fn) variadicClass(addr ir.Ptr, rec *types.Record) ([]ir.Value, bool) {
	if fl.u.model.VaList != types.VaListPointer || fl.u.model.ABI != types.ItaniumAppleARM64 {
		return nil, false
	}
	b := fl.blk
	size, _ := fl.u.sizeAlign(rec)
	if size > 16 {
		tmp := fl.alloc(rec, "vararg")
		if !fl.copyObject(tmp, addr, rec, nil) {
			return nil, false
		}
		return []ir.Value{tmp}, true
	}
	// The words are read from a copy padded to whole slots, so the last
	// one reads no further than the class.
	words := roundUp(size, 8) / 8
	buf := fl.entry.Ptr.Alloc(uint64(words*8), 8)
	b.MemCpy(buf, addr, b.I64.Const(size))
	var out []ir.Value
	for i := int64(0); i < words; i++ {
		out = append(out, b.I64.Load(b.Ptr.Add(buf, b.I64.Const(i*8))))
	}
	return out, true
}
