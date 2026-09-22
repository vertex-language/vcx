package vcx_test

// Host platform toolchain interface for running corpus tests.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
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

// build is the oracle's build. -ffp-contract=off because vcx does not fuse
// a*b+c, and a last-bit difference there is a known gap rather than
// something every float test should trip over.
func (c clang) build(dir, src, bin string) (string, error) {
	return c.run("-std=c++23", "-w", "-ffp-contract=off", "-o", bin, src)
}

func (c clang) linkProgram(bin string, objs ...string) (string, error) {
	return c.run(append([]string{"-o", bin}, objs...)...)
}

func (c clang) run(args ...string) (string, error) {
	cmd := exec.Command(c.exe, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// --- MSVC discovery ---

// msvc holds paths and environment for cl.exe and link.exe.
type msvc struct {
	exe     string
	link    string
	include string
	lib     string
}

// run compiles a file with cl.exe and returns compiler output.
func (m msvc) run(dir, path string, flags ...string) (string, error) {
	args := append([]string{"/nologo", "/std:c++latest", "/utf-8", "/c", "/Fo:" + dir + `\`}, flags...)
	args = append(args, filepath.ToSlash(path))
	cmd := exec.Command(m.exe, args...)
	cmd.Env = append(os.Environ(), "INCLUDE="+m.include)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func findMSVC(t *testing.T) (msvc, bool) {
	t.Helper()

	toolsRoot := `C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Tools\MSVC`
	toolset, ok := newestDir(toolsRoot)
	if !ok {
		return msvc{}, false
	}
	bin := filepath.Join(toolsRoot, toolset, `bin\Hostx64\x64`)
	exe := filepath.Join(bin, "cl.exe")
	if _, err := os.Stat(exe); err != nil {
		return msvc{}, false
	}

	kitsRoot := `C:\Program Files (x86)\Windows Kits\10`
	kitsInclude := filepath.Join(kitsRoot, "Include")
	sdk, ok := newestDir(kitsInclude)
	if !ok {
		return msvc{}, false
	}

	include := strings.Join([]string{
		filepath.Join(toolsRoot, toolset, "include"),
		filepath.Join(kitsInclude, sdk, "ucrt"),
		filepath.Join(kitsInclude, sdk, "shared"),
		filepath.Join(kitsInclude, sdk, "um"),
	}, ";")
	// The linker needs %LIB% for the same reason cl needs %INCLUDE%, and
	// for the same reason it is set here rather than by vcvars: on this
	// machine vcvars resolves no Windows SDK at all.
	lib := strings.Join([]string{
		filepath.Join(toolsRoot, toolset, `lib\x64`),
		filepath.Join(kitsRoot, "Lib", sdk, `ucrt\x64`),
		filepath.Join(kitsRoot, "Lib", sdk, `um\x64`),
	}, ";")

	return msvc{
		exe:     exe,
		link:    filepath.Join(bin, "link.exe"),
		include: include,
		lib:     lib,
	}, true
}

// newestDir is the last entry of a versioned directory, which is how both
// the MSVC toolsets and the Windows Kits are laid out.
func newestDir(root string) (string, bool) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", false
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 0 {
		return "", false
	}
	sort.Strings(dirs)
	return dirs[len(dirs)-1], true
}
