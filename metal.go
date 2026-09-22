package vcx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vertex-language/air"
	"github.com/vertex-language/air/metallib"
)

// A .metal file builds to a Metal library rather than an object: there is
// no host code in it, so nothing to link, and what comes out is the
// .metallib an app loads with newLibraryWithURL:, as xcrun's metal and
// metallib make one. Several .metal files make one library holding every
// kernel, as `xcrun metallib a.air b.air` does.

// DefaultMetallib is the library a build of .metal files writes when -o
// names none: the name Xcode gives an app's own, which
// newDefaultLibrary finds.
const DefaultMetallib = "default.metallib"

// metalInputs reports whether the build is of .metal files, which are
// built alone: a library and an executable are not one output.
func (c *Compiler) metalInputs(ins []Input) (bool, error) {
	n := 0
	for _, in := range ins {
		if in.isSource() && c.language(in) == LangMetal {
			n++
		}
	}
	switch {
	case n == 0:
		return false, nil
	case n != len(ins):
		return false, fmt.Errorf("a .metal file builds to a .metallib, and the other inputs to a program; build them separately")
	}
	return true, nil
}

// buildMetal compiles every .metal input and writes the library: one for
// all of them, or with -c one per file.
func (c *Compiler) buildMetal(params BuildParams) error {
	var libs []Input
	for _, in := range params.Inputs {
		lib, diags, err := c.Object(in)
		if err != nil {
			return fmt.Errorf("%s: %w", in.name(), err)
		}
		if HasErrors(diags) {
			return &DiagnosticError{Diagnostics: diags}
		}
		libs = append(libs, ObjectBytes(metallibName(in), lib))
	}
	if params.CompileOnly {
		return c.writeObjects(params, libs)
	}
	out := params.Output
	if out == "" {
		out = DefaultMetallib
	}
	data := libs[0].Data
	if len(libs) > 1 {
		var err error
		if data, err = mergeMetallibs(libs); err != nil {
			return err
		}
	}
	return os.WriteFile(out, data, 0o644)
}

// mergeMetallibs is one library holding every function of the ones given.
// They were built with the same options, so their headers agree, and a
// name defined twice is refused, as Metal could find only one of them.
func mergeMetallibs(libs []Input) ([]byte, error) {
	var out *metallib.Library
	from := map[string]string{}
	for _, in := range libs {
		lib, err := metallib.Read(in.Data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", in.Name, err)
		}
		for _, f := range lib.Functions {
			if prev, dup := from[f.Name]; dup {
				return nil, fmt.Errorf("kernel %s is defined in both %s and %s", f.Name, strings.TrimSuffix(prev, ".metallib"), strings.TrimSuffix(in.Name, ".metallib"))
			}
			from[f.Name] = in.Name
		}
		if out == nil {
			out = lib
			continue
		}
		out.Functions = append(out.Functions, lib.Functions...)
	}
	return metallib.Write(out)
}

// AIR is a .metal input lowered to an AIR module: what --emit air and
// --emit ll write, one module holding every kernel, for reading or for
// xcrun's tools.
func (c *Compiler) AIR(in Input) (*air.Module, []Diagnostic, error) {
	if c.language(in) != LangMetal {
		return nil, nil, fmt.Errorf("%s is not a .metal file; AIR is what a .metal file lowers to", in.name())
	}
	mod, diags, err := c.IR(in)
	if err != nil || mod == nil || HasErrors(diags) {
		return nil, diags, err
	}
	p, err := c.passFor(in)
	if err != nil {
		return nil, diags, err
	}
	am, err := airModule(mod, p.arch)
	return am, diags, err
}

// metallibName is where a .metal input's own library goes under -c.
func metallibName(in Input) string {
	return strings.TrimSuffix(filepath.Base(in.name()), filepath.Ext(in.name())) + ".metallib"
}
