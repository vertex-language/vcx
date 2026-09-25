package vcx

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"

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
