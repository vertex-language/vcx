package constexpr

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/vertex-language/vcx/types"
)

var (
	ErrUB                  = errors.New("undefined behavior during constant evaluation")
	ErrUninitializedMemory = errors.New("read of uninitialized memory in constant expression")
	ErrInactiveUnionMember = errors.New("read of inactive union member in constant expression")
	ErrArrayOutOfBounds    = errors.New("array access out of bounds in constant expression")
	ErrDanglingPointer     = errors.New("dereference of dangling pointer or dead object")
	ErrNullPointer         = errors.New("dereference of null pointer in constant expression")
	ErrConstMutation       = errors.New("modification of const object in constant expression")
	ErrNonConstexpr        = errors.New("expression is not a constant expression")
)

// Object represents a memory storage unit in the compile-time evaluator.
type Object struct {
	ID        int
	Name      string
	Type      types.Type
	Val       Value
	IsConst   bool
	IsAlive   bool
	IsDynamic bool // Allocated via constexpr new
}

// NewObject creates a new live object with the given type and value.
func NewObject(id int, name string, typ types.Type, val Value, isConst bool) *Object {
	return &Object{
		ID:      id,
		Name:    name,
		Type:    typ,
		Val:     val,
		IsConst: isConst,
		IsAlive: true,
	}
}

// Read reads the value at the specified subobject path and element offset.
func (obj *Object) Read(path []PathStep, offset int64) (Value, error) {
	if !obj.IsAlive {
		return nil, fmt.Errorf("%w: object %q is no longer alive", ErrDanglingPointer, obj.Name)
	}
	if obj.Val == nil {
		return nil, fmt.Errorf("%w: object %q has no value", ErrUninitializedMemory, obj.Name)
	}

	cur := obj.Val
	for _, step := range path {
		switch step.Kind {
		case PathIndex:
			// A string literal is an array of const char; the terminating null
			// is part of the array, so index len(s) is in bounds.
			if str, isStr := cur.(StringValue); isStr {
				if step.Index < 0 || step.Index > len(str.Val) {
					return nil, fmt.Errorf("%w: index %d out of bounds [0, %d)", ErrArrayOutOfBounds, step.Index, len(str.Val)+1)
				}
				var ch byte
				if step.Index < len(str.Val) {
					ch = str.Val[step.Index]
				}
				cur = IntValue{
					Val:      big.NewInt(int64(ch)),
					BitWidth: 8,
					IsSigned: true,
					Typ:      types.Typ(types.Char),
				}
				continue
			}
			arr, ok := cur.(ArrayValue)
			if !ok {
				return nil, fmt.Errorf("%w: index into non-array %T", ErrUB, cur)
			}
			if step.Index < 0 || step.Index >= len(arr.Elements) {
				return nil, fmt.Errorf("%w: index %d out of bounds [0, %d)", ErrArrayOutOfBounds, step.Index, len(arr.Elements))
			}
			cur = arr.Elements[step.Index]

		case PathField:
			rec, ok := cur.(RecordValue)
			if !ok {
				return nil, fmt.Errorf("%w: field access on non-record %T", ErrUB, cur)
			}
			if rec.RecordType != nil && rec.RecordType.Tag == types.TagUnion {
				if rec.ActiveUnionField != "" && rec.ActiveUnionField != step.Name {
					return nil, fmt.Errorf("%w: union field %q accessed while %q is active", ErrInactiveUnionMember, step.Name, rec.ActiveUnionField)
				}
			}
			fVal, exists := rec.Fields[step.Name]
			if !exists || fVal == nil {
				return nil, fmt.Errorf("%w: field %q uninitialized", ErrUninitializedMemory, step.Name)
			}
			cur = fVal

		case PathBase:
			rec, ok := cur.(RecordValue)
			if !ok {
				return nil, fmt.Errorf("%w: base class access on non-record %T", ErrUB, cur)
			}
			bVal, exists := rec.Bases[step.Name]
			if !exists || bVal == nil {
				return nil, fmt.Errorf("%w: base class %q not found or uninitialized", ErrUB, step.Name)
			}
			cur = bVal
		}
	}

	// Handle pointer element offset
	if offset != 0 {
		arr, ok := cur.(ArrayValue)
		if ok {
			if offset < 0 || int(offset) >= len(arr.Elements) {
				return nil, fmt.Errorf("%w: offset %d out of bounds [0, %d)", ErrArrayOutOfBounds, offset, len(arr.Elements))
			}
			return arr.Elements[offset], nil
		}
		str, ok := cur.(StringValue)
		if ok {
			if offset < 0 || int(offset) > len(str.Val) {
				return nil, fmt.Errorf("%w: string offset %d out of bounds [0, %d]", ErrArrayOutOfBounds, offset, len(str.Val))
			}
			if int(offset) == len(str.Val) {
				return NewInt(0, types.Typ(types.Char), types.LP64()), nil // null terminator
			}
			return NewInt(int64(str.Val[offset]), types.Typ(types.Char), types.LP64()), nil
		}
		return nil, fmt.Errorf("%w: non-zero offset %d on scalar value", ErrUB, offset)
	}

	if cur == nil {
		return nil, fmt.Errorf("%w: subobject at path uninitialized", ErrUninitializedMemory)
	}
	return cur, nil
}

