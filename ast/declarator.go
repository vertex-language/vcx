package ast

import "github.com/vertex-language/vcx/token"

// DeclSpec is one declaration specifier. The list is an ordered slice rather
// than a bag of flags because `const unsigned long int` and `unsigned const
// int long` are the same type and not the same text, and fmt has to put back
// what was written.
type DeclSpec interface {
	Node
	declSpecNode()
}

// DeclSpecs is a decl-specifier-seq, in written order.
type DeclSpecs struct {
	Span
	List []DeclSpec

	// Aligns are the alignas specifiers written among the specifiers,
	// which appertain to the entity being declared.
	Aligns []*AttrGroup
}

// BasicSpec is a specifier that is exactly one keyword: a storage class
// (static, extern, thread_local, mutable, register), a function specifier
// (inline, virtual, explicit — the last with an optional condition),
// friend, typedef, constexpr, consteval, constinit, a cv-qualifier, or a
// simple type name (int, char, bool, auto, void, …).
//
// One node for all of them because the parser's job is to record which
// keyword appeared where; deciding whether the combination is a type is
// sema's, and it wants the list in order to say so.
type BasicSpec struct {
	Span
	Kind token.Kind
}

// ExplicitSpec is `explicit` or `explicit ( expr )`.
type ExplicitSpec struct {
	Span
	Keyword Tok
	Lparen  Tok
	Cond    Expr
	Rparen  Tok
}

// NamedTypeSpec is a type named by a name: `T`, `std::vector<int>`,
// `typename T::type`.
//
// Typename records an explicit `typename` keyword, which is not decoration:
// inside a template it is what tells the parser the qualified name is a type
// at all.
type NamedTypeSpec struct {
	Span
	Typename Tok // the `typename` keyword, or NoTok
	Name     Name
}

// ElaboratedSpec is `class C`, `struct S`, `union U`, `enum E` used as a type
// specifier rather than a definition.
type ElaboratedSpec struct {
	Span
	Keyword Tok
	Kind    token.Kind // CLASS, STRUCT, UNION, ENUM
	Attrs   []*Attr
	Name    Name
}

// DecltypeSpec is `decltype(expr)` or `decltype(auto)`.
type DecltypeSpec struct {
	Span
	Keyword Tok
	Lparen  Tok
	Auto    Tok // the `auto` of decltype(auto), or NoTok
	X       Expr
	Rparen  Tok
}

// TypeTransformSpec is a compiler's type-transformation builtin written as a
// type specifier: `__remove_reference_t(T)`, `__decay(T)`,
// `__underlying_type(E)`. The name says which transformation, and the
// operand is a type-id.
type TypeTransformSpec struct {
	Span
	Name   Tok
	Lparen Tok
	Arg    *TypeId
	Rparen Tok
}

// ConstrainedAutoSpec is `Concept auto` or `Concept<Args> auto` — a
// placeholder constrained by a concept.
type ConstrainedAutoSpec struct {
	Span
	Concept Name
	Auto    Tok
	Kind    token.Kind // AUTO, or DECLTYPE for `C decltype(auto)`
}

// ClassSpec is a class definition used as a type specifier: the whole
// `class C : public B { ... }`.
type ClassSpec struct {
	Span
	Keyword Tok
	Kind    token.Kind // CLASS, STRUCT, UNION
	Attrs   []*Attr
	Aligns  []*AttrGroup // alignas on the class-head
	Name    Name         // nil for an unnamed class
	Final   Tok          // the contextual `final`, or NoTok
	Colon   Tok          // before the base list, or NoTok
	Bases   []*BaseSpec
	Lbrace  Tok
	Members []Decl
	Rbrace  Tok
}

// BaseSpec is one base-specifier: `public virtual B<int>`.
type BaseSpec struct {
	Span
	Attrs    []*Attr
	Virtual  Tok
	Access   Tok        // public/protected/private, or NoTok
	AccKind  token.Kind //
	Name     Name
	Ellipsis Tok // a pack expansion of the base, or NoTok
}

