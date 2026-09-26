package ast

import (
	"reflect"
	"sync"
)

// Clone returns a deep copy of an AST subtree, creating fresh nodes for template instantiation.
//
// Every pointer, slice, map and interface is copied, and an unexported
// field is left zero. The walk is by reflection, but what it does to a
// type is worked out once: a type that holds no references is copied as
// a value, and a struct's reference-free fields are copied together
// rather than one reflective step each. Template instantiation clones a
// great deal, and walking every Tok and Span was most of its cost.
func Clone[N Node](n N) N {
	v := reflect.ValueOf(n)
	if !v.IsValid() {
		return n
	}
	return cloneValue(v).Interface().(N)
}

// A plan is how values of one type are cloned.
type plan struct {
	// flat types hold no references and are copied as values.
	flat bool
	// whole says a struct may be copied in one assignment before its
	// deep fields are replaced: it has no unexported field to leave zero.
	whole bool
	// deep are the fields that hold references, and settable the
	// exported fields, for a struct that cannot be copied whole.
	deep, settable []int
}

var plans sync.Map // reflect.Type -> *plan

func planFor(t reflect.Type) *plan {
	if p, ok := plans.Load(t); ok {
		return p.(*plan)
	}
	p := &plan{flat: isFlat(t, map[reflect.Type]bool{})}
	if !p.flat && t.Kind() == reflect.Struct {
		p.whole = true
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				p.whole = false
				continue
			}
			p.settable = append(p.settable, i)
			if !isFlat(f.Type, map[reflect.Type]bool{}) {
				p.deep = append(p.deep, i)
			}
		}
	}
	plans.Store(t, p)
	return p
}

// isFlat reports whether a type holds nothing a deep copy must follow.
func isFlat(t reflect.Type, seen map[reflect.Type]bool) bool {
	switch t.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map,
		reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return false
	case reflect.Array:
		return isFlat(t.Elem(), seen)
	case reflect.Struct:
		if seen[t] {
			return true
		}
		seen[t] = true
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			// An unexported field is zeroed, never copied, so a
			// struct with one is not a plain value copy.
			if !f.IsExported() || !isFlat(f.Type, seen) {
				return false
			}
		}
	}
	return true
}

func cloneValue(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return v
		}
		out := reflect.New(v.Elem().Type())
		out.Elem().Set(cloneValue(v.Elem()))
		return out

	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		inner := cloneValue(v.Elem())
		out := reflect.New(v.Type()).Elem()
		out.Set(inner)
		return out

	case reflect.Struct:
		p := planFor(v.Type())
		if p.flat {
			return v
		}
		out := reflect.New(v.Type()).Elem()
		if p.whole {
			out.Set(v)
			for _, i := range p.deep {
				out.Field(i).Set(cloneValue(v.Field(i)))
			}
			return out
		}
		for _, i := range p.settable {
			out.Field(i).Set(cloneValue(v.Field(i)))
		}
		return out

	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		if planFor(v.Type().Elem()).flat {
			reflect.Copy(out, v)
			return out
		}
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(cloneValue(v.Index(i)))
		}
		return out

	case reflect.Map:
		if v.IsNil() {
			return v
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		for _, k := range v.MapKeys() {
			out.SetMapIndex(k, cloneValue(v.MapIndex(k)))
		}
		return out
	}
	return v
}
