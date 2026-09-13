package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/types"
)

// convert converts a value between types in IR registers (arithmetic conversions,
// pointer conversions, and member pointer null constants).
func (fl *fn) convert(v ir.Value, from, to types.Type) ir.Value {
	if v == nil || to == nil {
		return v
	}
	want := fl.u.regType(to)
	if want == ir.TypeNone {
		return v
	}

	b := fl.blk
	switch val := v.(type) {
	case ir.I1:
		// A comparison's result arriving where a number is wanted.
		return fl.convert(b.I32.ZExtI1(val), types.Typ(types.Int), to)

	case ir.I32:
		switch want {
		case ir.TypePtr:
			// Null pointer constant from integer zero.
			return b.Ptr.Const()
		case ir.TypeI32:
			if _, isMP := types.Unqualify(to).(*types.MemberPointer); isMP && from != nil {
				if _, fromMP := types.Unqualify(from).(*types.MemberPointer); !fromMP {
					// Null member pointer constant: -1.
					return b.I32.Const(-1)
				}
			}
			return fl.narrow(val, to)
		case ir.TypeI64:
			if _, isMP := types.Unqualify(to).(*types.MemberPointer); isMP && from != nil {
				if _, fromMP := types.Unqualify(from).(*types.MemberPointer); !fromMP {
					// Null member pointer constant: -1.
					return b.I64.Const(-1)
				}
			}
			// Source signedness decides sign- vs zero-extension.
			if from != nil && !isSigned(from) {
				return b.I64.ZExtI32(val)
			}
			return b.I64.SExtI32(val)
		case ir.TypeF32:
			if from != nil && !isSigned(from) {
				return b.F32.UCvtI32(val)
			}
			return b.F32.SCvtI32(val)
		case ir.TypeF64:
			if from != nil && !isSigned(from) {
				return b.F64.UCvtI32(val)
			}
			return b.F64.SCvtI32(val)
		}

	case ir.I64:
		switch want {
		case ir.TypePtr:
			// Integer to pointer conversion (reinterpret_cast or null pointer from zero).
			return b.Ptr.FromI64(val)
		case ir.TypeI64:
			return val
		case ir.TypeI32:
			return fl.narrow(b.I32.WrapI64(val), to)
		case ir.TypeF32:
			if from != nil && !isSigned(from) {
				return b.F32.UCvtI64(val)
			}
			return b.F32.SCvtI64(val)
		case ir.TypeF64:
			if from != nil && !isSigned(from) {
				return b.F64.UCvtI64(val)
			}
			return b.F64.SCvtI64(val)
		}

	case ir.F64:
		switch want {
		case ir.TypeF64:
			return val
		case ir.TypeF32:
			return b.F32.FCvtF64(val)
		case ir.TypeI32:
			// The fractional part is discarded toward zero.
			return fl.narrow(b.I32.SCvtF64(val), to)
		case ir.TypeI64:
			return b.I64.SCvtF64(val)
		}

	case ir.F32:
		switch want {
		case ir.TypeF32:
			return val
		case ir.TypeF64:
			return b.F64.FCvtF32(val)
		case ir.TypeI32:
			return fl.narrow(b.I32.SCvtF32(val), to)
		case ir.TypeI64:
			return b.I64.SCvtF32(val)
		}

	case ir.Ptr:
		switch want {
		case ir.TypeI64:
			if _, isMP := types.Unqualify(to).(*types.MemberPointer); isMP {
				// nullptr to data member pointer: -1.
				return b.I64.Const(-1)
			}
			// Pointer to integer conversion.
			return b.I64.FromPtr(val)
		case ir.TypeI32:
			if _, isMP := types.Unqualify(to).(*types.MemberPointer); isMP {
				// nullptr to data member pointer: -1.
				return b.I32.Const(-1)
			}
			return fl.narrow(b.I32.WrapI64(b.I64.FromPtr(val)), to)
		case ir.TypeI1:
			return b.Ptr.Ne(val, b.Ptr.Const())
		}
		if want == ir.TypePtr {
			// Derived-to-base conversion: adjust pointer by base subobject offset.
			if off := fl.u.baseAdjust(from, to); off != 0 {
				return b.Ptr.Add(val, b.I64.Const(off))
			}
			return val
		}
	}

	return v
}

// baseAdjust is the offset a pointer or reference moves by when converted
// from a derived class to one of its bases, or zero when the conversion is
// between anything else.
func (u *unit) baseAdjust(from, to types.Type) int64 {
	fromRec := types.AsRecord(types.Unqualify(pointee(from)))
	toRec := types.AsRecord(types.Unqualify(pointee(to)))
	if fromRec == nil || toRec == nil || fromRec == toRec {
		return 0
	}
	if off, ok := u.model.BaseOffset(fromRec, toRec); ok {
		return off
	}
	// Base-to-derived static_cast: subtract base subobject offset.
	if off, ok := u.model.BaseOffset(toRec, fromRec); ok {
		return -off
	}
	return 0
}

// pointee is what a pointer or reference type refers to, or nil.
func pointee(t types.Type) types.Type {
	switch t := types.Unqualify(t).(type) {
	case *types.Pointer:
		return t.Elem
	case *types.LValueReference:
		return t.Elem
	case *types.RValueReference:
		return t.Elem
	}
	return nil
}

// narrow truncates an i32 to a type narrower than a word, applying explicit masking or sign-extension.
func (fl *fn) narrow(v ir.I32, to types.Type) ir.Value {
	bits := fl.storeBytes(to) * 8
	if bits >= 32 {
		return v
	}
	b := fl.blk
	shift := b.I32.Const(32 - bits)
	if isSigned(to) {
		return b.I32.SShr(b.I32.Shl(v, shift), shift)
	}
	mask := int64(1)<<uint(bits) - 1
	return b.I32.And(v, b.I32.Const(mask))
}
