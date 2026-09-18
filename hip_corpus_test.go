package vcx_test

import (
	"bytes"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/vertex-language/vcx"
)

// The hip corpus: does a kernel written in HIP compile to an HSA code
// object? The files follow the cuda corpus's conventions -- one kernel
// `test(int *out)`, the launch and the expectation in the header -- so
// that a machine with an AMD GPU can run them the same way; here, with
// none, they are lowered for gfx942 and the code object checked for the
// kernel's symbol and descriptor.
func TestHIPCorpusCompiles(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("tests", "hip", "device", "*.hip"))
	if err != nil || len(files) == 0 {
		t.Skip("no tests/hip/device corpus")
	}
	sort.Strings(files)
	for _, path := range files {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".hip"), func(t *testing.T) {
			comp := &vcx.Compiler{OffloadArch: "gfx942", DeviceOnly: true}
			obj, diags, err := comp.Object(vcx.File(path))
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			for _, d := range diags {
				t.Logf("%s: %s", path, d)
			}
			if vcx.HasErrors(diags) || obj == nil {
				t.Fatalf("%s: did not compile", path)
			}
			if !bytes.HasPrefix(obj, []byte("\x7fELF")) {
				t.Fatalf("%s: not an ELF code object", path)
			}
			for _, sym := range []string{"_Z4testPi\x00", "_Z4testPi.kd\x00"} {
				if !bytes.Contains(obj, []byte(sym)) {
					t.Errorf("%s: the code object has no %q", path, strings.TrimRight(sym, "\x00"))
				}
			}
		})
	}
}

// The hip programs corpus: whole programs under tests/hip/host, compiled
// through both passes -- the gfx942 code object bundled into the host
// object and registered -- and not run, since that takes an AMD GPU and
// ROCm's runtime. The object is checked for the bundle and the
// registration the runtime would read.
func TestHIPPrograms(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("tests", "hip", "host", "*.hip"))
	if err != nil || len(files) == 0 {
		t.Skip("no tests/hip/host corpus")
	}
	sort.Strings(files)
	for _, path := range files {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".hip"), func(t *testing.T) {
			comp := &vcx.Compiler{OffloadArch: "gfx942"}
			obj, diags, err := comp.Object(vcx.File(path))
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			for _, d := range diags {
				t.Logf("%s: %s", path, d)
			}
			if vcx.HasErrors(diags) || obj == nil {
				t.Fatalf("%s: did not compile", path)
			}
			for _, want := range []string{"__CLANG_OFFLOAD_BUNDLE__", "hipv4-amdgcn-amd-amdhsa--gfx942", "__hipRegisterFatBinary", "__hipRegisterFunction", "_Z6vecaddPKiS0_Pii"} {
				if !bytes.Contains(obj, []byte(want)) {
					t.Errorf("%s: the host object has no %q", path, want)
				}
			}
		})
	}
}
