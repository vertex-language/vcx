package ast

import "github.com/vertex-language/vcx/token"

// TemplateDecl is `template < params > requires_opt decl`.
type TemplateDecl struct {
	Span
	Params   *TemplateParams
	Requires *RequiresClause
	Decl     Decl
}

// TemplateParams is `template < ... >`.
type TemplateParams struct {
	Span
	Keyword Tok
	Less    Tok
	Params  []Decl // *TypeParam, *ParamDecl (non-type), *TemplateTemplateParam
	Greater Tok
}

// TypeParam is `class T`, `typename T`, `typename... Ts`, `class T = int`,
// or a constrained parameter `Integral T`.
type TypeParam struct {
	Span
	Keyword    Tok        // `class` or `typename`, or NoTok when constrained
	Kind       token.Kind //
	Constraint Name       // the concept of a constrained parameter, or nil
	Ellipsis   Tok
	Name       *Ident // nil for an unnamed parameter
	Assign     Tok
	Default    *TypeId
}

// TemplateTemplateParam is `template <...> class T`.
type TemplateTemplateParam struct {
	Span
	Params   *TemplateParams
	Keyword  Tok
	Kind     token.Kind // CLASS or TYPENAME
	Ellipsis Tok
	Name     *Ident
	Assign   Tok
	Default  Name
}

// ExplicitSpecDecl is `template <> decl` — an explicit specialization.
type ExplicitSpecDecl struct {
	Span
	Keyword Tok
	Less    Tok
	Greater Tok
	Decl    Decl
}

// ExplicitInstDecl is `extern_opt template decl` — an explicit
// instantiation.
type ExplicitInstDecl struct {
	Span
	Extern  Tok
	Keyword Tok
	Decl    Decl
}

// ConceptDecl is `template <...> concept C = constraint;`. It arrives
// wrapped in a TemplateDecl, because that is how it is written.
type ConceptDecl struct {
	Span
	Keyword Tok
	Name    *Ident
	Assign  Tok
	Value   Expr
	Semi    Tok
}

// RequiresClause is `requires constraint-logical-or-expression`.
type RequiresClause struct {
	Span
	Keyword Tok
	X       Expr
}

func (*TemplateDecl) declNode()          {}
func (*TypeParam) declNode()             {}
func (*TemplateTemplateParam) declNode() {}
func (*ExplicitSpecDecl) declNode()      {}
func (*ExplicitInstDecl) declNode()      {}
func (*ConceptDecl) declNode()           {}
