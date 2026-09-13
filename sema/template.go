package sema

import (
	"github.com/vertex-language/vcx/types"
)

// DeduceFunctionTemplateArgs attempts to deduce template arguments from argument types.
func DeduceFunctionTemplateArgs(tmpl *TemplateSymbol, argTypes []types.Type) ([]types.TemplateArg, bool) {
	if tmpl == nil {
		return nil, false
	}
	deduced := make([]types.TemplateArg, len(tmpl.Params))

	// Simple 1-to-1 type deduction for single parameter templates
	if len(tmpl.Params) == 1 && len(argTypes) >= 1 {
		deduced[0] = types.TemplateArg{
			IsType: true,
			Type:   types.Decay(argTypes[0]),
		}
		return deduced, true
	}

	for i := range tmpl.Params {
		if i < len(argTypes) {
			deduced[i] = types.TemplateArg{
				IsType: true,
				Type:   types.Decay(argTypes[i]),
			}
		}
	}

	return deduced, true
}

// InstantiateClassTemplate instantiates a class template with concrete arguments.
func InstantiateClassTemplate(tmpl *TemplateSymbol, args []types.TemplateArg) (*types.Record, error) {
	// Produces an instantiated record type with the template name and specialized args
	instName := tmpl.Name()
	return &types.Record{
		Tag:      types.TagStruct,
		Name:     instName,
		Complete: true,
	}, nil
}
