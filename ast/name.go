package ast

import "github.com/vertex-language/vcx/token"

// Every name node here implements both Name and Expr.

// BadName is a name the parser gave up on. Its span is non-empty.
type BadName struct {
	Span
}

// QualifiedName represents a qualified name (e.g. A::B::c, ::c, T::type).
type QualifiedName struct {
	Span
	Global Tok    // the leading `::`, NoTok if absent
	Qual   []Name // the components before the final one, in order
	Colons []Tok
	Name   Name // the final component
}

// TemplateName is a template-id f<Args...>.
type TemplateName struct {
	Span
	Template Tok  // an explicit `template` keyword before the name, or NoTok
	Name     Name // the template's own name
	Less     Tok
	Args     []Node // *TypeId or Expr, in written order
	Comma    Tok
	Greater  Tok
}

// OperatorName represents an operator-function-id (e.g. operator+, operator[]).
type OperatorName struct {
	Span
	Operator Tok
	OpPos    Tok
	Op       token.Kind
	Array    bool // `operator new[]` / `operator delete[]`
	Lbrack   Tok  // the brackets of operator[] or the array forms
	Rbrack   Tok
	Lparen   Tok // the parens of operator()
	Rparen   Tok
}

// ConversionName is `operator T` — a conversion-function-id.
type ConversionName struct {
	Span
	Operator Tok
	Type     *TypeId
}

// LiteralOperatorName is a user-defined literal operator name (e.g. operator ""_km).
type LiteralOperatorName struct {
	Span
	Operator Tok
	String   Tok // the "" token
	Suffix   *Ident
}

// DestructorName is `~C` or `~decltype(e)`.
type DestructorName struct {
	Span
	Tilde Tok
	Name  Name // the class name, for `~C`
	Type  Node // a *DecltypeSpec, for `~decltype(e)`; nil otherwise
}

// PackName is a name followed by `...` in an expansion context.
type PackName struct {
	Span
	Name     Name
	Ellipsis Tok
}

func (*BadName) exprNode()             {}
func (*BadName) nameNode()             {}
func (*QualifiedName) exprNode()       {}
func (*QualifiedName) nameNode()       {}
func (*TemplateName) exprNode()        {}
func (*TemplateName) nameNode()        {}
func (*OperatorName) exprNode()        {}
func (*OperatorName) nameNode()        {}
func (*ConversionName) exprNode()      {}
func (*ConversionName) nameNode()      {}
func (*LiteralOperatorName) exprNode() {}
func (*LiteralOperatorName) nameNode() {}
func (*DestructorName) exprNode()      {}
func (*DestructorName) nameNode()      {}
func (*PackName) exprNode()            {}
func (*PackName) nameNode()            {}
