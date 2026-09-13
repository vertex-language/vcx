package sema

import (
	"github.com/vertex-language/vcx/types"
)

// ConvRank identifies the standard conversion sequence ranking level.
type ConvRank uint8

const (
	RankExactMatch  ConvRank = iota // Identity, lvalue-to-rvalue, qualification, decay
	RankPromotion                   // Integral / floating promotion
	RankConversion                  // Standard integral/float/pointer/bool conversions
	RankUserDefined                 // User-defined conversion via ctor or operator
	RankEllipsis                    // Ellipsis match (...)
	RankNone                        // No viable conversion
)

func (r ConvRank) String() string {
	switch r {
	case RankExactMatch:
		return "exact match"
	case RankPromotion:
		return "promotion"
	case RankConversion:
		return "conversion"
	case RankUserDefined:
		return "user-defined conversion"
	case RankEllipsis:
		return "ellipsis"
	case RankNone:
		return "none"
	}
	return "unknown"
}

// ConversionSequence models an implicit conversion sequence in C++.
type ConversionSequence struct {
	From        types.Type
	To          types.Type
	Rank        ConvRank
	Valid       bool
	UserDefined bool
	UserConv    *FuncSymbol
	Ellipsis    bool
}

// ClassifyConversion evaluates the conversion sequence from type 'from' to 'to'.
func ClassifyConversion(from, to types.Type, fromIsLValue bool) ConversionSequence {
	return classifyConversion(from, to, fromIsLValue, true)
}

// classifyConversion evaluates conversions, allowing at most one user-defined conversion.
func classifyConversion(from, to types.Type, fromIsLValue, allowUser bool) ConversionSequence {
	if from == nil || to == nil {
		return ConversionSequence{Rank: RankNone, Valid: false}
	}

	// 1. Reference binding
	if types.IsReference(to) {
		toElem := types.RemoveReference(to)
		fromElem := types.RemoveReference(from)

		// Reference to same type or base class
		if fromElem.Equal(toElem) || types.IsBaseOf(toElem, fromElem) {
			if types.IsLValueReference(to) {
				if fromIsLValue {
					// Direct reference binding to lvalue
					if !types.IsConst(toElem) && types.IsConst(fromElem) {
						return ConversionSequence{Rank: RankNone, Valid: false} // Discards const
					}
					rank := RankExactMatch
					if types.IsBaseOf(toElem, fromElem) {
						rank = RankConversion
					}
					return ConversionSequence{From: from, To: to, Rank: rank, Valid: true}
				}
				// Cannot bind non-const lvalue reference to rvalue
				if !types.IsConst(toElem) {
					return ConversionSequence{Rank: RankNone, Valid: false}
				}
				// Const lvalue ref can bind to rvalue
				return ConversionSequence{From: from, To: to, Rank: RankExactMatch, Valid: true}
			}

			// RValue reference
			if types.IsRValueReference(to) {
				if fromIsLValue {
					// Cannot bind rvalue reference to lvalue directly
					return ConversionSequence{Rank: RankNone, Valid: false}
				}
				rank := RankExactMatch
				if types.IsBaseOf(toElem, fromElem) {
					rank = RankConversion
				}
				return ConversionSequence{From: from, To: to, Rank: rank, Valid: true}
			}
		}

		// Const lvalue ref or rvalue ref can bind to a converted temporary
		if (types.IsLValueReference(to) && types.IsConst(toElem)) || types.IsRValueReference(to) {
			subCS := classifyConversion(from, toElem, fromIsLValue, allowUser)
			if subCS.Valid {
				return ConversionSequence{From: from, To: to, Rank: subCS.Rank, Valid: true, UserDefined: subCS.UserDefined, UserConv: subCS.UserConv}
			}
		}

		return ConversionSequence{Rank: RankNone, Valid: false}
	}

	// 2. Non-reference: unqualify value types
	f := types.Unqualify(types.RemoveReference(from))
	t := types.Unqualify(to)

	// Exact match
	if f.Equal(t) {
		return ConversionSequence{From: from, To: to, Rank: RankExactMatch, Valid: true}
	}

	// Array / function decay
	if types.IsArray(from) || types.IsFunc(from) {
		decayed := types.Decay(from)
		if decayed.Equal(t) {
			return ConversionSequence{From: from, To: to, Rank: RankExactMatch, Valid: true}
		}
		// Continue checking with decayed type
		f = decayed
	}

	// Promotions
	if isPromotion(f, t) {
		return ConversionSequence{From: from, To: to, Rank: RankPromotion, Valid: true}
	}

	// Standard conversions
	if isStandardConversion(f, t) {
		return ConversionSequence{From: from, To: to, Rank: RankConversion, Valid: true}
	}

	// User-defined conversions (converting constructor or conversion operator)
	if allowUser {
		if cs, ok := findUserDefinedConversion(from, to); ok {
			return cs
		}
	}

	return ConversionSequence{Rank: RankNone, Valid: false}
}