// Write writes a value at the specified subobject path and offset.
func (obj *Object) Write(path []PathStep, offset int64, val Value) error {
	if !obj.IsAlive {
		return fmt.Errorf("%w: object %q is no longer alive", ErrDanglingPointer, obj.Name)
	}
	if obj.IsConst && len(path) == 0 && offset == 0 && obj.Val != nil {
		return fmt.Errorf("%w: cannot modify const object %q", ErrConstMutation, obj.Name)
	}

	if len(path) == 0 && offset == 0 {
		obj.Val = val
		return nil
	}

	newVal, err := updateSubobject(obj.Val, path, offset, val)
	if err != nil {
		return err
	}
	obj.Val = newVal
	return nil
}

func updateSubobject(cur Value, path []PathStep, offset int64, newVal Value) (Value, error) {
	if len(path) == 0 {
		if offset != 0 {
			arr, ok := cur.(ArrayValue)
			if ok {
				if offset < 0 || int(offset) >= len(arr.Elements) {
					return nil, fmt.Errorf("%w: offset %d out of bounds [0, %d)", ErrArrayOutOfBounds, offset, len(arr.Elements))
				}
				arr.Elements[offset] = newVal
				return arr, nil
			}
			return nil, fmt.Errorf("%w: non-zero offset on non-array", ErrUB)
		}
		return newVal, nil
	}

	step := path[0]
	rest := path[1:]

	switch step.Kind {
	case PathIndex:
		arr, ok := cur.(ArrayValue)
		if !ok {
			return nil, fmt.Errorf("%w: index step on non-array %T", ErrUB, cur)
		}
		if step.Index < 0 || step.Index >= len(arr.Elements) {
			return nil, fmt.Errorf("%w: index %d out of bounds [0, %d)", ErrArrayOutOfBounds, step.Index, len(arr.Elements))
		}
		updatedElem, err := updateSubobject(arr.Elements[step.Index], rest, offset, newVal)
		if err != nil {
			return nil, err
		}
		arr.Elements[step.Index] = updatedElem
		return arr, nil

	case PathField:
		rec, ok := cur.(RecordValue)
		if !ok {
			return nil, fmt.Errorf("%w: field step on non-record %T", ErrUB, cur)
		}
		if rec.RecordType != nil && rec.RecordType.Tag == types.TagUnion {
			rec.ActiveUnionField = step.Name
		}
		subVal := rec.Fields[step.Name]
		updatedField, err := updateSubobject(subVal, rest, offset, newVal)
		if err != nil {
			return nil, err
		}
		rec.Fields[step.Name] = updatedField
		return rec, nil

	case PathBase:
		rec, ok := cur.(RecordValue)
		if !ok {
			return nil, fmt.Errorf("%w: base step on non-record %T", ErrUB, cur)
		}
		subVal := rec.Bases[step.Name]
		updatedBase, err := updateSubobject(subVal, rest, offset, newVal)
		if err != nil {
			return nil, err
		}
		rec.Bases[step.Name] = updatedBase
		return rec, nil
	}

	return nil, fmt.Errorf("%w: invalid path step kind %d", ErrUB, step.Kind)
}
