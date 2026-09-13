package constexpr

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/vertex-language/vcx/types"
)

// ValueKind represents the kind of compile-time constant value.
type ValueKind int

const (
	ValVoid ValueKind = iota
	ValInt
	ValFloat
	ValBool
	ValNullptr
	ValPointer
	ValReference
	ValArray
	ValRecord
	ValString
)

func (k ValueKind) String() string {
	switch k {
	case ValVoid:
		return "void"
	case ValInt:
		return "int"
	case ValFloat:
		return "float"
	case ValBool:
		return "bool"
	case ValNullptr:
		return "nullptr"
	case ValPointer:
		return "pointer"
	case ValReference:
		return "reference"
	case ValArray:
		return "array"
	case ValRecord:
		return "record"
	case ValString:
		return "string"
	default:
		return "unknown"
	}
}

// Value is the interface implemented by all compile-time values.
type Value interface {
	Kind() ValueKind
	Type() types.Type
	String() string
	ToBool() bool
	Equal(other Value) bool
}

// VoidValue represents a void expression result.
type VoidValue struct{}

func (VoidValue) Kind() ValueKind        { return ValVoid }
func (VoidValue) Type() types.Type       { return types.Typ(types.Void) }
func (VoidValue) String() string         { return "void" }
func (VoidValue) ToBool() bool           { return false }
func (VoidValue) Equal(other Value) bool { _, ok := other.(VoidValue); return ok }

// IntValue represents an integral value with arbitrary precision and bit-width.
type IntValue struct {
	Val      *big.Int
	BitWidth int
	IsSigned bool
	Typ      types.Type
}

// NewInt creates an integral value normalized to its bit-width.
func NewInt(v int64, typ types.Type, model types.Model) IntValue {
	bw := 32
	signed := true
	if typ != nil {
		sz, ok := model.Sizeof(typ)
		if ok && sz > 0 {
			bw = int(sz * 8)
		}
		signed = types.IsSigned(typ)
	}
	bi := big.NewInt(v)
	iv := IntValue{
		Val:      bi,
		BitWidth: bw,
		IsSigned: signed,
		Typ:      typ,
	}
	iv.normalize()
	return iv
}

// NewBigInt creates an integral value from a *big.Int.
func NewBigInt(bi *big.Int, typ types.Type, model types.Model) IntValue {
	bw := 32
	signed := true
	if typ != nil {
		sz, ok := model.Sizeof(typ)
		if ok && sz > 0 {
			bw = int(sz * 8)
		}
		signed = types.IsSigned(typ)
	}
	iv := IntValue{
		Val:      new(big.Int).Set(bi),
		BitWidth: bw,
		IsSigned: signed,
		Typ:      typ,
	}
	iv.normalize()
	return iv
}

func (i *IntValue) normalize() {
	if i.BitWidth <= 0 {
		return
	}
	// Mask to bit-width
	mask := new(big.Int).Lsh(big.NewInt(1), uint(i.BitWidth))
	mask.Sub(mask, big.NewInt(1))
	i.Val.And(i.Val, mask)

	// If signed and top bit is set, sign-extend
	if i.IsSigned {
		topBit := new(big.Int).Lsh(big.NewInt(1), uint(i.BitWidth-1))
		if new(big.Int).And(i.Val, topBit).Sign() != 0 {
			// Negative number in two's complement: val = val - 2^BitWidth
			mod := new(big.Int).Lsh(big.NewInt(1), uint(i.BitWidth))
			i.Val.Sub(i.Val, mod)
		}
	}
}

func (i IntValue) Kind() ValueKind { return ValInt }
func (i IntValue) Type() types.Type {
	if i.Typ != nil {
		return i.Typ
	}
	if i.IsSigned {
		return types.Typ(types.Int)
	}
	return types.Typ(types.UInt)
}
func (i IntValue) String() string {
	if i.Val == nil {
		return "0"
	}
	return i.Val.String()
}
func (i IntValue) ToBool() bool {
	return i.Val != nil && i.Val.Sign() != 0
}
func (i IntValue) Int64() int64 {
	if i.Val == nil {
		return 0
	}
	return i.Val.Int64()
}
func (i IntValue) Uint64() uint64 {
	if i.Val == nil {
		return 0
	}
	return i.Val.Uint64()
}
func (i IntValue) Equal(other Value) bool {
	if ov, ok := other.(IntValue); ok {
		return i.Val.Cmp(ov.Val) == 0
	}
	if bv, ok := other.(BoolValue); ok {
		return i.ToBool() == bv.Val
	}
	return false
}

