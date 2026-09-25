package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/vertex-language/vcx"
)

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

type ppFlags struct {
	includes     stringList
	defs         stringList
	undefs       stringList
	target       string
	std          string
	freestanding bool
	keepComments bool

	// The offload flags, as nvcc and hipcc spell them.
	language     string
	offloadArchs stringList
	deviceOnly   bool
	hostOnly     bool
	cudart       string
	cudaPath     string
	minOS        string
	noFastMath   bool
	moduleFiles  stringList

	c *vcx.Compiler
}

func (p *ppFlags) register(fs *flag.FlagSet) {
	fs.Var(&p.includes, "I", "add an include search directory (repeatable, in order)")
	fs.Var(&p.defs, "D", "define a macro (repeatable)")
	fs.Var(&p.undefs, "U", "undefine a macro (repeatable)")
	fs.StringVar(&p.target, "target", vcx.DefaultTarget().Name, "target to compile for")
	fs.StringVar(&p.std, "std", "c++23", "language standard: c++20, c++23, c++26; metal3.0 ... metal4.0 for a .metal file")
	fs.BoolVar(&p.freestanding, "freestanding", false, "freestanding environment (no library runtime)")
	fs.StringVar(&p.language, "x", "", "language of the inputs: c++, cuda, hip, metal (default: by extension)")
	fs.Var(&p.offloadArchs, "offload-arch", "device to compile kernels for: sm_75, gfx942, apple8, ... (repeatable; default sm_52 for CUDA, apple7 for Metal)")
	fs.Var(&p.offloadArchs, "arch", "same as --offload-arch")
	fs.BoolVar(&p.deviceOnly, "cuda-device-only", false, "compile only the device pass of a CUDA or HIP unit")
	fs.BoolVar(&p.deviceOnly, "offload-device-only", false, "same as --cuda-device-only")
	fs.BoolVar(&p.hostOnly, "cuda-host-only", false, "compile only the host pass of a CUDA or HIP unit")
	fs.BoolVar(&p.hostOnly, "offload-host-only", false, "same as --cuda-host-only")
	fs.StringVar(&p.cudart, "cudart", "", "the CUDA runtime to link: vcx, static, shared, none (default: static with a toolkit, else vcx)")
	fs.StringVar(&p.cudaPath, "cuda-path", "", "the CUDA toolkit to use (default: CUDA_PATH or the installed one; none for none)")
	fs.StringVar(&p.minOS, "mmacosx-version-min", "", "the oldest macOS a .metallib loads on (default 13.0)")
	fs.BoolVar(&p.noFastMath, "fno-fast-math", false, "precise float math in a .metal file (fast is the default)")
	fs.Bool("ffast-math", false, "fast float math in a .metal file (the default)")
	fs.Var(&p.moduleFiles, "fmodule-file", "the interface unit of a module the inputs import: name=path (repeatable)")
}

func (p *ppFlags) compiler() (*vcx.Compiler, error) {
	if p.c != nil {
		return p.c, nil
	}

	std := vcx.Cxx23
	var metalStd string
	switch s := strings.ToLower(p.std); {
	case strings.HasPrefix(s, "metal"):
		// A Metal version is the language of the .metal files; the C++
		// underneath is the compiler's own.
		metalStd = s
	case s == "c++20", s == "cxx20", s == "20":
		std = vcx.Cxx20
	case s == "c++23", s == "cxx23", s == "23":
		std = vcx.Cxx23
	case s == "c++26", s == "cxx26", s == "26":
		std = vcx.Cxx26
	default:
		return nil, fmt.Errorf("unknown standard %q (supported: c++20, c++23, c++26, metal3.0 ... metal4.0)", p.std)
	}

	lang, err := vcx.ParseLanguage(p.language)
	if err != nil {
		return nil, err
	}
	if p.deviceOnly && p.hostOnly {
		return nil, fmt.Errorf("--cuda-device-only and --cuda-host-only name no pass together")
	}

	var modules map[string]string
	for _, mf := range p.moduleFiles {
		name, file, ok := strings.Cut(mf, "=")
		if !ok || name == "" || file == "" {
			return nil, fmt.Errorf("-fmodule-file wants name=path, not %q", mf)
		}
		if modules == nil {
			modules = map[string]string{}
		}
		modules[name] = file
	}

	p.c = &vcx.Compiler{
		Modules:      modules,
		Target:       p.target,
		Std:          std,
		IncludeDirs:  p.includes,
		Defs:         p.defs,
		Undefs:       p.undefs,
		Freestanding: p.freestanding,
		Language:     lang,
		OffloadArchs: p.offloadArchs,
		CUDARuntime:  p.cudart,
		CUDAPath:     p.cudaPath,
		DeviceOnly:   p.deviceOnly,
		HostOnly:     p.hostOnly,
		MetalStd:     metalStd,
		MinOS:        p.minOS,
		NoFastMath:   p.noFastMath,
	}
	return p.c, nil
}

func input(path string) (vcx.Input, error) {
	if path == "" || path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return vcx.Input{}, err
		}
		return vcx.Text("<stdin>", data), nil
	}
	return vcx.File(path), nil
}
