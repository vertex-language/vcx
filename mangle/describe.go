package mangle

import (
	"strings"

	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// Describe reads a function's symbol into what a mangler needs.
//
// This is the one place the analysis's view of a function is translated
// into the mangler's, so that lowering and `v++ symbols` cannot disagree
// about what a function is called: both ask here.
func Describe(fn *sema.FuncSymbol) *Function {
	f := &Function{
		Scopes:       Scopes(fn.SymScope.Path()...),
		Name:         fn.SymName,
		Type:         fn.FuncType,
		Static:       fn.Static,
		Virtual:      fn.Virtual,
		Access:       fn.Access,
		ExternC:      fn.ExternC,
		TemplateArgs: fn.TemplateArgs,
	}
	if fn.LinkName != "" {
		f.Name, f.ExternC = fn.LinkName, true
	}
	if fn.TemplateArgs != nil && fn.TemplateOf != nil {
		f.Pattern = fn.TemplateOf.FuncType
	}
	if fn.InClass != nil {
		// A member's scopes are its class's, whether the definition sat
		// inside the class or was written out of line with a qualified
		// name; the scope it was declared in says where the text was, not
		// where the function is.
		f.Scopes = recordScopes(fn.InClass)
		f.Member = !fn.Static
		switch {
		case fn.SymName == fn.InClass.Name:
			f.Kind = Ctor
		case fn.SymName == "~"+fn.InClass.Name:
			f.Kind = Dtor
		case strings.HasPrefix(fn.SymName, "operator "):
			// A conversion function is named for its target type.
			f.Kind = Conversion
			f.Conv = fn.FuncType.Ret
		}
	}
	return f
}

// DescribeVariable reads an object's symbol into what a mangler needs.
func DescribeVariable(v *sema.VarSymbol, inClass *types.Record) *Variable {
	d := &Variable{
		Scopes:  Scopes(v.SymScope.Path()...),
		Name:    v.SymName,
		Type:    v.SymType,
		ExternC: v.ExternC,
	}
	if inClass != nil {
		d.Scopes = recordScopes(inClass)
		d.Static = true
	} else if v.InClass != nil {
		d.Scopes = recordScopes(v.InClass)
		d.Static = true
	}
	return d
}
