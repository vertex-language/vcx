package vcx_test

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestObjCCorpus is the Objective-C++ ladder, tests/objc: each program is
// built by clang++ as Objective-C++ with ARC -- how a .mm is compiled
// everywhere vcx meets one -- and by vcx, then both are run and compared.
//
// The rungs came from objv's corpus: the same programs, as .mm. A rung
// may name what it links beyond Foundation on a line of its own:
//
//	// frameworks: CoreGraphics
//	// libraries: m
func TestObjCCorpus(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Objective-C++ needs Apple's runtime")
	}
	clangxx, err := exec.LookPath("clang++")
	if err != nil {
		t.Skip("no clang++ on PATH; there is no oracle to compare against")
	}
	for _, f := range ladder(t, filepath.Join("tests", "objc"), "*.mm") {
		t.Run(strings.TrimSuffix(filepath.Base(f), ".mm"), func(t *testing.T) {
			frameworks, libs := objcLinks(t, f)
			dir := t.TempDir()

			oracle := filepath.Join(dir, "oracle")
			args := []string{"-x", "objective-c++", "-std=c++23", "-fobjc-arc", "-w", "-ffp-contract=off", "-o", oracle, f}
			for _, fw := range frameworks {
				args = append(args, "-framework", fw)
			}
			for _, l := range libs {
				args = append(args, "-l"+l)
			}
			if out, err := exec.Command(clangxx, args...).CombinedOutput(); err != nil {
				t.Fatalf("clang++ refused it: %v\n%s", err, out)
			}
			want := runProgram(t, oracle)

			prog := filepath.Join(dir, "vcx")
			vargs := []string{"build", "-o", prog}
			for _, fw := range frameworks {
				vargs = append(vargs, "-framework", fw)
			}
			for _, l := range libs {
				vargs = append(vargs, "-l"+l)
			}
			vcxBuild(t, append(vargs, f)...)
			got := runProgram(t, prog)
			compareRuns(t, got, want, "clang++'s")
		})
	}
}

// objcLinks reads a rung's frameworks and libraries. Foundation is always
// linked.
func objcLinks(t *testing.T, path string) (frameworks, libs []string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	frameworks = []string{"Foundation"}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if rest, ok := strings.CutPrefix(line, "// frameworks:"); ok {
			frameworks = append(frameworks, strings.Fields(rest)...)
		}
		if rest, ok := strings.CutPrefix(line, "// libraries:"); ok {
			libs = append(libs, strings.Fields(rest)...)
		}
	}
	return frameworks, libs
}
