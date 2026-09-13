package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/types"
)

// Bit-field lowering. A bit-field has no independent address; it is packed into
// bits of a storage unit. Member access produces the unit's address and records
// bit offsets, and load/store perform masking and shifting.

// bitField is a bit-field's place within its storage unit.
type bitField struct {
	unitBytes int64
	bitOff    int64 // from the least significant bit, as cl packs them
	width     int64
	signed    bool
}

// bitFieldNamed finds a bit-field member of rec, or of a base of rec, by
// name.
func (u *unit) bitFieldNamed(rec *types.Record, name string) (bitField, bool) {
	for i, f := range rec.Fields {
		if f.Name != name || !f.BitField {
			continue
		}
		_, unitBytes, bitOff, ok := u.model.BitField(rec, i)
		if !ok {
			return bitField{}, false
		}
		return bitField{unitBytes: unitBytes, bitOff: bitOff, width: f.Width, signed: isSigned(f.Type)}, true
	}
	for _, b := range rec.Bases {
		if br := classOf(b.Type); br != nil && !b.Virtual {
			if bf, ok := u.bitFieldNamed(br, name); ok {
				return bf, true
			}
		}
	}
	return bitField{}, false
}

// noteBitField records that the address unit is a bit-field's storage unit.
func (fl *fn) noteBitField(unit ir.Ptr, bf bitField) {
	if fl.bitFields == nil {
		fl.bitFields = map[ir.Ptr]bitField{}
	}
	fl.bitFields[unit] = bf
}

// bitFieldLoad reads a bit-field: the unit, shifted so the field's top
// bit is the register's, then down again -- arithmetically for a signed
// field, which is how -3 in four bits comes back as -3.
func (fl *fn) bitFieldLoad(unit ir.Ptr, bf bitField, t types.Type) ir.Value {
	b := fl.blk
	if bf.unitBytes == 8 {
		v := b.I64.Load(unit)
		up := 64 - bf.bitOff - bf.width
		v = b.I64.Shl(v, b.I64.Const(up))
		if bf.signed {
			return b.I64.SShr(v, b.I64.Const(64-bf.width))
		}
		return b.I64.UShr(v, b.I64.Const(64-bf.width))
	}
	var v ir.I32
	switch bf.unitBytes {
	case 1:
		v = b.I32.ULoad8(unit)
	case 2:
		v = b.I32.ULoad16(unit)
	default:
		v = b.I32.Load(unit)
	}
	up := 32 - bf.bitOff - bf.width
	v = b.I32.Shl(v, b.I32.Const(int64(up)))
	if bf.signed {
		v = b.I32.SShr(v, b.I32.Const(int64(32-bf.width)))
	} else {
		v = b.I32.UShr(v, b.I32.Const(int64(32-bf.width)))
	}
	if fl.u.regType(t) == ir.TypeI64 {
		if bf.signed {
			return b.I64.SExtI32(v)
		}
		return b.I64.ZExtI32(v)
	}
	return v
}

// bitFieldStore writes a bit-field: the unit's other bits kept, the
// value's low width bits put in place.
func (fl *fn) bitFieldStore(unit ir.Ptr, bf bitField, v ir.Value) {
	b := fl.blk
	if bf.unitBytes == 8 {
		var val ir.I64
		switch x := v.(type) {
		case ir.I64:
			val = x
		case ir.I32:
			val = b.I64.SExtI32(x)
		case ir.I1:
			val = b.I64.ZExtI32(b.I32.ZExtI1(x))
		default:
			return
		}
		mask := int64((uint64(1)<<uint64(bf.width) - 1) << uint64(bf.bitOff))
		old := b.I64.Load(unit)
		kept := b.I64.And(old, b.I64.Const(^mask))
		put := b.I64.And(b.I64.Shl(val, b.I64.Const(bf.bitOff)), b.I64.Const(mask))
		b.I64.Store(b.I64.Or(kept, put), unit)
		return
	}
	var val ir.I32
	switch x := v.(type) {
	case ir.I32:
		val = x
	case ir.I64:
		val = b.I32.WrapI64(x)
	case ir.I1:
		val = b.I32.ZExtI1(x)
	default:
		return
	}
	mask := int64((uint64(1)<<uint64(bf.width) - 1) << uint64(bf.bitOff))
	var old ir.I32
	switch bf.unitBytes {
	case 1:
		old = b.I32.ULoad8(unit)
	case 2:
		old = b.I32.ULoad16(unit)
	default:
		old = b.I32.Load(unit)
	}
	kept := b.I32.And(old, b.I32.Const(int64(uint32(^mask))))
	put := b.I32.And(b.I32.Shl(val, b.I32.Const(bf.bitOff)), b.I32.Const(int64(uint32(mask))))
	res := b.I32.Or(kept, put)
	switch bf.unitBytes {
	case 1:
		b.I32.Store8(res, unit)
	case 2:
		b.I32.Store16(res, unit)
	default:
		b.I32.Store(res, unit)
	}
}
