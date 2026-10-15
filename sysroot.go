package vcx

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vertex-language/vcx/sysroot"
)

// SystemInclude is a header directory or filesystem for the target.
type SystemInclude struct {
	Name string
	FS   fs.FS
	// Framework marks a directory of Apple frameworks, searched as -F.
	Framework bool
}

// SystemIncludes returns the target's system header search paths in priority order.
func (c *Compiler) SystemIncludes() []SystemInclude {
	tgt, err := c.target()
	if err != nil {
		return nil
	}
	var own []SystemInclude
	if tgt.Dialect == DialectGNU {
		own = []SystemInclude{compilerHeaders()}
	}
	if c.Freestanding {
		return own
	}
	if c.Sysroot != "" {
		return append([]SystemInclude{{Name: c.Sysroot, FS: os.DirFS(c.Sysroot)}}, own...)
	}
	entries, _ := sysroot.Resolve(tgt.Name, true)
	var out []SystemInclude
	for _, e := range entries {
		if e.Name == "<builtin>" {
			if tgt.OS == "macos" {
				if dir, ok := libcxxDir(); ok {
					out = append(out, SystemInclude{Name: dir, FS: os.DirFS(dir)})
				}
			}
			if tgt.OS == "linux" {
				if dir, ok := linuxLibcxxDir(); ok {
					out = append(out, SystemInclude{Name: dir, FS: os.DirFS(dir)})
				}
			}
			out = append(out, own...)
			continue
		}
		out = append(out, SystemInclude{Name: e.Name, FS: e.FS})
	}
	// An Apple SDK's frameworks, after its headers, as clang searches
	// them: <CoreFoundation/CoreFoundation.h> is a framework's header.
	if tgt.OS == "macos" {
		if sdk, ok := sysroot.SDK(nil); ok {
			for _, sub := range []string{"System/Library/Frameworks", "System/Library/SubFrameworks"} {
				dir := filepath.Join(sdk, filepath.FromSlash(sub))
				if st, err := os.Stat(dir); err == nil && st.IsDir() {
					out = append(out, SystemInclude{Name: dir, FS: os.DirFS(dir), Framework: true})
				}
			}
		}
	}
	return out
}

//go:embed include/gnu
var compilerHeaderFS embed.FS

// compilerHeaders returns an include mount for embedded compiler-provided headers.
func compilerHeaders() SystemInclude {
	sub, err := fs.Sub(compilerHeaderFS, "include/gnu")
	if err != nil {
		panic("vcx: embedded compiler headers missing: " + err.Error())
	}
	return SystemInclude{Name: "<vcx>", FS: sub}
}

// linuxLibcxxDir returns a Linux distribution's libc++ header directory
// if one is installed (Debian's and Ubuntu's libc++-dev): libc++, not
// libstdc++, is the library vcx's C++ is written against, as on macOS.
func linuxLibcxxDir() (string, bool) {
	// The newest LLVM's first: apt.llvm.org installs beside the
	// distribution's, which /usr/include/c++/v1 is.
	dirs, _ := filepath.Glob("/usr/lib/llvm-*/include/c++/v1")
	sort.Slice(dirs, func(i, j int) bool { return llvmVersion(dirs[i]) > llvmVersion(dirs[j]) })
	dirs = append(dirs, "/usr/include/c++/v1")
	for _, dir := range dirs {
		if st, err := os.Stat(filepath.Join(dir, "__config")); err == nil && !st.IsDir() {
			return dir, true
		}
	}
	return "", false
}

// llvmVersion is the major version in a /usr/lib/llvm-N/... path.
func llvmVersion(path string) int {
	_, rest, ok := strings.Cut(path, "/llvm-")
	if !ok {
		return 0
	}
	n := 0
	for _, c := range rest {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// libcxxDir returns the macOS SDK libc++ header directory if found.
func libcxxDir() (string, bool) {
	sdk, ok := sysroot.SDK(nil)
	if !ok {
		return "", false
	}
	dir := filepath.Join(sdk, "usr", "include", "c++", "v1")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return "", false
	}
	return dir, true
}
