package vcx

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/parser"
)

// TestCloneIsDeep clones every top-level declaration of the corpus, which
// with its headers is most of libc++, and holds each copy to two things
// instantiation relies on: it equals the original, and it shares no node
// with it, so rewriting the copy cannot touch the template.
func TestCloneIsDeep(t *testing.T) {
	files, _ := filepath.Glob("tests/*.cpp")
	if len(files) == 0 {
		t.Skip("no corpus")
	}
	if testing.Short() {
		files = files[:20]
	}
	c := &Compiler{Std: Cxx23}
	for _, f := range files {
		file, _, err := c.Parse(File(f), parser.DefaultMode)
		if err != nil || file == nil {
			continue
		}
		for _, d := range file.Decls {
			cp := ast.Clone(d)
			if !reflect.DeepEqual(cp, d) {
				t.Fatalf("%s: a clone of %T differs from it", f, d)
			}
			if p, shared := sharedPointer(reflect.ValueOf(d), reflect.ValueOf(cp)); shared {
				t.Fatalf("%s: a clone of %T shares %s with it", f, d, p)
			}
		}
		file.Release()
	}
}

// sharedPointer walks two values of the same shape in step and reports
// the first pointer, slice or map they have in common.
func sharedPointer(a, b reflect.Value) (string, bool) {
	switch a.Kind() {
	case reflect.Ptr:
		if a.IsNil() {
			return "", false
		}
		if a.Pointer() == b.Pointer() {
			return a.Type().String(), true
		}
		return sharedPointer(a.Elem(), b.Elem())
	case reflect.Interface:
		if a.IsNil() {
			return "", false
		}
		return sharedPointer(a.Elem(), b.Elem())
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			if !a.Type().Field(i).IsExported() {
				continue
			}
			if p, ok := sharedPointer(a.Field(i), b.Field(i)); ok {
				return p, true
			}
		}
	case reflect.Slice:
		if a.Len() > 0 && a.Pointer() == b.Pointer() {
			return a.Type().String(), true
		}
		for i := 0; i < a.Len(); i++ {
			if p, ok := sharedPointer(a.Index(i), b.Index(i)); ok {
				return p, true
			}
		}
	case reflect.Map:
		if !a.IsNil() && a.Pointer() == b.Pointer() {
			return a.Type().String(), true
		}
	}
	return "", false
}
