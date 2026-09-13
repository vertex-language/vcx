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

	c *vcx.Compiler
}

func (p *ppFlags) register(fs *flag.FlagSet) {
	fs.Var(&p.includes, "I", "add an include search directory (repeatable, in order)")
	fs.Var(&p.defs, "D", "define a macro (repeatable)")
	fs.Var(&p.undefs, "U", "undefine a macro (repeatable)")
	fs.StringVar(&p.target, "target", vcx.DefaultTarget().Name, "target to compile for")
	fs.StringVar(&p.std, "std", "c++23", "language standard: c++20, c++23, c++26")
	fs.BoolVar(&p.freestanding, "freestanding", false, "freestanding environment (no library runtime)")
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

	p.c = &vcx.Compiler{
		Target:       p.target,
		Std:          std,
		IncludeDirs:  p.includes,
		Defs:         p.defs,
		Undefs:       p.undefs,
		Freestanding: p.freestanding,
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
