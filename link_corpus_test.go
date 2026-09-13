package vcx_test

// Test interoperability by linking objects built by vcx with objects built by the platform compiler.

import (
	"path/filepath"
	"testing"
)

func TestLinkCorpus(t *testing.T) {
	h, why, ok := findHost(t)
	if !ok {
		t.Skip(why)
	}

	dirs, names := corpusEntries(t, "link", h.target(), true)
	for i, dir := range dirs {
		t.Run(names[i], func(t *testing.T) {
			a := filepath.Join(dir, "a.cpp")
			b := filepath.Join(dir, "b.cpp")

			want := runMixed(t, h, a, b, false, false)
			if got := runMixed(t, h, a, b, true, false); got != want {
				t.Errorf("a by vcx, b by the platform compiler: exited %d; the platform compiler alone exited %d", got, want)
			}
			if got := runMixed(t, h, a, b, false, true); got != want {
				t.Errorf("a by the platform compiler, b by vcx: exited %d; the platform compiler alone exited %d", got, want)
			}
		})
	}
}

// runMixed builds a program from two files, each by the compiler the flag
// names, links them, and runs the result.
func runMixed(t *testing.T, h host, a, b string, aByVCX, bByVCX bool) int {
	t.Helper()
	dir := t.TempDir()

	objA := compileFor(t, h, dir, a, "a", aByVCX)
	objB := compileFor(t, h, dir, b, "b", bByVCX)
	bin := filepath.Join(dir, h.exeName("prog"))

	if out, err := h.linkProgram(bin, objA, objB); err != nil {
		t.Fatalf("linking failed: %v\n%s", err, out)
	}
	return run(t, bin)
}

// compileFor compiles one file to an object with whichever compiler is
// asked for.
func compileFor(t *testing.T, h host, dir, src, stem string, byVCX bool) string {
	t.Helper()
	stem += map[bool]string{true: "_vcx", false: "_host"}[byVCX]
	if byVCX {
		return buildWithVCX(t, h, dir, src, stem)
	}
	obj := filepath.Join(dir, h.objName(stem))
	if out, err := h.compile(dir, src, obj); err != nil {
		t.Fatalf("the platform compiler refused a corpus file: %v\n%s", err, out)
	}
	return obj
}
