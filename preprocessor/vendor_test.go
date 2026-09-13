package preprocessor

import (
	"testing"
	"testing/fstest"

	"github.com/vertex-language/vcx/token"
)

// The vendor operators answer from the Vendor they were given, and exist
// only when there is one.
func TestVendorOperators(t *testing.T) {
	fsys := fstest.MapFS{
		"a/v.h": &fstest.MapFile{Data: []byte("#if __has_include_next(<v.h>)\nnext\n#endif\n")},
		"b/v.h": &fstest.MapFile{Data: []byte("")},
	}
	sub := func(dir string) Mount {
		f, _ := fsys.Sub(dir)
		return Mount{Name: dir, FS: f}
	}
	vendor := &Vendor{
		Features:   map[string]bool{"cxx_rtti": true},
		Extensions: map[string]bool{"cxx_fixed_enum": true},
		Builtins:   map[string]bool{"__builtin_expect": true},
		Attributes: map[string]int{"always_inline": 1},
		Arch:       []string{"arm64", "aarch64"}, VendorName: []string{"apple"}, OS: []string{"macos", "darwin"},
	}
	for _, tc := range []struct{ src, want string }{
		{"#if __has_feature(cxx_rtti)\nyes\n#endif\n", "yes"},
		{"#if __has_feature(cxx_exceptions)\nyes\n#endif\n", ""},
		{"#if __has_extension(cxx_fixed_enum) && __has_extension(cxx_rtti)\nyes\n#endif\n", "yes"},
		{"#if __has_builtin(__builtin_expect) && !__has_builtin(__is_same)\nyes\n#endif\n", "yes"},
		{"#if __has_attribute(__always_inline__) && __has_attribute(always_inline)\nyes\n#endif\n", "yes"},
		{"#if __is_target_os(macos) && __is_target_arch(arm64) && !__is_target_os(ios)\nyes\n#endif\n", "yes"},
		{"#ifdef __has_builtin\nyes\n#endif\n", "yes"},
		{"#include <v.h>\n", "next"},
	} {
		p := New(Config{Vendor: vendor, Search: []Mount{sub("a"), sub("b")}})
		toks, diags := p.Run(token.NewFile("main.cpp", []byte(tc.src)))
		if got := norm(toks); got != tc.want {
			t.Errorf("%q = %q, want %q", tc.src, got, tc.want)
		}
		for _, d := range diags {
			if d.Severity == token.Error {
				t.Errorf("%q: unexpected %s", tc.src, d)
			}
		}
	}

	// Without a vendor they are not operators, and not defined.
	p := New(Config{})
	toks, _ := p.Run(token.NewFile("main.cpp", []byte("#ifdef __has_builtin\nyes\n#endif\n")))
	if got := norm(toks); got != "" {
		t.Errorf("__has_builtin is defined with no vendor: %q", got)
	}
}

// push_macro saves a definition and pop_macro restores it -- or its
// absence -- whatever happened in between.
func TestPushPopMacro(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"#define min 1\n#pragma push_macro(\"min\")\n#undef min\nmin\n#pragma pop_macro(\"min\")\nmin\n", "min 1"},
		{"#pragma push_macro(\"max\")\n#define max 2\nmax\n#pragma pop_macro(\"max\")\nmax\n", "2 max"},
		{"#define a x\n#pragma push_macro(\"a\")\n#define a y\n#pragma push_macro(\"a\")\n#undef a\n#pragma pop_macro(\"a\")\na\n#pragma pop_macro(\"a\")\na\n", "y x"},
	} {
		p := New(Config{})
		toks, _ := p.Run(token.NewFile("main.cpp", []byte(tc.src)))
		if got := norm(toks); got != tc.want {
			t.Errorf("%q = %q, want %q", tc.src, got, tc.want)
		}
	}
}
