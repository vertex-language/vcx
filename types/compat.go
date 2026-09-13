package types

// Type classification and standard C++23 relations.

// IsInteger reports whether t (unqualified) is an integral type (including enums).
func IsInteger(t Type) bool {
	if t == nil {
		return false
	}
	switch Unqualify(t).Kind() {
	case Bool, Char, SChar, UChar, Char8, Char16, Char32, WChar,
		Short, UShort, Int, UInt, Long, ULong, LongLong, ULongLong,
		Int128, UInt128, EnumKind:
		return true
	}
	return false
}

// IsSigned reports whether t is a signed integral or floating type.
func IsSigned(t Type) bool {
	if t == nil {
		return false
	}
	u := Unqualify(t)
	if e, ok := u.(*Enum); ok {
		if e.Underlying != nil {
			return IsSigned(e.Underlying)
		}
		return true
	}
	switch u.Kind() {
	case SChar, Short, Int, Long, LongLong, Int128, Float, Double, LongDouble:
		return true
	}
	return false
}

// IsUnsigned reports whether t is an unsigned integral type.
func IsUnsigned(t Type) bool {
	if t == nil {
		return false
	}
	u := Unqualify(t)
	if e, ok := u.(*Enum); ok {
		if e.Underlying != nil {
			return IsUnsigned(e.Underlying)
		}
		return false
	}
	switch u.Kind() {
	case Bool, UChar, Char8, Char16, Char32, UShort, UInt, ULong, ULongLong, UInt128:
		return true
	}
	return false
}

// IsFloat reports whether t is a floating-point type.
func IsFloat(t Type) bool {
	if t == nil {
		return false
	}
	switch Unqualify(t).Kind() {
	case Float, Double, LongDouble:
		return true
	}
	return false
}

// IsArithmetic reports whether t is an integer or floating-point type.
func IsArithmetic(t Type) bool {
	return IsInteger(t) || IsFloat(t)
}

// IsScalar reports whether t is a scalar type: arithmetic, enumeration, pointer,
// pointer to member, std::nullptr_t, or a cv-qualified version thereof.
func IsScalar(t Type) bool {
	if t == nil {
		return false
	}
	u := Unqualify(t)
	if IsArithmetic(u) || IsEnum(u) || IsPointer(u) {
		return true
	}
	switch u.(type) {
	case *MemberPointer:
		return true
	case *Basic:
		return u.Kind() == NullptrKind
	}
	return false
}

// IsPointer reports whether t is a pointer type.
func IsPointer(t Type) bool {
	if t == nil {
		return false
	}
	return Unqualify(t).Kind() == PointerKind
}

// IsReference reports whether t is an lvalue or rvalue reference.
func IsReference(t Type) bool {
	if t == nil {
		return false
	}
	k := t.Kind()
	return k == LValueReferenceKind || k == RValueReferenceKind
}

// IsLValueReference reports whether t is an lvalue reference (T&).
func IsLValueReference(t Type) bool {
	if t == nil {
		return false
	}
	return t.Kind() == LValueReferenceKind
}

// IsRValueReference reports whether t is an rvalue reference (T&&).
func IsRValueReference(t Type) bool {
	if t == nil {
		return false
	}
	return t.Kind() == RValueReferenceKind
}

// IsVoid reports whether t is void.
func IsVoid(t Type) bool {
	if t == nil {
		return false
	}
	return Unqualify(t).Kind() == Void
}

// IsBool reports whether t is bool.
func IsBool(t Type) bool {
	if t == nil {
		return false
	}
	return Unqualify(t).Kind() == Bool
}

// IsNullptr reports whether t is std::nullptr_t.
func IsNullptr(t Type) bool {
	if t == nil {
		return false
	}
	return Unqualify(t).Kind() == NullptrKind
}

// IsRecord reports whether t is a class, struct, or union.
func IsRecord(t Type) bool {
	if t == nil {
		return false
	}
	return Unqualify(t).Kind() == RecordKind
}

// IsClass reports whether t is a class or struct (not union).
func IsClass(t Type) bool {
	if r, ok := Unqualify(t).(*Record); ok {
		return r.Tag == TagClass || r.Tag == TagStruct
	}
	return false
}

// IsUnion reports whether t is a union.
func IsUnion(t Type) bool {
	if r, ok := Unqualify(t).(*Record); ok {
		return r.Tag == TagUnion
	}
	return false
}

