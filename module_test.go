package vcx

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vertex-language/vcx/preprocessor"
)

// A module's interface unit, one of its implementation units, and a unit
// that imports it: the implementation sees the interface's declarations,
// including what it does not export, the importer sees what it exports,
// and each definition is in one object only.
func TestModuleUnits(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("runs the program it builds: aarch64-macos only")
	}
	dir := t.TempDir()
	write := func(name, src string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	iface := write("math.cpp", `module;
#include <cstdint>
export module geo.math;

export enum class Code : int32_t { ok = 0, bad = -1 };
export int add(int a, int b) { return a + b; }
export namespace math { int mul(int a, int b); }
int hidden(int x) { return x * 2; }
export int counter = 7;
`)
	// Declarations in the implementation unit's global module fragment:
	// the `module geo.math;` after them is still a directive, and the
	// interface is still seen.
	impl := write("math_impl.cpp", `module;
extern "C" int abs(int);
namespace detail { inline int one() { return 1; } }
module geo.math;
namespace math { int mul(int a, int b) { return a * b + hidden(0) + (int)Code::ok + abs(detail::one()) - 1; } }
`)
	main := write("main.cpp", `import geo.math;
#include <cstdio>
int main() { std::printf("%d %d %d %d\n", add(2, 3), math::mul(4, 5), (int)Code::bad, counter); return 0; }
`)

	c := &Compiler{Modules: map[string]string{"geo.math": iface}}
	symbols := map[string][]string{
		iface: {"__ZW3geoW4math3addii", "__ZW3geoW4math6hiddeni", "__ZW3geoW4math7counter"},
		impl:  {"__ZN4mathW3geoW4math3mulEii"},
	}
	var objs []Input
	for _, src := range []string{iface, impl, main} {
		obj, diags, err := c.Object(File(src))
		if err != nil || HasErrors(diags) {
			t.Fatalf("%s: %v %v", filepath.Base(src), err, diags)
		}
		for _, sym := range symbols[src] {
			if !bytes.Contains(obj, []byte(sym)) {
				t.Errorf("%s: no symbol %s", filepath.Base(src), sym)
			}
		}
		objs = append(objs, ObjectBytes(filepath.Base(src)+".o", obj))
	}
	out := filepath.Join(dir, "prog")
	if err := c.Build(BuildParams{Output: out, Inputs: objs}); err != nil {
		t.Fatal(err)
	}
	got, err := exec.Command(out).Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "5 20 -1 7\n" {
		t.Fatalf("got %q", got)
	}
}

func TestModuleDirectives(t *testing.T) {
	c := &Compiler{}
	ds, diags, err := c.ModuleDirectives(Text("a.cpp", []byte("module;\n#include <cstdint>\nexport module net.tcp;\nimport vertex.task;\n")))
	if err != nil || HasErrors(diags) {
		t.Fatalf("%v %v", err, diags)
	}
	var kinds []preprocessor.ModuleKind
	var names []string
	for _, d := range ds {
		kinds = append(kinds, d.Kind)
		names = append(names, d.Name)
	}
	want := []preprocessor.ModuleKind{preprocessor.GlobalFragment, preprocessor.Interface, preprocessor.Import}
	if len(kinds) != 3 || kinds[0] != want[0] || kinds[1] != want[1] || kinds[2] != want[2] || names[1] != "net.tcp" || names[2] != "vertex.task" {
		t.Fatalf("got %v %v", kinds, names)
	}
}

func TestCAndObjectiveCAreNotCXX(t *testing.T) {
	c := &Compiler{}
	for _, name := range []string{"a.c", "a.m"} {
		if _, _, err := c.Object(Text(name, []byte("int x;"))); err == nil {
			t.Errorf("%s compiled; v++ is a C++ compiler", name)
		}
	}
}

// Two units that include <compare> link: libc++ defines its orderings'
// constants out of class, `inline constexpr weak_ordering
// weak_ordering::less(...)`, and an inline definition is every unit's, so
// each object's copy is COMDAT.
func TestInlineStaticMembersAcrossUnits(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("runs the program it builds: aarch64-macos only")
	}
	dir := t.TempDir()
	a := filepath.Join(dir, "a.cpp")
	b := filepath.Join(dir, "b.cpp")
	os.WriteFile(a, []byte("#include <compare>\nint a() { return (1 <=> 2) < 0 ? 1 : 0; }\n"), 0o644)
	os.WriteFile(b, []byte("#include <compare>\nint a();\nint main() { return a() - 1; }\n"), 0o644)
	out := filepath.Join(dir, "ab")
	c := &Compiler{}
	if err := c.Build(BuildParams{Output: out, Inputs: []Input{File(a), File(b)}}); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command(out).Run(); err != nil {
		t.Fatal(err)
	}
}

// [dcl.typedef]/9: `typedef struct { ... } Meta;` names the class Meta for
// linkage, so a C++ function taking one has the symbol clang gives it.
func TestTypedefNameForLinkage(t *testing.T) {
	c := &Compiler{Target: "aarch64-macos"}
	obj, diags, err := c.Object(Text("m.cpp", []byte("typedef struct { int kind; } Meta;\nint kindOf(const Meta* m) { return m->kind; }\n")))
	if err != nil || HasErrors(diags) {
		t.Fatalf("%v %v", err, diags)
	}
	if !bytes.Contains(obj, []byte("__Z6kindOfPK4Meta")) {
		t.Error("no symbol __Z6kindOfPK4Meta")
	}
}

func TestLinkPragmas(t *testing.T) {
	c := &Compiler{}
	sc, diags, err := c.Scan(Text("w.cpp", []byte("module;\n#pragma vertex framework(\"AppKit\")\n#pragma vertex library(\"m\")\n#pragma comment(lib, \"ws2_32\")\n#pragma vertex frobnicate(\"x\")\nexport module ui.window;\n")))
	if err != nil || HasErrors(diags) {
		t.Fatalf("%v %v", err, diags)
	}
	var got []string
	for _, l := range sc.Links {
		kind := "lib"
		if l.Kind == preprocessor.LinkFramework {
			kind = "framework"
		}
		got = append(got, kind+":"+l.Name)
	}
	if strings.Join(got, " ") != "framework:AppKit lib:m lib:ws2_32" {
		t.Errorf("links = %v", got)
	}
	if len(sc.Modules) != 2 || sc.Modules[1].Name != "ui.window" {
		t.Errorf("modules = %+v", sc.Modules)
	}
}

// An Apple framework's header is found in the SDK's frameworks, as clang
// finds it with -F: <CoreFoundation/CoreFoundation.h> is
// CoreFoundation.framework/Headers/CoreFoundation.h.
func TestFrameworkHeaders(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("reads the macOS SDK")
	}
	c := &Compiler{Target: "aarch64-macos"}
	_, diags, err := c.Object(Text("cf.cpp", []byte("#include <CoreFoundation/CoreFoundation.h>\ndouble now() { return CFAbsoluteTimeGetCurrent(); }\n")))
	if err != nil || HasErrors(diags) {
		t.Fatalf("%v %v", err, diags)
	}
}
