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
	deviceOnly  bool
	hostOnly    bool

	c *vcx.Compiler
}

func (p *ppFlags) register(fs *flag.FlagSet) {
	fs.Var(&p.includes, "I", "add an include search directory (repeatable, in order)")
	fs.Var(&p.defs, "D", "define a macro (repeatable)")
	fs.Var(&p.undefs, "U", "undefine a macro (repeatable)")
	fs.StringVar(&p.target, "target", vcx.DefaultTarget().Name, "target to compile for")
	fs.StringVar(&p.std, "std", "c++23", "language standard: c++20, c++23, c++26")
	fs.BoolVar(&p.freestanding, "freestanding", false, "freestanding environment (no library runtime)")
	fs.StringVar(&p.language, "x", "", "language of the inputs: c++, cuda, hip (default: by extension)")
	fs.Var(&p.offloadArchs, "offload-arch", "device to compile kernels for: sm_75, gfx942, ... (repeatable; default sm_52 for CUDA)")
	fs.Var(&p.offloadArchs, "arch", "same as --offload-arch")
	fs.BoolVar(&p.deviceOnly, "cuda-device-only", false, "compile only the device pass of a CUDA or HIP unit")
	fs.BoolVar(&p.deviceOnly, "offload-device-only", false, "same as --cuda-device-only")
	fs.BoolVar(&p.hostOnly, "cuda-host-only", false, "compile only the host pass of a CUDA or HIP unit")
	fs.BoolVar(&p.hostOnly, "offload-host-only", false, "same as --cuda-host-only")
}

func (p *ppFlags) compiler() (*vcx.Compiler, error) {
	if p.c != nil {
		return p.c, nil
	}

	std := vcx.Cxx23
	switch strings.ToLower(p.std) {
	case "c++20", "cxx20", "20":
		std = vcx.Cxx20
	case "c++23", "cxx23", "23":
		std = vcx.Cxx23
	case "c++26", "cxx26", "26":
		std = vcx.Cxx26
	default:
		return nil, fmt.Errorf("unknown standard %q (supported: c++20, c++23, c++26)", p.std)
	}

	lang, err := vcx.ParseLanguage(p.language)
	if err != nil {
		return nil, err
	}
	if p.deviceOnly && p.hostOnly {
		return nil, fmt.Errorf("--cuda-device-only and --cuda-host-only name no pass together")
	}

	p.c = &vcx.Compiler{
		Target:       p.target,
		Std:          std,
		IncludeDirs:  p.includes,
		Defs:         p.defs,
		Undefs:       p.undefs,
		Freestanding: p.freestanding,
		Language:     lang,
		OffloadArchs: p.offloadArchs,
		DeviceOnly:   p.deviceOnly,
		HostOnly:     p.hostOnly,
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
