package cli

import (
	"strings"
)

// The driver spelling: `v++ main.cu -o main`, `v++ -c -O2 -Ifoo x.cpp`,
// `CXX=v++` in a build that expects nvcc, hipcc, clang++ or g++. A
// command line whose first word is no command of v++'s is a build, and
// its arguments are read the way those drivers read them: flags and
// files in any order, a value attached to its flag (-Ifoo, -DX=1, -lm)
// or after it, and the flags the other drivers take that mean nothing
// here accepted and ignored, so that a Makefile written for one of them
// runs unchanged.

// isCommand reports whether the word names one of v++'s commands.
func isCommand(word string) bool {
	switch word {
	case "tokens", "ast", "layout", "symbols", "check", "build", "run", "env", "help", "-h", "--help":
		return true
	}
	return false
}

// valueFlags are v++'s flags that take a value in the next argument.
var valueFlags = map[string]bool{
	"I": true, "D": true, "U": true, "target": true, "std": true, "x": true, "o": true,
	"l": true, "L": true, "offload-arch": true, "arch": true, "emit": true,
	"cudart": true, "cuda-path": true,
}

// attachable are the flags whose value may be written against them.
const attachable = "IDULlo"

// ignoredFlags are what nvcc, hipcc, gcc and clang take that v++ has no
// use for: accepted so the build goes on. The value says whether the
// flag consumes the argument after it.
var ignoredFlags = map[string]bool{
	"g": false, "G": false, "lineinfo": false, "w": false, "Wall": false, "Wextra": false, "Werror": false,
	"pedantic": false, "fPIC": false, "fpic": false, "m64": false, "pipe": false, "pthread": false,
	"expt-relaxed-constexpr": false, "expt-extended-lambda": false, "extended-lambda": false,
	"use_fast_math": false, "ftz=true": false, "ftz=false": false, "prec-div=true": false, "prec-sqrt=true": false,
	"fno-exceptions": false, "fno-rtti": false, "fexceptions": false, "frtti": false, "fvisibility=hidden": false,
	"MMD": false, "MD": false, "MP": false, "M": false,
	"MF": true, "MT": true, "MQ": true,
	"Xcompiler": true, "Xlinker": true, "Xptxas": true, "Xnvlink": true, "Xcudafe": true,
	"ccbin": true, "compiler-bindir": true, "maxrregcount": true, "default-stream": true,
	"cudadevrt": true, "gpu-architecture": true, "gpu-code": true,
}

// driverArgs rewrites a driver-style command line into what cmdBuild's
// flag set reads: every flag first, each with its value separate, and
// the files after. It returns the arguments and the flags it dropped.
func driverArgs(args []string) (out []string, dropped []string) {
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			files = append(files, a)
			continue
		}
		name := strings.TrimLeft(a, "-")
		flagName, value, hasValue := strings.Cut(name, "=")

		// -O2, -O3, -Os: no -O yet; the level is ignored.
		if strings.HasPrefix(flagName, "O") && len(flagName) <= 2 {
			dropped = append(dropped, a)
			continue
		}
		// -gencode arch=compute_75,code=sm_75: the device named inside.
		if flagName == "gencode" {
			spec := value
			if !hasValue && i+1 < len(args) {
				i++
				spec = args[i]
			}
			if arch := gencodeArch(spec); arch != "" {
				out = append(out, "-offload-arch", arch)
			} else {
				dropped = append(dropped, a)
			}
			continue
		}
		// -std=c++17 and older: v++ speaks C++20 and later, which are
		// supersets for what a build of that vintage asks.
		if flagName == "std" {
			if !hasValue && i+1 < len(args) {
				i++
				value = args[i]
			}
			switch strings.ToLower(value) {
			case "c++11", "c++14", "c++17", "gnu++11", "gnu++14", "gnu++17", "gnu++20":
				value = "c++20"
			case "gnu++23":
				value = "c++23"
			}
			out = append(out, "-std", value)
			continue
		}
		if takes, ignored := ignoredFlags[flagName]; ignored {
			dropped = append(dropped, a)
			if takes && !hasValue && i+1 < len(args) {
				i++
			}
			continue
		}
		if takes, ignored := ignoredFlags[name]; ignored {
			dropped = append(dropped, a)
			if takes && i+1 < len(args) {
				i++
			}
			continue
		}
		// A value written against a one-letter flag: -Ifoo, -DX=1, -lm.
		if len(name) > 1 && strings.IndexByte(attachable, name[0]) >= 0 && !valueFlags[flagName] && !isKnownFlag(flagName) {
			out = append(out, "-"+name[:1], name[1:])
			continue
		}
		if valueFlags[flagName] {
			if hasValue {
				out = append(out, "-"+flagName, value)
			} else if i+1 < len(args) {
				i++
				out = append(out, "-"+flagName, args[i])
			}
			continue
		}
		out = append(out, "-"+name)
	}
	return append(out, files...), dropped
}

// isKnownFlag reports whether the whole spelling is a flag of v++'s, so
// that -lineinfo is not read as -l ineinfo.
func isKnownFlag(name string) bool {
	switch name {
	case "c", "freestanding", "cuda-device-only", "cuda-host-only", "offload-device-only", "offload-host-only", "help", "h":
		return true
	}
	_, ignored := ignoredFlags[name]
	return ignored || valueFlags[name]
}

// gencodeArch is the sm_NN a -gencode value names, from its code= or
// arch= part, or "".
func gencodeArch(spec string) string {
	for _, part := range strings.Split(spec, ",") {
		k, v, _ := strings.Cut(part, "=")
		switch k {
		case "code", "arch":
			v = strings.Trim(v, "[]\"")
			for _, item := range strings.Split(v, ",") {
				if strings.HasPrefix(item, "sm_") {
					return item
				}
				if rest, ok := strings.CutPrefix(item, "compute_"); ok {
					return "sm_" + rest
				}
			}
		}
	}
	return ""
}
