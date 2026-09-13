// Package types represents C++23 types, layout models, and type relations.
package types

import (
	"fmt"
	"strconv"
	"strings"
)

// Kind identifies a type's shape or category.
type Kind uint8

const (
	Invalid Kind = iota

	// Primitive / fundamental types
	Void
	Bool
	Char
	SChar
	UChar
	Char8
	Char16
	Char32
	WChar
	Short
	UShort
	Int
	UInt
	Long
	ULong
	LongLong
	ULongLong
	Int128
	UInt128
	Float
	Double
	LongDouble
	NullptrKind

	// Placeholders
	AutoKind
	DecltypeAutoKind

	// Compound & derived types
	PointerKind
	LValueReferenceKind
	RValueReferenceKind
	MemberPointerKind
	ArrayKind
	FuncKind
	RecordKind
	EnumKind

	// Template types
	TemplateParamKind
	TemplateSpecializationKind
	DependentKind
	PackKind
	TemplateRefKind
	TransformKind
)

// Type is the interface all C++ types implement.
type Type interface {
	Kind() Kind
	String() string
	Equal(other Type) bool
}

// Basic represents a fundamental type, void, nullptr_t, or auto placeholder.
type Basic struct {
	K Kind
}

var basics [DecltypeAutoKind + 1]Basic

func init() {
	for k := range basics {
		basics[k].K = Kind(k)
	}
}

// Typ returns the canonical *Basic singleton for a fundamental kind.
func Typ(k Kind) *Basic {
	if int(k) < len(basics) {
		return &basics[k]
	}
	return &Basic{K: k}
}

func (b *Basic) Kind() Kind { return b.K }

func (b *Basic) Equal(other Type) bool {
	if other == nil {
		return false
	}
	o, ok := Unqualify(other).(*Basic)
	return ok && b.K == o.K
}

// Qual represents cv-qualifiers (const, volatile).
type Qual uint8

const (
	QConst Qual = 1 << iota
	QVolatile
)

// Qualified wraps a Type with cv-qualifiers.
type Qualified struct {
	Q Qual
	T Type
}

func (q *Qualified) Kind() Kind { return q.T.Kind() }

func (q *Qualified) Equal(other Type) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*Qualified); ok {
		return q.Q == o.Q && q.T.Equal(o.T)
	}
	if q.Q == 0 {
		return q.T.Equal(other)
	}
	return false
}

// Qualify applies cv-qualifiers, flattening any existing Qualified wrapper.
func Qualify(t Type, q Qual) Type {
	if t == nil || q == 0 {
		return t
	}
	// References cannot be cv-qualified in C++.
	if IsReference(t) {
		return t
	}
	// Functions cannot be cv-qualified outside of member function types.
	if IsFunc(t) {
		return t
	}
	if in, ok := t.(*Qualified); ok {
		return &Qualified{Q: in.Q | q, T: in.T}
	}
	return &Qualified{Q: q, T: t}
}

// Unqualify strips top-level cv-qualifiers.
func Unqualify(t Type) Type {
	if q, ok := t.(*Qualified); ok {
		return q.T
	}
	return t
}

// QualsOf returns the top-level qualifiers of t.
func QualsOf(t Type) Qual {
	if q, ok := t.(*Qualified); ok {
		return q.Q
	}
	return 0
}

// Pointer represents a pointer-to-Elem (T*).
type Pointer struct {
	Elem Type
}

func (*Pointer) Kind() Kind { return PointerKind }

func (p *Pointer) Equal(other Type) bool {
	if other == nil {
		return false
	}
	o, ok := Unqualify(other).(*Pointer)
	return ok && (p.Elem == nil && o.Elem == nil || (p.Elem != nil && p.Elem.Equal(o.Elem)))
}

// LValueReference represents an lvalue reference (T&).
type LValueReference struct {
	Elem Type
}

func (*LValueReference) Kind() Kind { return LValueReferenceKind }

func (r *LValueReference) Equal(other Type) bool {
	if other == nil {
		return false
	}
	o, ok := Unqualify(other).(*LValueReference)
	return ok && (r.Elem == nil && o.Elem == nil || (r.Elem != nil && r.Elem.Equal(o.Elem)))
}

// RValueReference represents an rvalue reference (T&&).
type RValueReference struct {
	Elem Type
}

func (*RValueReference) Kind() Kind { return RValueReferenceKind }

func (r *RValueReference) Equal(other Type) bool {
	if other == nil {
		return false
	}
	o, ok := Unqualify(other).(*RValueReference)
	return ok && (r.Elem == nil && o.Elem == nil || (r.Elem != nil && r.Elem.Equal(o.Elem)))
}