// FloatValue represents a compile-time floating-point value.
type FloatValue struct {
	Val float64
	Typ types.Type
}

func NewFloat(v float64, typ types.Type) FloatValue {
	return FloatValue{Val: v, Typ: typ}
}

func (f FloatValue) Kind() ValueKind { return ValFloat }
func (f FloatValue) Type() types.Type {
	if f.Typ != nil {
		return f.Typ
	}
	return types.Typ(types.Double)
}
func (f FloatValue) String() string {
	return fmt.Sprintf("%g", f.Val)
}
func (f FloatValue) ToBool() bool {
	return f.Val != 0.0
}
func (f FloatValue) Equal(other Value) bool {
	if ov, ok := other.(FloatValue); ok {
		return f.Val == ov.Val
	}
	if iv, ok := other.(IntValue); ok {
		return f.Val == float64(iv.Int64())
	}
	return false
}

// BoolValue represents a boolean value.
type BoolValue struct {
	Val bool
}

func NewBool(v bool) BoolValue { return BoolValue{Val: v} }

func (b BoolValue) Kind() ValueKind  { return ValBool }
func (b BoolValue) Type() types.Type { return types.Typ(types.Bool) }
func (b BoolValue) String() string   { return fmt.Sprintf("%t", b.Val) }
func (b BoolValue) ToBool() bool     { return b.Val }
func (b BoolValue) Equal(other Value) bool {
	if bv, ok := other.(BoolValue); ok {
		return b.Val == bv.Val
	}
	if iv, ok := other.(IntValue); ok {
		return b.Val == iv.ToBool()
	}
	return false
}

// NullptrValue represents std::nullptr_t.
type NullptrValue struct{}

func (NullptrValue) Kind() ValueKind  { return ValNullptr }
func (NullptrValue) Type() types.Type { return types.Typ(types.NullptrKind) }
func (NullptrValue) String() string   { return "nullptr" }
func (NullptrValue) ToBool() bool     { return false }
func (NullptrValue) Equal(other Value) bool {
	if _, ok := other.(NullptrValue); ok {
		return true
	}
	if pv, ok := other.(PointerValue); ok {
		return pv.IsNull()
	}
	return false
}

// PathStep represents a subobject navigation step.
type PathStepKind int

const (
	PathIndex PathStepKind = iota
	PathField
	PathBase
)

type PathStep struct {
	Kind  PathStepKind
	Index int
	Name  string
}

// PointerValue represents a pointer to an Object, a function, or null.
type PointerValue struct {
	Target *Object
	Offset int64 // element or byte offset
	Path   []PathStep
	Typ    types.Type
}

func (p PointerValue) Kind() ValueKind { return ValPointer }
func (p PointerValue) Type() types.Type {
	if p.Typ != nil {
		return p.Typ
	}
	return &types.Pointer{Elem: types.Typ(types.Void)}
}
func (p PointerValue) IsNull() bool {
	return p.Target == nil
}
func (p PointerValue) String() string {
	if p.IsNull() {
		return "nullptr"
	}
	if p.Offset != 0 {
		return fmt.Sprintf("&%s+%d", p.Target.Name, p.Offset)
	}
	return fmt.Sprintf("&%s", p.Target.Name)
}
func (p PointerValue) ToBool() bool {
	return !p.IsNull()
}
func (p PointerValue) Equal(other Value) bool {
	if ov, ok := other.(PointerValue); ok {
		if p.IsNull() && ov.IsNull() {
			return true
		}
		if p.Target != ov.Target || p.Offset != ov.Offset || len(p.Path) != len(ov.Path) {
			return false
		}
		for i := range p.Path {
			if p.Path[i] != ov.Path[i] {
				return false
			}
		}
		return true
	}
	if _, ok := other.(NullptrValue); ok {
		return p.IsNull()
	}
	return false
}

// ReferenceValue represents an lvalue reference to an Object.
type ReferenceValue struct {
	Target *Object
	Path   []PathStep
	Typ    types.Type
}

func (r ReferenceValue) Kind() ValueKind { return ValReference }
func (r ReferenceValue) Type() types.Type {
	if r.Typ != nil {
		return r.Typ
	}
	return &types.LValueReference{Elem: types.Typ(types.Void)}
}
func (r ReferenceValue) String() string {
	if r.Target == nil {
		return "<dangling ref>"
	}
	return fmt.Sprintf("ref(%s)", r.Target.Name)
}
func (r ReferenceValue) ToBool() bool {
	return r.Target != nil
}
func (r ReferenceValue) Equal(other Value) bool {
	if ov, ok := other.(ReferenceValue); ok {
		return r.Target == ov.Target
	}
	return false
}

