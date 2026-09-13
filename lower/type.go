package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/types"
)

// regType is the machine register a value of this type is held in.
//
// VIR has no i8 or i16: every integer narrower than a word is held in an
// i32, and its declared width matters only when it is stored, converted, or
// wrapped. That is the same choice every SSA IR makes, and it is why
// storeType exists separately -- a `char` is an i32 in a register and one
// byte in memory, and confusing the two writes three bytes of somebody
// else's object.
func (u *unit) regType(t types.Type) ir.RegType {
	if t == nil {
		return ir.TypeNone
	}
	switch t := types.Unqualify(t).(type) {
	case *types.Basic:
		switch t.K {
		case types.Void:
			return ir.TypeNone
		case types.Float:
			return ir.TypeF32
		case types.Double, types.LongDouble:
			// long double is double under the Microsoft ABI, and vcx does
			// not yet select the x87 or quad register the others use.
			return ir.TypeF64
		default:
			return u.intReg(types.Typ(t.K))
		}
	case *types.Enum:
		if t.Underlying != nil {
			return u.regType(t.Underlying)
		}
		return ir.TypeI32
	case *types.MemberPointer:
		// A data member pointer is an offset, four bytes under Microsoft
		// for a class without virtual or multiple bases; a function
		// member pointer is a function's address (see memptr.go).
		if size, ok := u.model.Sizeof(t); ok && size == 4 {
			return ir.TypeI32
		}
		if !types.IsFunc(t.Elem) {
			// Itanium's is a ptrdiff_t, whatever the class.
			return ir.TypeI64
		}
		return ir.TypePtr
	case *types.Pointer, *types.Array, *types.Func:
		// Arrays and functions decay to pointers in expression position.
		return ir.TypePtr
	case *types.LValueReference, *types.RValueReference:
		// A reference is an address at machine level.
		return ir.TypePtr
	case *types.Record:
		// A class is carried by address; where the copy happens is the
		// caller's business and the ABI's.
		return ir.TypePtr
	}
	return ir.TypeNone
}

// intReg picks i32 or i64 for an integer by its width in the *target's*
// model. `long` is not i64 by name -- it is four bytes on Windows and eight
// on Linux, which is the whole reason the model is asked.
func (u *unit) intReg(t types.Type) ir.RegType {
	if sz, ok := u.model.Sizeof(t); ok && sz > 4 {
		return ir.TypeI64
	}
	return ir.TypeI32
}

// isSigned reports whether an integer type's operations are the signed ones.
// It decides which division, which shift, and which comparison, so a wrong
// answer is a wrong result rather than a slow one.
func isSigned(t types.Type) bool {
	switch t := types.Unqualify(t).(type) {
	case *types.Basic:
		switch t.K {
		case types.UChar, types.UShort, types.UInt, types.ULong,
			types.ULongLong, types.Bool, types.Char8, types.Char16, types.Char32:
			return false
		}
		return true
	case *types.Enum:
		if t.Underlying != nil {
			return isSigned(t.Underlying)
		}
	}
	return true
}