// EnumSpec is an enum definition or an opaque enum declaration.
type EnumSpec struct {
	Span
	Keyword Tok
	Kind    token.Kind // ENUM
	Scoped  Tok        // the `class` or `struct` of a scoped enum, or NoTok
	Attrs   []*Attr
	Name    Name
	Colon   Tok        // before the underlying type, or NoTok
	Base    *DeclSpecs // the underlying type
	Lbrace  Tok        // NoTok for an opaque declaration
	Values  []*Enumerator
	Comma   Tok // trailing comma
	Rbrace  Tok
}

// Enumerator is one `NAME = value` inside an enum.
type Enumerator struct {
	Span
	Name   *Ident
	Attrs  []*Attr
	Assign Tok
	Value  Expr
}

func (*DeclSpecs) declSpecNode()           {}
func (*BasicSpec) declSpecNode()           {}
func (*ExplicitSpec) declSpecNode()        {}
func (*NamedTypeSpec) declSpecNode()       {}
func (*ElaboratedSpec) declSpecNode()      {}
func (*DecltypeSpec) declSpecNode()        {}
func (*TypeTransformSpec) declSpecNode()   {}
func (*ConstrainedAutoSpec) declSpecNode() {}
func (*ClassSpec) declSpecNode()           {}
func (*EnumSpec) declSpecNode()            {}

// ---- declarators ----

// NameDeclarator is the leaf: the declared name, or nothing at all. An
// abstract declarator — a type-id's, or an unnamed parameter's — is this node
// with a nil Name and an empty span at the point the name would have been.
type NameDeclarator struct {
	Span
	Name  Name
	Attrs []*Attr
}

// PointerDeclarator is `* D`, `& D`, `&& D`, or `C::* D`.
//
// One node for all four because they are one production: a ptr-operator
// followed by a declarator. Kind says which, Class is the nested-name of a
// pointer-to-member, and Quals are the cv-qualifiers that bind to the pointer
// rather than to what it points at.
type PointerDeclarator struct {
	Span
	OpPos Tok
	Kind  token.Kind // MUL, AND (lvalue ref), LAND (rvalue ref)
	Class Name       // the `C::` of a pointer-to-member, or nil
	Attrs []*Attr
	Quals []DeclSpec // const, volatile, __restrict
	Inner Declarator
}

// ArrayDeclarator is `D [ size ]`.
type ArrayDeclarator struct {
	Span
	Inner  Declarator
	Lbrack Tok
	Size   Expr // nil for an unbounded array
	Attrs  []*Attr
	Rbrack Tok
}

// FuncDeclarator is `D ( params ) cv ref noexcept attrs trailing requires`.
//
// This node carries everything that can follow a parameter list, which in
// C++23 is a great deal: the cv-qualifiers and ref-qualifier of a member
// function, an exception specification, a trailing return type, and a
// trailing requires-clause. All of it belongs to the declarator rather than
// to the declaration, because `auto f() -> int` puts the return type here.
type FuncDeclarator struct {
	Span
	Inner  Declarator
	Lparen Tok
	Params []*ParamDecl
	Vararg Tok // a trailing `...`, or NoTok
	Rparen Tok

	Quals    []DeclSpec // const, volatile
	RefQual  Tok        // `&` or `&&`, or NoTok
	RefKind  token.Kind
	Noexcept *NoexceptSpec
	Attrs    []*Attr
	Trailing *TrailingReturn
	Requires *RequiresClause

	// Virt records the virt-specifiers `override` and `final`, which are
	// identifiers with special meaning and therefore positions rather than
	// keywords.
	Override Tok
	Final    Tok
}

// ParenDeclarator is `( D )`. Kept because it changes what the suffixes bind
// to — `int (*f)()` against `int *f()` — and because fmt must not remove it.
type ParenDeclarator struct {
	Span
	Lparen Tok
	Inner  Declarator
	Rparen Tok
}

// BitfieldDeclarator is `D : width`, a member declarator.
type BitfieldDeclarator struct {
	Span
	Inner Declarator
	Colon Tok
	Width Expr
}

// PackDeclarator is `... D`, the declarator of a parameter pack.
type PackDeclarator struct {
	Span
	Ellipsis Tok
	Inner    Declarator
}

// BadDeclarator is a declarator the parser gave up on.
type BadDeclarator struct {
	Span
}

