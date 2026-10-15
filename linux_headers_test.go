package vcx

import (
	"testing"

	"github.com/vertex-language/ir"
)

// What glibc's headers write that macOS's never made vcx read: a macro
// named by a keyword vcx knows from clang (glibc defines
// __nonnull(params), which Apple's headers spell as a qualifier), GNU's
// __signed__, and a cast to __typeof (x).
func TestGlibcHeaderForms(t *testing.T) {
	src := `
#define __attribute_nonnull__(params) __attribute__ ((__nonnull__ params))
#define __nonnull(params) __attribute_nonnull__ (params)
extern "C" int f(const void* a, const void* b) __nonnull ((1, 2));
typedef __signed__ char s8;
#define __iscanonicalf(x) ((void) (__typeof (x)) (x), 1)
int canonical(float v) { return __iscanonicalf (v); }
__typeof__(int*) p;
int main() { s8 c = -1; return canonical(1.0f) + c + (p == nullptr ? 0 : 1); }
`
	c := &Compiler{Std: Cxx23}
	if _, diags, err := c.IR(Text("glibc.cpp", []byte(src))); err != nil || HasErrors(diags) {
		t.Fatalf("err %v, diagnostics %v", err, diags)
	}
}

// [basic.link]/3.2: a const variable is internal unless declared extern or
// inline. An inline one -- libc++'s `inline constexpr` customization
// points -- is one object a program over, kept once from many units by
// COMDAT; a static one is its unit's, and in no group, or another unit's
// of its name would elect it away.
func TestInlineConstVariableLinkage(t *testing.T) {
	src := `
inline constexpr int shared = 1;
static inline constexpr int own = 2;
int main() { return shared + own; }
`
	c := &Compiler{Std: Cxx23}
	m, diags, err := c.IR(Text("inline.cpp", []byte(src)))
	if err != nil || HasErrors(diags) {
		t.Fatalf("err %v, diagnostics %v", err, diags)
	}
	found := 0
	for _, g := range m.Globals() {
		_, grouped := g.ComdatAttr()
		switch g.Name() {
		case "_own":
			found++
			if g.Linkage() != ir.Internal || grouped {
				t.Errorf("static inline const %s: linkage %v, COMDAT %v; want internal, none", g.Name(), g.Linkage(), grouped)
			}
		case "_shared":
			found++
			if g.Linkage() == ir.Internal || !grouped {
				t.Errorf("inline const %s: linkage %v, COMDAT %v; want external, COMDAT", g.Name(), g.Linkage(), grouped)
			}
		}
	}
	if found != 2 {
		for _, g := range m.Globals() {
			t.Logf("global %s", g.Name())
		}
		t.Errorf("found %d of the two variables", found)
	}
}
