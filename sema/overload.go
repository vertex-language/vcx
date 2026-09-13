package sema

import (
	"fmt"

	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// Argument represents an evaluated expression argument to a call or operator.
type Argument struct {
	Type     types.Type
	IsLValue bool

	// NullConst marks an integer literal zero null pointer constant.
	NullConst bool
}

// ViableCandidate records a viable candidate function and its argument conversion sequences.
type ViableCandidate struct {
	Func        *FuncSymbol
	Conversions []ConversionSequence
	IsTemplate  bool
}

// ResolveOverload selects the best viable candidate from candidate funcs.
func ResolveOverload(candidates []*FuncSymbol, args []Argument) (*FuncSymbol, error) {
	return ResolveOverloadOn(candidates, nil, args)
}

// ResolveOverloadOn performs overload resolution, ranking candidates
// against the implicit object argument and explicit arguments.
func ResolveOverloadOn(candidates []*FuncSymbol, object *Argument, args []Argument) (*FuncSymbol, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no candidate functions")
	}

	var viable []ViableCandidate

	for _, fn := range candidates {
		if fn == nil || fn.FuncType == nil {
			continue
		}

		ft := fn.FuncType
		numParams := len(ft.Params)
		numArgs := len(args)

		var objectConv []ConversionSequence
		if object != nil {
			cs, ok := implicitObjectConversion(fn, *object)
			if !ok {
				continue
			}
			objectConv = []ConversionSequence{cs}
		}

		// Check parameter count
		if numArgs > numParams && !ft.Variadic {
			continue
		}
		if numArgs < numParams {
			// Check if missing parameters have default arguments
			hasDefaults := true
			for i := numArgs; i < numParams; i++ {
				if !ft.Params[i].HasDefault {
					hasDefaults = false
					break
				}
			}
			if !hasDefaults {
				continue
			}
		}

		// Check conversion sequence for each provided argument
		convs := make([]ConversionSequence, numArgs)
		allValid := true

		for i, arg := range args {
			if i < numParams {
				paramType := ft.Params[i].Type
				cs := ClassifyConversion(arg.Type, paramType, arg.IsLValue)
				if !cs.Valid && arg.NullConst && isPointerLike(paramType) {
					cs = ConversionSequence{From: arg.Type, To: paramType, Rank: RankConversion, Valid: true}
				}
				if !cs.Valid {
					allValid = false
					break
				}
				convs[i] = cs
			} else {
				// Variadic argument
				convs[i] = ConversionSequence{
					From:     arg.Type,
					Rank:     RankEllipsis,
					Valid:    true,
					Ellipsis: true,
				}
			}
		}

		if allValid {
			viable = append(viable, ViableCandidate{
				Func:        fn,
				Conversions: append(objectConv, convs...),
				IsTemplate:  fn.TemplateOf != nil || fn.Template != nil,
			})
		}
	}

	if len(viable) == 0 {
		return nil, fmt.Errorf("no viable candidates for call with %d arguments", len(args))
	}

	if len(viable) == 1 {
		return viable[0].Func, nil
	}

	// Rank viable candidates
	best := 0
	for i := 1; i < len(viable); i++ {
		cmp := compareCandidates(viable[i], viable[best])
		if cmp < 0 {
			best = i
		}
	}

	// Verify that the chosen best is strictly better than all other candidates
	for i := 0; i < len(viable); i++ {
		if i == best {
			continue
		}
		cmp := compareCandidates(viable[best], viable[i])
		if cmp >= 0 {
			return nil, fmt.Errorf("call to %q is ambiguous between candidate declarations", viable[best].Func.Name())
		}
	}

	return viable[best].Func, nil
}

