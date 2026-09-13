package vcx

// The GNU dialect's predefines, held to clang's.
//
// Every macro vcx defines for a GNU target has to be the one clang defines
// for the same triple, with the same replacement list. The list is generated
// from the data model rather than copied, so this is where a wrong type
// choice -- Darwin's int64_t, Linux's wint_t -- shows up, before any header
// has read it. Macros clang has and vcx does not are fine: the first kind
// of mistake is a wrong claim, the second an unmade one.

import (
	"os/exec"
	"strings"
	"testing"
)

// clangTriple is the triple clang names each GNU target by.
var clangTriple = map[string]string{
	"aarch64-macos": "arm64-apple-macos11",
	"x86_64-macos":  "x86_64-apple-macos10.13",
	"aarch64-linux": "aarch64-unknown-linux-gnu",
	"x86_64-linux":  "x86_64-unknown-linux-gnu",
}

// vcxOwn are the macros vcx defines as itself rather than as clang: its
// version string, and the feature-test macros of what it implements.
func vcxOwn(name string) bool {
	return strings.HasPrefix(name, "__cpp_") || name == "__VERSION__" ||
		name == "__clang_version__" || strings.HasPrefix(name, "__clang_") && name != "__clang_literal_encoding__" ||
		name == "__CLANG_ATOMIC_CHAR8_T_LOCK_FREE" || name == "__GCC_ATOMIC_CHAR8_T_LOCK_FREE"
}

func TestGNUPredefines(t *testing.T) {
	clang, err := exec.LookPath("clang++")
	if err != nil {
		t.Skip("no clang++ to compare against")
	}
	for target, triple := range clangTriple {
		t.Run(target, func(t *testing.T) {
			out, err := exec.Command(clang, "-target", triple, "-std=c++23", "-dM", "-E", "-x", "c++", "/dev/null").Output()
			if err != nil {
				t.Skipf("clang++ cannot target %s: %v", triple, err)
			}
			theirs := map[string]string{}
			for _, line := range strings.Split(string(out), "\n") {
				rest, ok := strings.CutPrefix(line, "#define ")
				if !ok {
					continue
				}
				name, value, _ := strings.Cut(rest, " ")
				theirs[name] = value
			}
			tgt, err := TargetByName(target)
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range tgt.Predefines() {
				name, value, _ := strings.Cut(d.Text, "=")
				if vcxOwn(name) {
					continue
				}
				want, has := theirs[name]
				switch {
				case !has:
					t.Errorf("%s: vcx defines it as %q; clang does not define it", name, value)
				case want != value:
					t.Errorf("%s: vcx says %q, clang says %q", name, value, want)
				}
			}
		})
	}
}
