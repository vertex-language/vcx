package vcx_test

// Host platform toolchain interface for running corpus tests.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	vcx "github.com/vertex-language/vcx"
)

type host interface {
	// target is the vcx target the host's objects are built for.
	target() string

	// objName and exeName spell an object and executable the host's way.
	objName(stem string) string
	exeName(stem string) string

	// compile builds a source file to an object file.
	compile(dir, src, obj string) (string, error)

	// build compiles and links a source file into a program.
	build(dir, src, bin string) (string, error)

	// linkProgram links objects into an executable.
	linkProgram(bin string, objs ...string) (string, error)
}

// findHost finds the native toolchain for the current platform.
func findHost(t *testing.T) (host, string, bool) {
	t.Helper()
	switch runtime.GOOS {
	case "windows":
		cl, ok := findMSVC(t)
		if !ok {
			return nil, "no MSVC found; there is no oracle to compare against and no linker", false
		}
		return cl, "", true
	case "darwin":
		exe, err := exec.LookPath("clang++")
		if err != nil {
			return nil, "no clang++ on PATH; there is no oracle to compare against and no linker", false
		}
		return clang{exe: exe, tgt: vcx.DefaultTarget().Name}, "", true
	}
	return nil, "no oracle wired up for " + runtime.GOOS, false
}

// --- MSVC ---

func (msvc) target() string             { return "x86_64-windows" }
func (msvc) objName(stem string) string { return stem + ".obj" }
func (msvc) exeName(stem string) string { return stem + ".exe" }

func (m msvc) compile(dir, src, obj string) (string, error) {
	return m.run(dir, src, "/Fo:"+obj)
}

// build compiles and links with cl.exe.
func (m msvc) build(dir, src, bin string) (string, error) {
	args := []string{
		"/nologo", "/std:c++latest", "/W0", "/EHsc", "/utf-8",
		"/Fo:" + dir + `\`,
		"/Fe:" + bin,
		filepath.ToSlash(src),
	}
	cmd := exec.Command(m.exe, args...)
	cmd.Env = append(os.Environ(), "INCLUDE="+m.include, "LIB="+m.lib)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// linkProgram links objects into a console executable.
func (m msvc) linkProgram(bin string, objs ...string) (string, error) {
	args := append([]string{"/nologo", "/subsystem:console", "/out:" + bin}, objs...)
	args = append(args, "libcmt.lib")
	cmd := exec.Command(m.link, args...)
	cmd.Env = append(os.Environ(), "LIB="+m.lib)
	b, err := cmd.CombinedOutput()
	return string(b), err
}

// --- clang++ ---

// clang invokes clang++ for compiling and linking.
type clang struct {
	exe string
	tgt string
}

func (c clang) target() string           { return c.tgt }
func (clang) objName(stem string) string { return stem + ".o" }
func (clang) exeName(stem string) string { return stem }

func (c clang) compile(dir, src, obj string) (string, error) {
	return c.run("-std=c++23", "-w", "-c", "-o", obj, src)
}

func (c clang) build(dir, src, bin string) (string, error) {
	return c.run("-std=c++23", "-w", "-o", bin, src)
}

func (c clang) linkProgram(bin string, objs ...string) (string, error) {
	return c.run(append([]string{"-o", bin}, objs...)...)
}

func (c clang) run(args ...string) (string, error) {
	cmd := exec.Command(c.exe, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
