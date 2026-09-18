package vcx_test

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/vertex-language/vcx"
)

// The cuda corpus: does a kernel written in CUDA compile to PTX, and does
// it compute? Each file under tests/cuda/device is one kernel named
// `test`, of the signature `__global__ void test(int *out)`, and says in
// its header how it is launched and what it writes:
//
//	// grid: 4
//	// block: 64
//	// shmem: 256        (dynamic shared memory, optional)
//	// expect: 0 1 2 3 ...
//
// The expected values are the file's own -- worked out by hand, or by
// nvcc on a machine that has it -- and never restated from the lowering.
// A float result is written through __float_as_int, or scaled to an
// integer, so that every expectation is a whole number.
//
// TestCUDACorpusCompiles lowers every file to PTX on any machine;
// TestCUDACorpusRuns (cuda_windows_test.go) runs each on the GPU through
// the driver where there is one.

type cudaCase struct {
	name   string
	path   string
	grid   [3]uint32
	block  [3]uint32
	expect []int32
	arch   string
	shmem  uint32 // dynamic shared memory, in bytes
}

func cudaCases(t *testing.T) []cudaCase {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("tests", "cuda", "device", "*.cu"))
	if err != nil || len(files) == 0 {
		t.Skip("no tests/cuda/device corpus")
	}
	sort.Strings(files)
	var out []cudaCase
	for _, path := range files {
		c := cudaCase{name: strings.TrimSuffix(filepath.Base(path), ".cu"), path: path, grid: [3]uint32{1, 1, 1}, block: [3]uint32{1, 1, 1}, arch: "sm_52"}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			key, rest, ok := strings.Cut(strings.TrimPrefix(line, "//"), ":")
			if !strings.HasPrefix(line, "//") || !ok {
				continue
			}
			rest = strings.TrimSpace(rest)
			switch strings.TrimSpace(key) {
			case "grid":
				c.grid = dims(t, path, rest)
			case "block":
				c.block = dims(t, path, rest)
			case "arch":
				c.arch = rest
			case "shmem":
				n, err := strconv.ParseUint(rest, 10, 32)
				if err != nil {
					t.Fatalf("%s: shmem %q: %v", path, rest, err)
				}
				c.shmem = uint32(n)
			case "expect":
				for _, w := range strings.Fields(rest) {
					n, err := strconv.ParseInt(w, 0, 32)
					if err != nil {
						t.Fatalf("%s: expect %q: %v", path, w, err)
					}
					c.expect = append(c.expect, int32(n))
				}
			}
		}
		f.Close()
		out = append(out, c)
	}
	return out
}

func dims(t *testing.T, path, s string) [3]uint32 {
	d := [3]uint32{1, 1, 1}
	for i, w := range strings.Fields(s) {
		if i >= 3 {
			t.Fatalf("%s: more than three dimensions in %q", path, s)
		}
		n, err := strconv.ParseUint(w, 10, 32)
		if err != nil {
			t.Fatalf("%s: dimension %q: %v", path, w, err)
		}
		d[i] = uint32(n)
	}
	return d
}

// ptxOf compiles the case's file for its architecture and is the PTX.
func ptxOf(t *testing.T, c cudaCase, arch string) string {
	t.Helper()
	comp := &vcx.Compiler{OffloadArch: arch, DeviceOnly: true}
	obj, diags, err := comp.Object(vcx.File(c.path))
	if err != nil {
		t.Fatalf("%s: %v", c.path, err)
	}
	for _, d := range diags {
		t.Logf("%s: %s", c.path, d)
	}
	if vcx.HasErrors(diags) || obj == nil {
		t.Fatalf("%s: did not compile", c.path)
	}
	return string(obj)
}

func TestCUDACorpusCompiles(t *testing.T) {
	for _, c := range cudaCases(t) {
		t.Run(c.name, func(t *testing.T) {
			src := ptxOf(t, c, c.arch)
			if !strings.Contains(src, ".entry _Z4testPi") {
				t.Fatalf("no kernel `test(int *)` in the PTX:\n%s", src)
			}
		})
	}
}