func isPromotion(from, to types.Type) bool {
	fk := from.Kind()
	tk := to.Kind()

	// Floating-point promotion: float -> double
	if fk == types.Float && tk == types.Double {
		return true
	}

	// Integral promotions
	if types.IsInteger(from) && tk == types.Int {
		switch fk {
		case types.Bool, types.Char, types.SChar, types.UChar, types.Short, types.UShort, types.Char8, types.Char16:
			return true
		case types.EnumKind:
			return true
		}
	}

	return false
}

func isStandardConversion(from, to types.Type) bool {
	// Arithmetic conversions
	if types.IsArithmetic(from) && types.IsArithmetic(to) {
		return true
	}

	// Pointer conversions
	if types.IsPointer(from) && types.IsPointer(to) {
		fElem := types.AsPointer(from).Elem
		tElem := types.AsPointer(to).Elem

		// T* to void* (can add const)
		if types.IsVoid(tElem) {
			if types.IsConst(fElem) && !types.IsConst(tElem) {
				return false
			}
			return true
		}

		// Derived* to Base*
		if types.IsBaseOf(tElem, fElem) {
			if types.IsConst(fElem) && !types.IsConst(tElem) {
				return false
			}
			return true
		}

		// Qualification conversions: T* to const T*
		if types.Unqualify(fElem).Equal(types.Unqualify(tElem)) {
			if types.IsConst(fElem) && !types.IsConst(tElem) {
				return false
			}
			return true
		}
	}

	// Nullptr conversions
	if types.IsNullptr(from) {
		if types.IsPointer(to) || to.Kind() == types.MemberPointerKind || types.IsBool(to) {
			return true
		}
	}

	// Boolean conversions (from arithmetic or pointer)
	if types.IsBool(to) && (types.IsArithmetic(from) || types.IsPointer(from) || types.IsEnum(from)) {
		return true
	}

	return false
}

func findUserDefinedConversion(from, to types.Type) (ConversionSequence, bool) {
	// Check converting constructor in 'to': to::to(from)
	if toRec, ok := types.Unqualify(to).(*types.Record); ok {
		for _, m := range toRec.Methods {
			if m.Name == toRec.Name && !m.Explicit && len(m.Func.Params) == 1 {
				paramT := m.Func.Params[0].Type
				if m.Template {
					// Deduce constructor template parameter from argument type.
					b := Binding{}
					if !deduceArg(paramT, Argument{Type: from}, b) {
						continue
					}
					paramT = substitute(paramT, b)
					if isDependentType(paramT) {
						continue
					}
				}
				subCS := classifyConversion(from, paramT, false, false)
				if subCS.Valid && subCS.Rank <= RankConversion {
					return ConversionSequence{
						From:        from,
						To:          to,
						Rank:        RankUserDefined,
						Valid:       true,
						UserDefined: true,
						UserConv:    &FuncSymbol{SymName: m.Name, FuncType: m.Func, InClass: toRec},
					}, true
				}
			}
		}
	}

	// Check conversion operator in 'from': operator to()
	if fromRec, ok := types.Unqualify(from).(*types.Record); ok {
		for _, m := range fromRec.Methods {
			if !m.Explicit && m.Func.Ret.Equal(to) {
				return ConversionSequence{
					From:        from,
					To:          to,
					Rank:        RankUserDefined,
					Valid:       true,
					UserDefined: true,
					UserConv:    &FuncSymbol{SymName: m.Name, FuncType: m.Func, InClass: fromRec},
				}, true
			}
		}
	}

	return ConversionSequence{}, false
}

// CompareConversions compares two conversion sequences for the same argument.
// Returns -1 if cs1 is better, 1 if cs2 is better, 0 if indistinguishable.
func CompareConversions(cs1, cs2 ConversionSequence) int {
	if !cs1.Valid && !cs2.Valid {
		return 0
	}
	if cs1.Valid && !cs2.Valid {
		return -1
	}
	if !cs1.Valid && cs2.Valid {
		return 1
	}

	if cs1.Rank < cs2.Rank {
		return -1
	}
	if cs1.Rank > cs2.Rank {
		return 1
	}

	// Tie-breaking within same rank
	// 1. Reference binding: binding to lvalue vs rvalue
	if types.IsReference(cs1.To) && types.IsReference(cs2.To) {
		ref1L := types.IsLValueReference(cs1.To)
		ref2L := types.IsLValueReference(cs2.To)
		if ref1L != ref2L {
			// If argument is rvalue, rvalue reference is better
			if types.IsRValueReference(cs1.From) {
				if !ref1L {
					return -1
				}
				return 1
			}
		}
	}

	// 2. Qualification: fewer added qualifiers is better
	q1 := types.QualsOf(types.RemoveReference(cs1.To))
	q2 := types.QualsOf(types.RemoveReference(cs2.To))
	if q1 != q2 {
		if q1&types.QConst == 0 && q2&types.QConst != 0 {
			return -1
		}
		if q1&types.QConst != 0 && q2&types.QConst == 0 {
			return 1
		}
	}

	return 0
}
