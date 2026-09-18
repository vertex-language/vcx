//go:build windows

package vcx_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/vertex-language/vcx"
)

// nvcc as the oracle: where the toolkit and cl.exe are installed, every
// program of tests/cuda/host is built by nvcc too, run, and its output
// compared with what the vcx-built program printed. The header's
// `// expect:` lines are what a reader sees; this is the check that they
// are what NVIDIA's own compiler prints, and that two compilers agree on
// every program, which is the test of a drop-in replacement.
func TestCUDAProgramsAgainstNvcc(t *testing.T) {
	tk, ok := vcx.FindCUDAToolkit()
	if !ok {
		t.Skip("no CUDA toolkit")
	}
	nvcc := filepath.Join(tk.Dir, "bin", "nvcc.exe")
	if _, err := os.Stat(nvcc); err != nil {
		t.Skip("no nvcc in the toolkit")
	}
	cl, ok := findMSVC(t)
	if !ok {
		t.Skip("no cl.exe for nvcc's host side")
	}
	if _, skip := driverPresent(); skip != "" {
		t.Skip(skip)
	}
	files, err := filepath.Glob(filepath.Join("tests", "cuda", "host", "*.cu"))
	if err != nil || len(files) == 0 {
		t.Skip("no tests/cuda/host corpus")
	}
	sort.Strings(files)
	dir := t.TempDir()
	for _, path := range files {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".cu"), func(t *testing.T) {
			want, archs := expectLines(t, path)
			exe := filepath.Join(dir, strings.TrimSuffix(filepath.Base(path), ".cu")+".exe")
			args := []string{"-ccbin", filepath.Dir(cl.exe), "-o", exe}
			for _, a := range archs {
				args = append(args, "-gencode", "arch=compute_"+strings.TrimPrefix(a, "sm_")+",code="+a)
			}
			if len(archs) == 0 {
				args = append(args, "-arch=sm_75")
			}
			cmd := exec.Command(nvcc, append(args, path)...)
			cmd.Env = append(os.Environ(), "INCLUDE="+cl.include, "LIB="+cl.lib)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("nvcc refused %s: %v\n%s", path, err, out)
			}
			out, err := exec.Command(exe).Output()
			if err != nil {
				t.Fatalf("nvcc's %s: %v\n%s", path, err, out)
			}
			got := strings.TrimRight(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
			if got != want {
				t.Fatalf("nvcc's %s printed:\n%s\nthe header expects:\n%s", path, got, want)
			}
		})
	}
}