// MemberPointer represents a pointer-to-member (T Class::*).
type MemberPointer struct {
	Class Type
	Elem  Type
}

func (*MemberPointer) Kind() Kind { return MemberPointerKind }

func (m *MemberPointer) Equal(other Type) bool {
	if other == nil {
		return false
	}
	o, ok := Unqualify(other).(*MemberPointer)
	return ok && m.Class.Equal(o.Class) && m.Elem.Equal(o.Elem)
}

// Array represents an array type (T[N] or T[]).
type Array struct {
	Elem       Type
	Len        int64
	Incomplete bool

	// DepLen names the template parameter the bound is written as --
	// `T[N]` in a partial specialization's pattern -- when it has no
	// value yet. Such an array is dependent, and deduction against it
	// binds the parameter to the argument's length.
	DepLen string
}

func (*Array) Kind() Kind { return ArrayKind }

func (a *Array) Equal(other Type) bool {
	if other == nil {
		return false
	}
	o, ok := Unqualify(other).(*Array)
	if !ok || !a.Elem.Equal(o.Elem) {
		return false
	}
	if a.Incomplete != o.Incomplete || a.DepLen != o.DepLen {
		return false
	}
	return a.Incomplete || a.DepLen != "" || a.Len == o.Len
}

// RefQual indicates reference qualification on member functions.
type RefQual uint8

const (
	RefQualNone   RefQual = iota
	RefQualLValue         // &
	RefQualRValue         // &&
)

// Param represents a function parameter.
type Param struct {
	Name       string
	Type       Type
	HasDefault bool

	// Pack marks a parameter expansion; PackOf names the origin pack.
	Pack   bool
	PackOf string
}

// Func represents a function type.
//
// EmptyPacks names the parameter packs that expanded to no parameter at
// all, which leave no Param to carry their name and are still names in
// the function's body.
type Func struct {
	Ret        Type
	Params     []Param
	Variadic   bool
	Quals      Qual
	RefQual    RefQual
	Noexcept   bool
	EmptyPacks []string
}

func (*Func) Kind() Kind { return FuncKind }

func (f *Func) Equal(other Type) bool {
	if other == nil {
		return false
	}
	o, ok := Unqualify(other).(*Func)
	if !ok || f.Variadic != o.Variadic || f.Quals != o.Quals || f.RefQual != o.RefQual || f.Noexcept != o.Noexcept {
		return false
	}
	if !f.Ret.Equal(o.Ret) || len(f.Params) != len(o.Params) {
		return false
	}
	for i := range f.Params {
		if !f.Params[i].Type.Equal(o.Params[i].Type) {
			return false
		}
	}
	return true
}

// TagKind distinguishes class, struct, or union.
type TagKind uint8

const (
	TagClass TagKind = iota
	TagStruct
	TagUnion
)

// Access represents member access level.
type Access uint8

const (
	AccessPublic Access = iota
	AccessProtected
	AccessPrivate
)

func (a Access) String() string {
	switch a {
	case AccessPublic:
		return "public"
	case AccessProtected:
		return "protected"
	case AccessPrivate:
		return "private"
	}
	return "public"
}

// BaseSpec represents a base class specifier.
type BaseSpec struct {
	Type    Type
	Access  Access
	Virtual bool
}

// Field represents a non-static data member.
type Field struct {
	Name     string
	Type     Type
	BitField bool
	Width    int64
	Access   Access

	// HasInit says the member has a default member initializer.
	HasInit bool

	// Align is the member's alignas value, zero when none.
	Align int64
}

// Method represents a member function.
type Method struct {
	Name        string
	Func        *Func
	Access      Access
	Virtual     bool
	PureVirtual bool

	// Template marks a member template: a family of members rather than one,
	// whose Func is written in its own template parameters.
	Template  bool
	Static    bool
	Explicit  bool
	Friend    bool
	Defaulted bool
	Deleted   bool
}

// Record represents a class, struct, or union type. Identity is pointer-based.
type Record struct {
	Tag  TagKind
	Name string

	// Scopes are the namespaces and classes enclosing the declaration, outermost first.
	Scopes []string

	// TemplateArgs is set on a specialization of a class template: the
	// arguments it was instantiated with, which are part of its name.
	TemplateArgs []TemplateArg

	Bases    []BaseSpec
	Fields   []Field
	Methods  []*Method
	Complete bool
	Packed   bool
	Align    int64
	Pack     int64

	// FriendClasses names classes granted access as friends.
	FriendClasses []string

	// FriendFuncs names functions granted friend access.
	FriendFuncs []string

	// Final is the contextual `final` on the class-head.
	Final bool

	// InheritedCtors are direct bases whose constructors were inherited via using-declarations.
	InheritedCtors []*Record
}