// IsFunc reports whether t is a function type.
func IsFunc(t Type) bool {
	if t == nil {
		return false
	}
	return Unqualify(t).Kind() == FuncKind
}

// IsArray reports whether t is an array type.
func IsArray(t Type) bool {
	if t == nil {
		return false
	}
	return Unqualify(t).Kind() == ArrayKind
}

// IsEnum reports whether t is an enum type.
func IsEnum(t Type) bool {
	if t == nil {
		return false
	}
	return Unqualify(t).Kind() == EnumKind
}

// IsConst reports whether t has top-level const qualification.
func IsConst(t Type) bool {
	return QualsOf(t)&QConst != 0
}

// IsVolatile reports whether t has top-level volatile qualification.
func IsVolatile(t Type) bool {
	return QualsOf(t)&QVolatile != 0
}

// AsPointer returns t as *Pointer, or nil.
func AsPointer(t Type) *Pointer {
	p, _ := Unqualify(t).(*Pointer)
	return p
}

// AsArray returns t as *Array, or nil.
func AsArray(t Type) *Array {
	a, _ := Unqualify(t).(*Array)
	return a
}

// AsFunc returns t as *Func, or nil.
func AsFunc(t Type) *Func {
	f, _ := Unqualify(t).(*Func)
	return f
}

// AsRecord returns t as *Record, or nil.
func AsRecord(t Type) *Record {
	if t == nil {
		return nil
	}
	u := Unqualify(RemoveReference(t))
	if r, ok := u.(*Record); ok {
		return r
	}
	if ts, ok := u.(*TemplateSpecialization); ok {
		if r, ok := ts.Type.(*Record); ok {
			return r
		}
	}
	return nil
}

// AsEnum returns t as *Enum, or nil.
func AsEnum(t Type) *Enum {
	e, _ := Unqualify(t).(*Enum)
	return e
}

// RemoveReference strips lvalue or rvalue reference, returning the referenced type.
func RemoveReference(t Type) Type {
	if t == nil {
		return nil
	}
	switch r := t.(type) {
	case *LValueReference:
		return r.Elem
	case *RValueReference:
		return r.Elem
	}
	return t
}

// RemoveCV strips top-level cv-qualifiers.
func RemoveCV(t Type) Type {
	return Unqualify(t)
}

// RemoveCVRef removes references and then top-level cv-qualifiers (std::remove_cvref_t).
func RemoveCVRef(t Type) Type {
	return Unqualify(RemoveReference(t))
}

// AddLValueReference applies reference collapsing to form an lvalue reference.
func AddLValueReference(t Type) Type {
	if t == nil {
		return nil
	}
	if IsVoid(t) {
		return t
	}
	elem := RemoveReference(t)
	return &LValueReference{Elem: elem}
}

// AddRValueReference applies reference collapsing to form an rvalue reference.
func AddRValueReference(t Type) Type {
	if t == nil {
		return nil
	}
	if IsVoid(t) {
		return t
	}
	if IsLValueReference(t) {
		return t // & + && -> &
	}
	elem := RemoveReference(t)
	return &RValueReference{Elem: elem}
}

// AddConst applies const qualification to t.
func AddConst(t Type) Type {
	return Qualify(t, QConst)
}

// Decay applies C++ array-to-pointer and function-to-pointer conversion.
func Decay(t Type) Type {
	if t == nil {
		return nil
	}
	u := RemoveReference(t)
	switch v := u.(type) {
	case *Array:
		return &Pointer{Elem: v.Elem}
	case *Func:
		return &Pointer{Elem: v}
	}
	return Unqualify(u)
}

// IsBaseOf reports whether base is a base class of derived (directly or indirectly).
func IsBaseOf(base, derived Type) bool {
	if base == nil || derived == nil {
		return false
	}
	bRec, ok1 := Unqualify(base).(*Record)
	dRec, ok2 := Unqualify(derived).(*Record)
	if !ok1 || !ok2 || bRec == dRec {
		return false
	}
	return isBaseOfRec(bRec, dRec)
}

func isBaseOfRec(base, derived *Record) bool {
	for _, b := range derived.Bases {
		if bRec, ok := Unqualify(b.Type).(*Record); ok {
			if bRec == base {
				return true
			}
			if isBaseOfRec(base, bRec) {
				return true
			}
		}
	}
	return false
}

