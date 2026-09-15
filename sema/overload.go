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

	// Dependent is set when some argument was accepted only because its
	// type, or its parameter's, depends on a template parameter: the
	// candidate's rank is not known until instantiation.
	Dependent bool
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
	seen := map[*FuncSymbol]bool{}

	for _, fn := range candidates {
		if fn == nil || fn.FuncType == nil || seen[fn] {
			continue
		}
		seen[fn] = true
		// One member reached twice -- through the class's scope and through
		// the method table -- is one candidate, not two that tie.
		if fn.InClass != nil && sameMemberAsViable(fn, viable) {
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
		dependent := false

		for i, arg := range args {
			if i < numParams {
				paramType := ft.Params[i].Type
				cs := ClassifyConversion(arg.Type, paramType, arg.IsLValue)
				if isDependentType(paramType) || isDependentType(arg.Type) {
					// However the conversion ranks here, it is ranked again
					// against the types the instantiation supplies.
					dependent = true
				}
				if !cs.Valid && arg.NullConst && isPointerLike(paramType) {
					cs = ConversionSequence{From: arg.Type, To: paramType, Rank: RankConversion, Valid: true}
				}
				if !cs.Valid && (isDependentType(paramType) || isDependentType(arg.Type)) {
					// Inside a template, a parameter typed `size_type` --
					// `typename __alloc_traits::size_type` -- is whatever
					// the instantiation makes it; the call is checked again
					// then, against the type it has.
					cs = ConversionSequence{From: arg.Type, To: paramType, Rank: RankConversion, Valid: true}
					dependent = true
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
				Dependent:   dependent,
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
			if viable[best].Dependent || viable[i].Dependent {
				// Not an ambiguity yet: which candidate is better turns on
				// a type the instantiation has not supplied, and the call
				// is resolved again there.
				continue
			}
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

	// [over.match.best]/2.5: then the more specialized template.
	if c1.IsTemplate && c2.IsTemplate {
		p1, p2 := templatePattern(c1.Func), templatePattern(c2.Func)
		if p1 != nil && p2 != nil && p1 != p2 {
			first := atLeastAsSpecialized(p2, p1)
			second := atLeastAsSpecialized(p1, p2)
			if first && !second {
				return -1
			}
			if second && !first {
				return 1
			}
		}
	}

	return 0
}

// templatePattern is the function type of the template a candidate is, or
// is a specialization of, as written.
func templatePattern(fn *FuncSymbol) *types.Func {
	if fn.Template != nil && fn.Template.Pattern != nil {
		return fn.Template.Pattern
	}
	if fn.TemplateOf != nil {
		if fn.TemplateOf.Template != nil && fn.TemplateOf.Template.Pattern != nil {
			return fn.TemplateOf.Template.Pattern
		}
		return fn.TemplateOf.FuncType
	}
	if fn.Template != nil {
		return fn.FuncType
	}
	return nil
}

// atLeastAsSpecialized reports [temp.func.order]/3: whether the parameters
// of template p deduce from the parameter types of template a. libc++'s
// fallback `__nat __invoke(_Args&&...)` deduces from bullet 7's
// `(_Fp&&, _Args&&...)`, and not the reverse -- an argument that is a pack
// expansion does not deduce a parameter that is not one
// ([temp.deduct.type]/10) -- so bullet 7 is chosen.
func atLeastAsSpecialized(p, a *types.Func) bool {
	b := Binding{}
	for i, par := range p.Params {
		if par.Pack {
			for _, arg := range a.Params[min(i, len(a.Params)):] {
				if !deducePartial(par.Type, arg.Type, Binding{}) {
					return false
				}
			}
			return true
		}
		if i >= len(a.Params) {
			return par.HasDefault
		}
		if a.Params[i].Pack {
			return false
		}
		if !deducePartial(par.Type, a.Params[i].Type, b) {
			return false
		}
	}
	return len(a.Params) <= len(p.Params)
}

// deducePartial deduces P from A as [temp.deduct.partial]/5-7 says: top-level
// references and cv-qualifiers set aside.
func deducePartial(p, a types.Type, b Binding) bool {
	p = types.Unqualify(types.RemoveReference(p))
	a = types.Unqualify(types.RemoveReference(a))
	if !isDependent(p) {
		return !isDependent(a) && p.Equal(a)
	}
	return deduce(p, a, b)
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

// sameMemberAsViable reports whether a viable candidate already is fn: the
// same declaration reached a second way. Two members of one signature are
// not that -- a requires-clause tells `step() const` from
// `step() const requires Big<T>` -- so identity is by declaration, never by
// signature.
func sameMemberAsViable(fn *FuncSymbol, viable []ViableCandidate) bool {
	for _, v := range viable {
		if v.Func == fn || fn.Method != nil && v.Func.Method == fn.Method || fn.Decl != nil && v.Func.Decl == fn.Decl {
			return true
		}
	}
	return false
}
