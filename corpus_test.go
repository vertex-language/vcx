package vcx_test

// The corpus: tests/NNN-*.cpp, one small thing per file, numbered in the
// order they climb (see tests/README.md), and the offload ladders beside it
// in tests/cuda, tests/hip and tests/metal.
//
// Every C++ file is built twice -- once by vcx, once by the platform's own
// compiler -- run, and the stdout and exit status compared. Nothing here
// writes down an expected value: the other compiler's answer is the oracle,
// and a disagreement with it is a bug in vcx by definition.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCorpus(t *testing.T) {
	h, why, ok := findHost(t)
	if !ok {
		t.Skip(why)
	}
	files := ladder(t, "tests", "*.cpp")
	for _, f := range files {
		t.Run(strings.TrimSuffix(filepath.Base(f), ".cpp"), func(t *testing.T) {
			dir := t.TempDir()
			oracle := filepath.Join(dir, h.exeName("oracle"))
			if out, err := h.build(dir, f, oracle); err != nil {
				t.Fatalf("the platform compiler refused it: %v\n%s", err, out)
			}
			want := runProgram(t, oracle)

			prog := filepath.Join(dir, h.exeName("vcx"))
			vcxBuild(t, "build", "-target", h.target(), "-o", prog, f)
			got := runProgram(t, prog)
			compareRuns(t, got, want, "the platform compiler's")
		})
	}
}

// ladder is a ladder's files in order: the NNN- prefix sorts them.
func ladder(t *testing.T, dir, pattern string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no %s in %s", pattern, dir)
	}
	sort.Strings(files)
	return files
}

type programRun struct {
	out    string
	status string // "exit N", or how it ended otherwise
}

// runProgram runs a built program with a time limit and a cap on its
// output: a miscompiled loop must fail its own test, not take the whole run
// down with it.
func runProgram(t *testing.T, bin string) programRun {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	var out capped
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if ctx.Err() != nil {
		return programRun{out: out.String(), status: "timed out after 10s"}
	}
	var exit *exec.ExitError
	switch {
	case err == nil:
		return programRun{out: out.String(), status: "exit 0"}
	case errors.As(err, &exit):
		if code := exit.ExitCode(); code >= 0 {
			return programRun{out: out.String(), status: "exit " + strconv.Itoa(code)}
		}
		return programRun{out: out.String(), status: exit.String()}
	}
	t.Fatalf("running %s: %v", bin, err)
	return programRun{}
}

// compareRuns fails the test where the two runs differ.
func compareRuns(t *testing.T, got, want programRun, oracle string) {
	t.Helper()
	if got.out != want.out {
		t.Errorf("output differs from %s\n--- vcx ---\n%s\n--- %s ---\n%s", oracle, clip(got.out), oracle, clip(want.out))
	}
	if got.status != want.status {
		t.Errorf("vcx's build ended with %s; %s with %s", got.status, oracle, want.status)
	}
}

// capped is a buffer that keeps the first megabyte written to it.
type capped struct{ bytes.Buffer }

func (c *capped) Write(p []byte) (int, error) {
	if room := 1<<20 - c.Len(); room < len(p) {
		if room > 0 {
			c.Buffer.Write(p[:room])
		}
		return len(p), nil
	}
	return c.Buffer.Write(p)
}

// clip shortens a long output for a failure message.
func clip(s string) string {
	const max = 4000
	if len(s) > max {
		return s[:max] + "\n... (" + strconv.Itoa(len(s)-max) + " more bytes)"
	}
	return s
}

// vpp is the v++ binary, built once per test run. Every ladder builds its
// files through it, in a subprocess with a time limit, so that a compile
// that never finishes fails its own test instead of the whole run.
var vpp struct {
	once sync.Once
	path string
	err  error
}

func vppBinary(t *testing.T) string {
	t.Helper()
	vpp.once.Do(func() {
		dir, err := os.MkdirTemp("", "vcx-v++-")
		if err != nil {
			vpp.err = err
			return
		}
		vpp.path = filepath.Join(dir, exeName("v++"))
		if out, err := exec.Command("go", "build", "-o", vpp.path, "./cmd/v++").CombinedOutput(); err != nil {
			vpp.err = fmt.Errorf("building v++: %v\n%s", err, out)
		}
	})
	if vpp.err != nil {
		t.Fatal(vpp.err)
	}
	return vpp.path
}

// vcxBuild runs v++ with args, failing the test on a diagnostic, a crash,
// or a compile that runs past two minutes.
func vcxBuild(t *testing.T, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, vppBinary(t), args...).CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("vcx did not finish compiling in two minutes: v++ %s", strings.Join(args, " "))
	}
	if err != nil {
		t.Fatalf("vcx: %v\n%s", err, clip(string(out)))
	}
}