func (r *Record) Kind() Kind { return RecordKind }

func (r *Record) Equal(other Type) bool {
	if other == nil {
		return false
	}
	o, ok := Unqualify(other).(*Record)
	return ok && r == o
}

func (r *Record) MemberAlign(n int64) int64 {
	switch {
	case r.Packed:
		return 1
	case r.Pack > 0 && n > r.Pack:
		return r.Pack
	}
	return n
}

// Enumerator represents one enumerator constant.
type Enumerator struct {
	Name string
	Val  int64
}

// Enum represents an enumerated type. Identity is pointer-based.
type Enum struct {
	Name        string
	Scopes      []string // as Record.Scopes
	Scoped      bool     // enum class / enum struct
	Underlying  Type
	Enumerators []Enumerator
	Complete    bool
}

func (*Enum) Kind() Kind { return EnumKind }

func (e *Enum) Equal(other Type) bool {
	if other == nil {
		return false
	}
	o, ok := Unqualify(other).(*Enum)
	return ok && e == o
}

// TemplateParam represents a template parameter type placeholder.
type TemplateParam struct {
	Name    string
	Index   int
	Depth   int
	IsPack  bool
	IsType  bool
	NonType Type
}

func (*TemplateParam) Kind() Kind { return TemplateParamKind }

func (tp *TemplateParam) Equal(other Type) bool {
	if other == nil {
		return false
	}
	o, ok := Unqualify(other).(*TemplateParam)
	return ok && tp.Index == o.Index && tp.Depth == o.Depth && tp.IsType == o.IsType && tp.IsPack == o.IsPack
}

// TemplateArg represents an argument to a template.
type TemplateArg struct {
	IsType bool
	Type   Type

	// Val is a non-type argument's value, and ValType the converted parameter type.
	Val     int64
	ValType Type
}

// String is the argument as a template-argument-list would spell it.
func (ta TemplateArg) String() string {
	if ta.IsType {
		if ta.Type == nil {
			return "?"
		}
		return ta.Type.String()
	}
	return strconv.FormatInt(ta.Val, 10)
}

func (ta TemplateArg) Equal(other TemplateArg) bool {
	if ta.IsType != other.IsType {
		return false
	}
	if ta.IsType {
		if ta.Type == nil && other.Type == nil {
			return true
		}
		if ta.Type != nil && other.Type != nil {
			return ta.Type.Equal(other.Type)
		}
		return false
	}
	return ta.Val == other.Val
}

// TemplateSpecialization represents an instantiated or specialized template type.
type TemplateSpecialization struct {
	Name string
	Args []TemplateArg
	Type Type
}

func (*TemplateSpecialization) Kind() Kind { return TemplateSpecializationKind }

func (ts *TemplateSpecialization) Equal(other Type) bool {
	if other == nil {
		return false
	}
	o, ok := Unqualify(other).(*TemplateSpecialization)
	if !ok || ts.Name != o.Name || len(ts.Args) != len(o.Args) {
		return false
	}
	for i := range ts.Args {
		if !ts.Args[i].Equal(o.Args[i]) {
			return false
		}
	}
	return true
}

// TemplateRef represents a template template argument: a class template named by its primary record.
type TemplateRef struct {
	Name    string
	Primary *Record
}

func (*TemplateRef) Kind() Kind { return TemplateRefKind }

func (t *TemplateRef) Equal(other Type) bool {
	o, ok := other.(*TemplateRef)
	return ok && t.Primary == o.Primary
}

func (t *TemplateRef) String() string { return t.Name }

// Pack is what a template parameter pack is bound to in a specialization:
// the arguments it absorbed, in order, possibly none. It is a type only so
// that a binding can hold it beside the other arguments; nothing has a
// Pack as its type.
type Pack struct {
	Elems []TemplateArg
}

func (*Pack) Kind() Kind { return PackKind }

func (p *Pack) Equal(other Type) bool {
	o, ok := other.(*Pack)
	if !ok || len(p.Elems) != len(o.Elems) {
		return false
	}
	for i := range p.Elems {
		if !p.Elems[i].Equal(o.Elems[i]) {
			return false
		}
	}
	return true
}

func (p *Pack) String() string {
	parts := make([]string, len(p.Elems))
	for i, e := range p.Elems {
		parts[i] = e.String()
	}
	return strings.Join(parts, ", ")
}

// DependentType represents an uninstantiated dependent type.
type DependentType struct {
	Name string
}

func (*DependentType) Kind() Kind { return DependentKind }