func (d *NameDeclarator) DeclName() Name     { return d.Name }
func (d *PointerDeclarator) DeclName() Name  { return declName(d.Inner) }
func (d *ArrayDeclarator) DeclName() Name    { return declName(d.Inner) }
func (d *FuncDeclarator) DeclName() Name     { return declName(d.Inner) }
func (d *ParenDeclarator) DeclName() Name    { return declName(d.Inner) }
func (d *BitfieldDeclarator) DeclName() Name { return declName(d.Inner) }
func (d *PackDeclarator) DeclName() Name     { return declName(d.Inner) }
func (d *BadDeclarator) DeclName() Name      { return nil }

func declName(d Declarator) Name {
	if d == nil {
		return nil
	}
	return d.DeclName()
}

func (*NameDeclarator) declaratorNode()     {}
func (*PointerDeclarator) declaratorNode()  {}
func (*ArrayDeclarator) declaratorNode()    {}
func (*FuncDeclarator) declaratorNode()     {}
func (*ParenDeclarator) declaratorNode()    {}
func (*BitfieldDeclarator) declaratorNode() {}
func (*PackDeclarator) declaratorNode()     {}
func (*BadDeclarator) declaratorNode()      {}

// ---- what hangs off a declarator ----

// TypeId is a decl-specifier-seq plus an abstract declarator: the operand of
// sizeof, a template argument, a cast's target, a conversion function's type.
type TypeId struct {
	Span
	Specs *DeclSpecs
	Decl  Declarator // abstract: its DeclName is nil
}

// TrailingReturn is `-> T`.
type TrailingReturn struct {
	Span
	Arrow Tok
	Type  *TypeId
}

// NoexceptSpec is `noexcept`, `noexcept(expr)`, or the deprecated
// `throw()`. The last is kept because it is still in shipping headers and
// refusing to parse it means refusing the header.
type NoexceptSpec struct {
	Span
	Keyword Tok
	Throw   bool // spelled `throw()`
	Lparen  Tok
	Cond    Expr
	Rparen  Tok
}

// ParamDecl is one parameter: specifiers, a declarator that may be abstract,
// and an optional default argument.
type ParamDecl struct {
	Span
	Attrs   []*Attr
	Specs   *DeclSpecs
	Decl    Declarator
	Assign  Tok
	Default Expr
}

func (*ParamDecl) declNode() {}

// ---- attributes ----

// Attr is one attribute inside `[[ ... ]]`, or one tolerated vendor
// spelling.
//
// Namespace is the `gnu` of `[[gnu::always_inline]]`. Args are the tokens
// between the parentheses, kept as a span rather than a tree: an attribute
// argument clause is a balanced token sequence whose grammar belongs to
// whoever defined the attribute, and parsing it here would mean owning every
// vendor's syntax.
type Attr struct {
	Span
	Namespace *Ident
	Name      *Ident
	Lparen    Tok
	Args      Span // the raw token extent between the parentheses
	Rparen    Tok
	Ellipsis  Tok // a pack expansion of the attribute, or NoTok

	// Vendor is the spelling this attribute arrived as when it was not
	// written `[[...]]`: __attribute__, __declspec, or a pragma. Zero for a
	// standard attribute.
	Vendor token.Kind
}

// AttrGroup is one `[[ ... ]]` or `alignas(...)`, holding the attributes it
// contained and the extent of the brackets.
type AttrGroup struct {
	Span
	Using  *Ident // the `using ns:` prefix of an attribute-using-prefix
	Attrs  []*Attr
	Align  *TypeId // alignas(type-id), or nil
	AlignX Expr    // alignas(expr), or nil
}

// TypeTransforms lists type-transformation builtins recognized as type specifiers.
var TypeTransforms = map[string]bool{
	"__remove_reference_t": true, "__remove_reference": true,
	"__remove_cv": true, "__remove_const": true, "__remove_volatile": true, "__remove_cvref": true,
	"__remove_pointer": true, "__add_pointer": true,
	"__add_lvalue_reference": true, "__add_rvalue_reference": true,
	"__decay": true, "__remove_extent": true, "__remove_all_extents": true,
	"__underlying_type": true, "__make_signed": true, "__make_unsigned": true,
}