// ArrayValue represents a compile-time fixed-size array of values.
type ArrayValue struct {
	Elements []Value
	ElemType types.Type
	Typ      types.Type
}

func NewArray(elems []Value, elemType types.Type) ArrayValue {
	arrTyp := &types.Array{Elem: elemType, Len: int64(len(elems))}
	return ArrayValue{
		Elements: elems,
		ElemType: elemType,
		Typ:      arrTyp,
	}
}

func (a ArrayValue) Kind() ValueKind { return ValArray }
func (a ArrayValue) Type() types.Type {
	if a.Typ != nil {
		return a.Typ
	}
	return &types.Array{Elem: a.ElemType, Len: int64(len(a.Elements))}
}
func (a ArrayValue) String() string {
	var sb strings.Builder
	sb.WriteString("{")
	for i, el := range a.Elements {
		if i > 0 {
			sb.WriteString(", ")
		}
		if el != nil {
			sb.WriteString(el.String())
		} else {
			sb.WriteString("<uninit>")
		}
	}
	sb.WriteString("}")
	return sb.String()
}
func (a ArrayValue) ToBool() bool { return true }
func (a ArrayValue) Equal(other Value) bool {
	ov, ok := other.(ArrayValue)
	if !ok || len(a.Elements) != len(ov.Elements) {
		return false
	}
	for i := range a.Elements {
		if a.Elements[i] == nil || ov.Elements[i] == nil {
			if a.Elements[i] != ov.Elements[i] {
				return false
			}
		} else if !a.Elements[i].Equal(ov.Elements[i]) {
			return false
		}
	}
	return true
}

// RecordValue represents a struct/class/union instance.
type RecordValue struct {
	RecordType       *types.Record
	Fields           map[string]Value
	Bases            map[string]Value
	ActiveUnionField string // For unions: tracks the active initialized field
}

func NewRecord(rt *types.Record) RecordValue {
	return RecordValue{
		RecordType: rt,
		Fields:     make(map[string]Value),
		Bases:      make(map[string]Value),
	}
}

func (r RecordValue) Kind() ValueKind { return ValRecord }
func (r RecordValue) Type() types.Type {
	if r.RecordType != nil {
		return r.RecordType
	}
	return &types.Record{Name: "anon", Tag: types.TagStruct}
}
func (r RecordValue) String() string {
	var sb strings.Builder
	name := "struct"
	if r.RecordType != nil {
		name = r.RecordType.Name
	}
	sb.WriteString(name)
	sb.WriteString("{")
	first := true
	for f, val := range r.Fields {
		if !first {
			sb.WriteString(", .")
		} else {
			sb.WriteString(".")
			first = false
		}
		sb.WriteString(f)
		sb.WriteString(" = ")
		if val != nil {
			sb.WriteString(val.String())
		} else {
			sb.WriteString("<uninit>")
		}
	}
	sb.WriteString("}")
	return sb.String()
}
func (r RecordValue) ToBool() bool { return true }
func (r RecordValue) Equal(other Value) bool {
	ov, ok := other.(RecordValue)
	if !ok || len(r.Fields) != len(ov.Fields) {
		return false
	}
	for k, v := range r.Fields {
		ovVal, exists := ov.Fields[k]
		if !exists {
			return false
		}
		if v == nil || ovVal == nil {
			if v != ovVal {
				return false
			}
		} else if !v.Equal(ovVal) {
			return false
		}
	}
	return true
}

// StringValue represents a string literal.
type StringValue struct {
	Val string
	Typ types.Type
}

func NewString(s string) StringValue {
	charType := types.Typ(types.Char)
	arrType := &types.Array{Elem: types.Qualify(charType, types.QConst), Len: int64(len(s) + 1)}
	return StringValue{Val: s, Typ: arrType}
}

func (s StringValue) Kind() ValueKind  { return ValString }
func (s StringValue) Type() types.Type { return s.Typ }
func (s StringValue) String() string   { return fmt.Sprintf("%q", s.Val) }
func (s StringValue) ToBool() bool     { return true }
func (s StringValue) Equal(other Value) bool {
	if ov, ok := other.(StringValue); ok {
		return s.Val == ov.Val
	}
	return false
}
