package vcx

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// A Toolkit is an installed CUDA toolkit: where its headers and
// libraries are. vcx needs none of it -- its own headers and runtime
// stand in -- and uses what is there when it is: the toolkit's cudart
// for the link, so that a program gets the whole runtime API.
type Toolkit struct {
	Dir     string // the root: C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v13.4, /usr/local/cuda
	Include string // Dir/include
	Lib     string // Dir/lib/x64 on Windows, Dir/lib64 elsewhere
	Version string // "13.4", from the directory's name where it says
}

// FindCUDAToolkit looks where the toolkit installs itself: CUDA_PATH and
// CUDA_HOME first, as nvcc's own scripts read them, then the default
// directories, newest version first. It reports whether one was found.
func FindCUDAToolkit() (Toolkit, bool) {
	for _, env := range []string{"CUDA_PATH", "CUDA_HOME"} {
		if dir := os.Getenv(env); dir != "" {
			if tk, ok := toolkitAt(dir); ok {
				return tk, true
			}
		}
	}
	var candidates []string
	if runtime.GOOS == "windows" {
		for _, root := range []string{os.Getenv("ProgramFiles"), `C:\Program Files`} {
			if root == "" {
				continue
			}
			base := filepath.Join(root, "NVIDIA GPU Computing Toolkit", "CUDA")
			entries, err := os.ReadDir(base)
			if err != nil {
				continue
			}
			var versions []string
			for _, e := range entries {
				if e.IsDir() && strings.HasPrefix(e.Name(), "v") {
					versions = append(versions, e.Name())
				}
			}
			sort.Slice(versions, func(i, j int) bool { return versionLess(versions[j], versions[i]) })
			for _, v := range versions {
				candidates = append(candidates, filepath.Join(base, v))
			}
		}
	} else {
		candidates = append(candidates, "/usr/local/cuda", "/opt/cuda")
	}
	for _, dir := range candidates {
		if tk, ok := toolkitAt(dir); ok {
			return tk, true
		}
	}
	return Toolkit{}, false
}

// toolkitAt is the toolkit rooted at dir, if its headers are there.
func toolkitAt(dir string) (Toolkit, bool) {
	include := filepath.Join(dir, "include")
	if st, err := os.Stat(filepath.Join(include, "cuda_runtime.h")); err != nil || st.IsDir() {
		return Toolkit{}, false
	}
	tk := Toolkit{Dir: dir, Include: include}
	if runtime.GOOS == "windows" {
		tk.Lib = filepath.Join(dir, "lib", "x64")
	} else {
		tk.Lib = filepath.Join(dir, "lib64")
		if _, err := os.Stat(tk.Lib); err != nil {
			tk.Lib = filepath.Join(dir, "lib")
		}
	}
	tk.Version = strings.TrimPrefix(filepath.Base(dir), "v")
	return tk, true
}

// versionLess orders "v12.6" before "v13.4" by number, not by text.
func versionLess(a, b string) bool {
	pa, pb := strings.Split(strings.TrimPrefix(a, "v"), "."), strings.Split(strings.TrimPrefix(b, "v"), ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		var x, y int
		for _, c := range pa[i] {
			if c >= '0' && c <= '9' {
				x = x*10 + int(c-'0')
			}
		}
		for _, c := range pb[i] {
			if c >= '0' && c <= '9' {
				y = y*10 + int(c-'0')
			}
		}
		if x != y {
			return x < y
		}
	}
	return len(pa) < len(pb)
}

// cudaToolkit is the toolkit this compilation uses: the one CUDAPath
// names, or the one found, unless CUDAPath is "none".
func (c *Compiler) cudaToolkit() (Toolkit, bool) {
	switch c.CUDAPath {
	case "none":
		return Toolkit{}, false
	case "":
		return FindCUDAToolkit()
	}
	return toolkitAt(c.CUDAPath)
}

// cudaRuntime is how a CUDA program's runtime is linked: "vcx" is vcx's
// own over the driver, "static" and "shared" the toolkit's cudart, and
// "none" nothing. Empty picks the toolkit's static cudart when a
// toolkit is installed and vcx's otherwise.
func (c *Compiler) cudaRuntime() (string, Toolkit) {
	tk, found := c.cudaToolkit()
	switch c.CUDARuntime {
	case "vcx", "none":
		return c.CUDARuntime, tk
	case "static", "shared":
		return c.CUDARuntime, tk
	}
	if found {
		return "static", tk
	}
	return "vcx", tk
}