// compareCandidates returns:
// -1 if c1 is strictly better than c2
//
//	1 if c2 is strictly better than c1
//	0 if tied or ambiguous
func compareCandidates(c1, c2 ViableCandidate) int {
	numArgs := len(c1.Conversions)
	if len(c2.Conversions) < numArgs {
		numArgs = len(c2.Conversions)
	}

	c1Better := false
	c2Better := false

	for i := 0; i < numArgs; i++ {
		cmp := CompareConversions(c1.Conversions[i], c2.Conversions[i])
		if cmp < 0 {
			c1Better = true
		} else if cmp > 0 {
			c2Better = true
		}
	}

	if c1Better && !c2Better {
		return -1
	}
	if c2Better && !c1Better {
		return 1
	}

	// Tie-breaking: non-template preferred over template
	if !c1.IsTemplate && c2.IsTemplate {
		return -1
	}
	if c1.IsTemplate && !c2.IsTemplate {
		return 1
	}

	return 0
}

// OperatorSpelling returns the function name for an overloaded operator.
func OperatorSpelling(tok token.Kind) string {
	switch tok {
	case token.ADD:
		return "operator+"
	case token.SUB:
		return "operator-"
	case token.MUL:
		return "operator*"
	case token.QUO:
		return "operator/"
	case token.REM:
		return "operator%"
	case token.AND:
		return "operator&"
	case token.OR:
		return "operator|"
	case token.XOR:
		return "operator^"
	case token.TILDE:
		return "operator~"
	case token.NOT:
		return "operator!"
	case token.ASSIGN:
		return "operator="
	case token.EQL:
		return "operator=="
	case token.NEQ:
		return "operator!="
	case token.LSS:
		return "operator<"
	case token.LEQ:
		return "operator<="
	case token.GTR:
		return "operator>"
	case token.GEQ:
		return "operator>="
	case token.SPACESHIP:
		return "operator<=>"
	case token.SHL:
		return "operator<<"
	case token.SHR:
		return "operator>>"
	case token.LPAREN:
		return "operator()"
	case token.LBRACK:
		return "operator[]"
	case token.ARROW:
		return "operator->"
	case token.INC:
		return "operator++"
	case token.DEC:
		return "operator--"
	}
	return ""
}

// isPointerLike reports whether a type takes a null pointer constant: a
// pointer, a pointer to member, or std::nullptr_t.
func isPointerLike(t types.Type) bool {
	switch types.Unqualify(t).(type) {
	case *types.Pointer, *types.MemberPointer:
		return true
	case *types.Basic:
		return types.Unqualify(t).Kind() == types.NullptrKind
	}
	return false
}

// implicitObjectConversion converts the object argument to the member function's
// implicit object parameter type based on cv- and ref-qualifiers.
func implicitObjectConversion(fn *FuncSymbol, object Argument) (ConversionSequence, bool) {
	if fn.Static || fn.InClass == nil {
		return ConversionSequence{From: object.Type, To: object.Type, Rank: RankExactMatch, Valid: true}, true
	}
	// An object whose class is not the member's, nor derived from it --
	// a template's as written, still spelled `X<T>` -- has no
	// conversion to rank; the cv-qualifiers still decide.
	rec := types.AsRecord(types.Unqualify(object.Type))
	if rec == nil || rec != fn.InClass && !types.IsBaseOf(fn.InClass, rec) {
		if types.IsConst(object.Type) && fn.FuncType.Quals&types.QConst == 0 && fn.FuncType.RefQual != types.RefQualRValue {
			return ConversionSequence{}, false
		}
		return ConversionSequence{From: object.Type, To: object.Type, Rank: RankExactMatch, Valid: true}, true
	}
	var param types.Type = types.Qualify(fn.InClass, fn.FuncType.Quals)
	isLValue := object.IsLValue
	switch fn.FuncType.RefQual {
	case types.RefQualRValue:
		param = types.AddRValueReference(param)
	case types.RefQualLValue:
		param = types.AddLValueReference(param)
	default:
		param = types.AddLValueReference(param)
		isLValue = true
	}
	// The object as its class, cv and all: a specialization spelled
	// `X<T>` is the class the record is, and converts as it.
	from := types.Qualify(rec, qualsOf(object.Type))
	cs := ClassifyConversion(from, param, isLValue)
	if !cs.Valid {
		return cs, false
	}
	return cs, true
}

// dependentArguments reports whether any argument's type is dependent.
func dependentArguments(args []Argument) bool {
	for _, arg := range args {
		if isDependentType(arg.Type) {
			return true
		}
	}
	return false
}
