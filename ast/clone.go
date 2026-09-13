package ast

import "reflect"

// Clone returns a deep copy of an AST subtree, creating fresh nodes for template instantiation.
func Clone[N Node](n N) N {
	v := reflect.ValueOf(n)
	if !v.IsValid() {
		return n
	}
	return cloneValue(v).Interface().(N)
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
		out := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			if !out.Field(i).CanSet() {
				continue
			}
			out.Field(i).Set(cloneValue(f))
		}
		return out

	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
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