func (d *DependentType) Equal(other Type) bool {
	if other == nil {
		return false
	}
	o, ok := Unqualify(other).(*DependentType)
	return ok && d.Name == o.Name
}

// ---- Type String Representation ----

func (b *Basic) String() string {
	switch b.K {
	case Void:
		return "void"
	case Bool:
		return "bool"
	case Char:
		return "char"
	case SChar:
		return "signed char"
	case UChar:
		return "unsigned char"
	case Char8:
		return "char8_t"
	case Char16:
		return "char16_t"
	case Char32:
		return "char32_t"
	case WChar:
		return "wchar_t"
	case Short:
		return "short"
	case UShort:
		return "unsigned short"
	case Int:
		return "int"
	case UInt:
		return "unsigned int"
	case Long:
		return "long"
	case ULong:
		return "unsigned long"
	case LongLong:
		return "long long"
	case ULongLong:
		return "unsigned long long"
	case Int128:
		return "__int128"
	case UInt128:
		return "unsigned __int128"
	case Float:
		return "float"
	case Double:
		return "double"
	case LongDouble:
		return "long double"
	case NullptrKind:
		return "std::nullptr_t"
	case AutoKind:
		return "auto"
	case DecltypeAutoKind:
		return "decltype(auto)"
	}
	return "invalid"
}

func (q *Qualified) String() string {
	var b strings.Builder
	if q.Q&QConst != 0 {
		b.WriteString("const ")
	}
	if q.Q&QVolatile != 0 {
		b.WriteString("volatile ")
	}
	b.WriteString(q.T.String())
	return b.String()
}

func (p *Pointer) String() string {
	return p.Elem.String() + "*"
}

func (r *LValueReference) String() string {
	return r.Elem.String() + "&"
}

func (r *RValueReference) String() string {
	return r.Elem.String() + "&&"
}

func (m *MemberPointer) String() string {
	return fmt.Sprintf("%s::* %s", m.Class, m.Elem)
}

func (a *Array) String() string {
	if a.Incomplete {
		return a.Elem.String() + "[]"
	}
	if a.DepLen != "" {
		return fmt.Sprintf("%s[%s]", a.Elem, a.DepLen)
	}
	return fmt.Sprintf("%s[%d]", a.Elem, a.Len)
}

func (f *Func) String() string {
	var b strings.Builder
	b.WriteString(f.Ret.String())
	b.WriteString(" (")
	for i, p := range f.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Type.String())
	}
	if f.Variadic {
		if len(f.Params) > 0 {
			b.WriteString(", ")
		}
		b.WriteString("...")
	}
	b.WriteString(")")
	if f.Quals&QConst != 0 {
		b.WriteString(" const")
	}
	if f.Quals&QVolatile != 0 {
		b.WriteString(" volatile")
	}
	switch f.RefQual {
	case RefQualLValue:
		b.WriteString(" &")
	case RefQualRValue:
		b.WriteString(" &&")
	}
	if f.Noexcept {
		b.WriteString(" noexcept")
	}
	return b.String()
}

func (r *Record) String() string {
	kw := "struct"
	switch r.Tag {
	case TagClass:
		kw = "class"
	case TagUnion:
		kw = "union"
	}
	if r.Name == "" {
		return kw + " <anonymous>"
	}
	// A specialization is spelled with its arguments: `integral_constant`
	// names the template, and the template names no type.
	if r.TemplateArgs != nil {
		parts := make([]string, len(r.TemplateArgs))
		for i, a := range r.TemplateArgs {
			parts[i] = a.String()
		}
		return kw + " " + r.Name + "<" + strings.Join(parts, ", ") + ">"
	}
	return kw + " " + r.Name
}

func (e *Enum) String() string {
	kw := "enum"
	if e.Scoped {
		kw = "enum class"
	}
	if e.Name == "" {
		return kw + " <anonymous>"
	}
	return kw + " " + e.Name
}

func (tp *TemplateParam) String() string {
	if tp.Name != "" {
		return tp.Name
	}
	return fmt.Sprintf("T%d_%d", tp.Depth, tp.Index)
}

func (ts *TemplateSpecialization) String() string {
	var b strings.Builder
	b.WriteString(ts.Name)
	b.WriteByte('<')
	for i, a := range ts.Args {
		if i > 0 {
			b.WriteString(", ")
		}
		if a.IsType {
			if a.Type != nil {
				b.WriteString(a.Type.String())
			} else {
				b.WriteString("type")
			}
		} else {
			b.WriteString(fmt.Sprintf("%d", a.Val))
		}
	}
	b.WriteByte('>')
	return b.String()
}

func (d *DependentType) String() string {
	return d.Name
}