// IntegerRank returns the conversion rank of an integral type.
func IntegerRank(k Kind) int {
	switch k {
	case Bool:
		return 1
	case Char, SChar, UChar, Char8:
		return 2
	case Short, UShort:
		return 3
	case Char16:
		return 4
	case Int, UInt, WChar, Char32:
		return 5
	case Long, ULong:
		return 6
	case LongLong, ULongLong:
		return 7
	case Int128, UInt128:
		return 8
	}
	return 0
}

// CommonType computes the usual arithmetic conversions result type between two arithmetic types.
func CommonType(a, b Type) Type {
	if a == nil || b == nil {
		return nil
	}
	a = Unqualify(RemoveReference(a))
	b = Unqualify(RemoveReference(b))

	// If both are the exact same type
	if a.Equal(b) {
		return a
	}

	// Floating point conversions
	if a.Kind() == LongDouble || b.Kind() == LongDouble {
		return Typ(LongDouble)
	}
	if a.Kind() == Double || b.Kind() == Double {
		return Typ(Double)
	}
	if a.Kind() == Float || b.Kind() == Float {
		return Typ(Float)
	}

	// Integral promotions: promote types with rank < Int to Int
	promote := func(t Type) Type {
		if IsEnum(t) {
			if e, ok := t.(*Enum); ok && e.Underlying != nil {
				t = e.Underlying
			} else {
				t = Typ(Int)
			}
		}
		if IntegerRank(t.Kind()) < IntegerRank(Int) {
			return Typ(Int)
		}
		return t
	}

	a = promote(a)
	b = promote(b)

	if a.Equal(b) {
		return a
	}

	ra, rb := IntegerRank(a.Kind()), IntegerRank(b.Kind())
	sa, sb := IsSigned(a), IsSigned(b)

	if sa == sb {
		if ra > rb {
			return a
		}
		return b
	}

	// Mixed signedness
	var unsignedType, signedType Type
	if sa {
		signedType, unsignedType = a, b
	} else {
		signedType, unsignedType = b, a
	}

	if IntegerRank(unsignedType.Kind()) >= IntegerRank(signedType.Kind()) {
		return unsignedType
	}
	return signedType
}

// IsConvertible reports whether 'from' can be implicitly converted to 'to' via standard conversions.
func IsConvertible(from, to Type) bool {
	if from == nil || to == nil {
		return false
	}
	if from.Equal(to) {
		return true
	}

	// Reference binding
	if IsReference(to) {
		toElem := RemoveReference(to)
		fromElem := RemoveReference(from)

		// Exact match or base-derived match
		if fromElem.Equal(toElem) || IsBaseOf(toElem, fromElem) {
			// Check const correctness: if to is non-const lvalue reference, from must be compatible
			if IsLValueReference(to) && !IsConst(toElem) {
				return !IsConst(fromElem)
			}
			return true
		}
		// Const lvalue reference or rvalue reference can bind to temporaries / convertible values
		if IsConst(toElem) || IsRValueReference(to) {
			return IsConvertible(fromElem, toElem)
		}
		return false
	}

	// Non-reference target: unqualify both for value conversion
	f := Unqualify(RemoveReference(from))
	t := Unqualify(to)

	if f.Equal(t) {
		return true
	}

	// Arithmetic conversions
	if IsArithmetic(f) && IsArithmetic(t) {
		return true
	}

	// Pointer conversions
	if IsPointer(f) && IsPointer(t) {
		fElem := f.(*Pointer).Elem
		tElem := t.(*Pointer).Elem

		// Pointer to void (can add const/volatile)
		if IsVoid(tElem) {
			if IsConst(fElem) && !IsConst(tElem) {
				return false
			}
			return true
		}

		// Derived* to Base*
		if IsBaseOf(tElem, fElem) {
			if IsConst(fElem) && !IsConst(tElem) {
				return false
			}
			return true
		}

		// Qualification conversion
		if Unqualify(fElem).Equal(Unqualify(tElem)) {
			if IsConst(fElem) && !IsConst(tElem) {
				return false
			}
			return true
		}
	}

	// Nullptr to pointer or pointer-to-member or bool
	if IsNullptr(f) {
		if IsPointer(t) || t.Kind() == MemberPointerKind || IsBool(t) {
			return true
		}
	}

	// Array / function decay to pointer
	if IsArray(from) || IsFunc(from) {
		decayed := Decay(from)
		return IsConvertible(decayed, to)
	}

	// Conversion to bool from scalar
	if IsBool(t) && (IsArithmetic(f) || IsPointer(f) || IsNullptr(f) || IsEnum(f)) {
		return true
	}

	return false
}
