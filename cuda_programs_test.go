package vcx_test

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/vertex-language/vcx"
)

// The cuda programs corpus: whole programs under tests/cuda/host, each
// with a main that launches kernels and prints, built and run by vcx
// alone -- the device pass, the host pass with the image embedded, vcx's
// runtime over the driver, vcx's linker -- on the machine's GPU. What a
// program prints is what its header's `// expect:` lines say, in order.
// A machine without an NVIDIA driver skips; nothing else does.
func TestCUDAPrograms(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("tests", "cuda", "host", "*.cu"))
	if err != nil || len(files) == 0 {
		t.Skip("no tests/cuda/host corpus")
	}
	sort.Strings(files)
	if _, skip := driverPresent(); skip != "" {
		t.Skip(skip)
	}
	for _, path := range files {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".cu"), func(t *testing.T) {
			want, archs := expectLines(t, path)
			c := &vcx.Compiler{OffloadArchs: archs}
			out, err := c.Run(path)
			if err != nil {
				t.Fatalf("%s: %v\n--- stdout ---\n%s", path, err, out)
			}
			got := strings.TrimRight(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
			if got != want {
				t.Fatalf("%s printed:\n%s\nwant:\n%s", path, got, want)
			}
		})
	}
}

// expectLines is the program's expected output, from its header, and
// the architectures its `// arch:` line asks to be built for.
func expectLines(t *testing.T, path string) (string, []string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var lines, archs []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if rest, ok := strings.CutPrefix(line, "// expect:"); ok {
			lines = append(lines, strings.TrimSpace(rest))
		}
		if rest, ok := strings.CutPrefix(line, "// arch:"); ok {
			archs = append(archs, strings.Fields(rest)...)
		}
	}
	return strings.Join(lines, "\n"), archs
}
