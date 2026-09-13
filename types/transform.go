package types

// Type-transformation builtins: compiler-implemented <type_traits> transformations
// (such as __decay, __remove_reference_t) evaluated directly without template instantiation.
// If applied to a dependent type, they produce a Transform node evaluated at substitution time.

// Transform is a type-transformation builtin whose operand is dependent.
type Transform struct {
	Op  string
	Arg Type
}

func (*Transform) Kind() Kind { return TransformKind }

func (t *Transform) Equal(other Type) bool {
	o, ok := other.(*Transform)
	return ok && o.Op == t.Op && o.Arg.Equal(t.Arg)
}

func (t *Transform) String() string { return t.Op + "(" + t.Arg.String() + ")" }

// ApplyTransform is op applied to t, or a Transform when dependent says
// t is not yet a type the transformation can see through. It reports false
// for a name that is not a transformation.
func ApplyTransform(op string, t Type, dependent bool) (Type, bool) {
	if t == nil {
		return nil, false
	}
	if dependent {
		return &Transform{Op: op, Arg: t}, true
	}
	switch op {
	case "__remove_reference_t", "__remove_reference":
		return RemoveReference(t), true
	case "__remove_cv":
		return Unqualify(t), true
	case "__remove_const":
		return withQuals(t, func(q Qual) Qual { return q &^ QConst }), true
	case "__remove_volatile":
		return withQuals(t, func(q Qual) Qual { return q &^ QVolatile }), true
	case "__remove_cvref":
		return Unqualify(RemoveReference(t)), true
	case "__remove_pointer":
		if p, ok := Unqualify(t).(*Pointer); ok {
			return p.Elem, true
		}
		return t, true
	case "__add_pointer":
		if !referenceable(t) && !IsVoid(Unqualify(t)) {
			return t, true
		}
		return &Pointer{Elem: RemoveReference(t)}, true
	case "__add_lvalue_reference":
		if !referenceable(t) {
			return t, true
		}
		return AddLValueReference(t), true
	case "__add_rvalue_reference":
		if !referenceable(t) {
			return t, true
		}
		return AddRValueReference(t), true
	case "__decay":
		u := Unqualify(RemoveReference(t))
		switch x := u.(type) {
		case *Array:
			return &Pointer{Elem: x.Elem}, true
		case *Func:
			return &Pointer{Elem: x}, true
		}
		return u, true
	case "__remove_extent":
		if a, ok := Unqualify(t).(*Array); ok {
			return a.Elem, true
		}
		return t, true
	case "__remove_all_extents":
		for {
			a, ok := Unqualify(t).(*Array)
			if !ok {
				return t, true
			}
			t = a.Elem
		}
	case "__underlying_type":
		if e, ok := Unqualify(t).(*Enum); ok {
			if e.Underlying != nil {
				return e.Underlying, true
			}
			return Typ(Int), true
		}
		return nil, true
	case "__make_signed", "__make_unsigned":
		return makeSignedness(t, op == "__make_signed"), true
	}
	return nil, false
}

// withQuals rebuilds t with its top-level qualifiers changed.
func withQuals(t Type, f func(Qual) Qual) Type {
	if q, ok := t.(*Qualified); ok {
		return Qualify(q.T, f(q.Q))
	}
	return t
}

// referenceable reports whether a reference to t can be formed: an
// object type, a reference, or a function type without cv or ref qualifiers.
func referenceable(t Type) bool {
	switch x := Unqualify(t).(type) {
	case *Basic:
		return x.K != Void
	case *Func:
		return x.Quals == 0 && x.RefQual == RefQualNone
	}
	return true
}

// makeSignedness implements make_signed and make_unsigned: the integer
// type of the same rank with the other signedness, keeping cv; for an enum
// or a character type, the smallest of that signedness with its size.
func makeSignedness(t Type, signed bool) Type {
	var q Qual
	if qt, ok := t.(*Qualified); ok {
		q, t = qt.Q, qt.T
	}
	var k Kind
	switch x := t.(type) {
	case *Basic:
		k = x.K
	case *Enum:
		if x.Underlying == nil {
			k = Int
		} else if b, ok := Unqualify(x.Underlying).(*Basic); ok {
			k = b.K
		}
	default:
		return nil
	}
	pairs := [][2]Kind{
		{SChar, UChar}, {Short, UShort}, {Int, UInt}, {Long, ULong}, {LongLong, ULongLong}, {Int128, UInt128},
	}
	switch k {
	case Char:
		k = SChar
	case Char8:
		k = UChar
	case Char16:
		k = UShort
	case Char32, WChar:
		k = UInt
	}
	for _, p := range pairs {
		if k == p[0] || k == p[1] {
			if signed {
				return Qualify(Typ(p[0]), q)
			}
			return Qualify(Typ(p[1]), q)
		}
	}
	return nil
}
