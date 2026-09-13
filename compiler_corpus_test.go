package vcx_test

// Test programs compiled and executed against the platform compiler (cl / clang++).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	vcx "github.com/vertex-language/vcx"
)

func TestCompilerCorpus(t *testing.T) {
	h, why, ok := findHost(t)
	if !ok {
		t.Skip(why)
	}

	files, names := corpusEntries(t, "compiler", h.target(), false)
	for i, file := range files {
		t.Run(names[i], func(t *testing.T) {
			want := runUnderHost(t, h, file)
			got := runUnderVCX(t, h, file)
			if got != want {
				t.Errorf("vcx's program exited %d, the platform compiler's exited %d", got, want)
			}
		})
	}
}

// runUnderVCX compiles with this compiler and runs the result.
func runUnderVCX(t *testing.T, h host, path string) int {
	t.Helper()

	obj := buildWithVCX(t, h, t.TempDir(), path, "prog")
	bin := filepath.Join(filepath.Dir(obj), h.exeName("prog"))
	if out, err := h.linkProgram(bin, obj); err != nil {
		t.Fatalf("linking vcx's object failed: %v\n%s", err, out)
	}
	return run(t, bin)
}

// buildWithVCX compiles one file to an object in dir, for the host's target.
func buildWithVCX(t *testing.T, h host, dir, path, stem string) string {
	t.Helper()

	c := &vcx.Compiler{Std: vcx.Cxx23, Target: h.target()}
	obj, diags, err := c.Object(vcx.File(path))
	if err != nil {
		t.Fatalf("vcx: %v", err)
	}
	if vcx.HasErrors(diags) {
		// A refusal is a failure here. Every program in this corpus is one
		// vcx is expected to handle; a diagnostic means it no longer does,
		// or never did and the file was added too early.
		var b strings.Builder
		for _, d := range diags {
			b.WriteString("\n  ")
			b.WriteString(d.Message)
		}
		t.Fatalf("vcx refused a program the corpus expects it to build:%s", b.String())
	}
	if obj == nil {
		t.Fatal("vcx produced no object and no diagnostic")
	}

	objPath := filepath.Join(dir, h.objName(stem))
	if err := os.WriteFile(objPath, obj, 0o644); err != nil {
		t.Fatal(err)
	}
	return objPath
}

// runUnderHost compiles the same text with the platform's compiler.
func runUnderHost(t *testing.T, h host, path string) int {
	t.Helper()

	dir := t.TempDir()
	bin := filepath.Join(dir, h.exeName("oracle"))
	if out, err := h.build(dir, path, bin); err != nil {
		t.Fatalf("the platform compiler refused a program the corpus expects it to accept: %v\n%s", err, out)
	}
	return run(t, bin)
}

// run executes a built program and reports how it ended.
func run(t *testing.T, bin string) int {
	t.Helper()
	cmd := exec.Command(bin)
	err := cmd.Run()
	if err != nil {
		if _, isExit := err.(*exec.ExitError); !isExit {
			t.Fatalf("running %s: %v", bin, err)
		}
	}
	return cmd.ProcessState.ExitCode()
}
