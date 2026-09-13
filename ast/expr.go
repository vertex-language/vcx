package ast

import "github.com/vertex-language/vcx/token"

// BadExpr covers tokens the parser gave up on. Its span is non-empty even
// when nothing was consumed.
type BadExpr struct {
	Span
}

// BasicLit represents a numeric, character, boolean, or nullptr literal.
type BasicLit struct {
	Span
	Kind   token.Kind
	Suffix Span // zero when there is no ud-suffix

	// Text is the synthesized spelling of a synthetic literal, or empty.
	Text string
}

// Spelling is the literal's text: from the source, or the synthesized
// spelling when it has no source.
func (l *BasicLit) Spelling(u Unit) string {
	if l.Text != "" {
		return l.Text
	}
	return u.Text(l.Lo)
}

// StringLit represents one or more adjacent string literal tokens.
type StringLit struct {
	Span
	Segs   []Span
	Raw    bool // any segment was written R"(...)"
	Suffix Span // ud-suffix, zero when absent
}

// ThisExpr is `this`.
type ThisExpr struct {
	Span
}

// ParenExpr is ( X ). Kept even when redundant, for the round trip.
type ParenExpr struct {
	Span
	Lparen Tok
	X      Expr
	Rparen Tok
}

// IndexExpr represents a subscript expression X[Index...].
type IndexExpr struct {
	Span
	X      Expr
	Lbrack Tok
	Args   []Expr
	Rbrack Tok
}

// CallExpr is Fun(Args...).
type CallExpr struct {
	Span
	Fun    Expr
	Lparen Tok
	Args   []Expr
	Rparen Tok
}

// MemberExpr is X.Sel or X->Sel.
type MemberExpr struct {
	Span
	X        Expr
	OpPos    Tok
	Op       token.Kind // PERIOD or ARROW
	Template Tok        // the `template` keyword, or NoTok
	Sel      Name
}

// IncDecExpr is the postfix X++ or X--. Prefix forms are UnaryExpr.
type IncDecExpr struct {
	Span
	X     Expr
	OpPos Tok
	Op    token.Kind // INC or DEC
}

// CastExpr is a C-style cast (Type)X.
type CastExpr struct {
	Span
	Lparen Tok
	Type   *TypeId
	Rparen Tok
	X      Expr
}

// NamedCastExpr is static_cast<T>(x) and its three siblings.
type NamedCastExpr struct {
	Span
	Keyword Tok
	Kind    token.Kind // STATIC_CAST, DYNAMIC_CAST, CONST_CAST, REINTERPRET_CAST
	Less    Tok
	Type    *TypeId
	Greater Tok
	Lparen  Tok
	X       Expr
	Rparen  Tok
}

// FunctionalCastExpr is an explicit type conversion: T(args...) or T{args...}.
type FunctionalCastExpr struct {
	Span
	Type *TypeId
	Args *InitList // braced form
	// Parenthesized form:
	Lparen  Tok
	ArgList []Expr
	Rparen  Tok
}

// TypeidExpr is typeid(T) or typeid(expr): exactly one of Type and X is set.
type TypeidExpr struct {
	Span
	Keyword Tok
	Lparen  Tok
	Type    *TypeId
	X       Expr
	Rparen  Tok
}

// UnaryExpr is a prefix operator expression: & * + - ~ ! ++ --.
type UnaryExpr struct {
	Span
	OpPos Tok
	Op    token.Kind
	X     Expr
}

// SizeofExpr is sizeof X, sizeof(T), or sizeof...(pack).
type SizeofExpr struct {
	Span
	Keyword  Tok
	Ellipsis Tok // sizeof...(pack), or NoTok
	Lparen   Tok
	Type     *TypeId
	X        Expr
	Rparen   Tok
}

// AlignofExpr is alignof(T).
type AlignofExpr struct {
	Span
	Keyword Tok
	Lparen  Tok
	Type    *TypeId
	Rparen  Tok
}

// NoexceptExpr is the operator noexcept(e), not the specifier.
type NoexceptExpr struct {
	Span
	Keyword Tok
	Lparen  Tok
	X       Expr
	Rparen  Tok
}

// NewExpr represents a new-expression.
type NewExpr struct {
	Span
	Global    Tok // leading `::`, or NoTok
	Keyword   Tok
	Placement []Expr // the arguments of a placement new
	PlaceLp   Tok    // NoTok when there is no placement list
	PlaceRp   Tok
	Lparen    Tok // parenthesized type-id form: new (T)
	Type      *TypeId
	Rparen    Tok
	Init      Expr // *InitList, or a parenthesized argument list as *ParenExpr

	Args   []Expr // argument list for parenthesized initialization
	ArgsLp Tok    // NoTok when the initializer is braced or absent
	ArgsRp Tok
}

// DeleteExpr is `::opt delete X` or `::opt delete[] X`.
type DeleteExpr struct {
	Span
	Global  Tok
	Keyword Tok
	Lbrack  Tok // the array form's brackets, or NoTok
	Rbrack  Tok
	X       Expr
}

// ThrowExpr is `throw X` or a bare `throw` (a rethrow), where X is nil.
type ThrowExpr struct {
	Span
	Keyword Tok
	X       Expr
}

// CoAwaitExpr is `co_await X`.
type CoAwaitExpr struct {
	Span
	Keyword Tok
	X       Expr
}

// CoYieldExpr is `co_yield X`.
type CoYieldExpr struct {
	Span
	Keyword Tok
	X       Expr
}

// BinaryExpr represents a binary operator expression X op Y.
type BinaryExpr struct {
	Span
	X     Expr
	OpPos Tok
	Op    token.Kind
	Y     Expr
}

// CondExpr is Cond ? Then : Else.
type CondExpr struct {
	Span
	Cond     Expr
	Question Tok
	Then     Expr
	Colon    Tok
	Else     Expr
}

// AssignExpr is Lhs op= Rhs.
type AssignExpr struct {
	Span
	Lhs   Expr
	OpPos Tok
	Op    token.Kind // ASSIGN, MUL_ASSIGN, …
	Rhs   Expr
}

// PackExpansion is `X...` in an expression context.
type PackExpansion struct {
	Span
	X        Expr
	Ellipsis Tok
}

// FoldExpr represents a fold expression over a parameter pack.
type FoldExpr struct {
	Span
	Lparen   Tok
	Left     Expr // the operand before the operator, or nil
	OpPos    Tok  // the fold operator
	Op       token.Kind
	Ellipsis Tok
	Op2Pos   Tok        // the second operator of a binary fold, or NoTok
	Op2      token.Kind //
	Right    Expr       // the operand after the ellipsis, or nil
	Rparen   Tok
}

// LambdaExpr is a lambda expression.
type LambdaExpr struct {
	Span
	Lbrack   Tok
	Default  Tok        // `=` or `&` capture-default, or NoTok
	DefKind  token.Kind // ASSIGN or AND
	Captures []*Capture
	Rbrack   Tok

	Templ *TemplateParams // explicit template parameter list, or nil

	Lparen Tok
	Params []*ParamDecl
	Vararg Tok // `...` in the parameter list
	Rparen Tok

	Specs    []DeclSpec // mutable, constexpr, consteval, static
	Noexcept *NoexceptSpec
	Attrs    []*Attr
	Trailing *TrailingReturn
	Requires *RequiresClause

	Body *CompoundStmt
}

// Capture is one entry of a lambda's capture list: `x`, `&x`, `this`,
// `*this`, `x = expr`, or a pack expansion of any of those.
type Capture struct {
	Span
	Amp      Tok    // `&`, or NoTok
	Star     Tok    // the `*` of `*this`, or NoTok
	This     Tok    // the `this` keyword, or NoTok
	Name     *Ident // nil for a this-capture
	Assign   Tok    // init-capture's `=`, or NoTok
	Init     Expr   // init-capture's initializer
	Ellipsis Tok    // pack expansion, or NoTok
}

// RequiresExpr is `requires (params) { requirements }`.
type RequiresExpr struct {
	Span
	Keyword Tok
	Lparen  Tok
	Params  []*ParamDecl
	Rparen  Tok
	Lbrace  Tok
	Reqs    []Node // *SimpleReq, *TypeReq, *CompoundReq, *NestedReq
	Rbrace  Tok
}

// SimpleReq is `expr;` inside a requires-expression.
type SimpleReq struct {
	Span
	X    Expr
	Semi Tok
}

// TypeReq is `typename T::type;`.
type TypeReq struct {
	Span
	Typename Tok
	Name     Name
	Semi     Tok
}

// CompoundReq is `{ expr } noexceptopt -> constraintopt ;`.
type CompoundReq struct {
	Span
	Lbrace   Tok
	X        Expr
	Rbrace   Tok
	Noexcept Tok  // the keyword, or NoTok
	Arrow    Tok  // `->` before the return-type-requirement, or NoTok
	Ret      Expr // the type-constraint
	Semi     Tok
}

// NestedReq is `requires constraint;`.
type NestedReq struct {
	Span
	Keyword Tok
	X       Expr
	Semi    Tok
}

// InitList is a braced-init-list { ... }.
type InitList struct {
	Span
	Lbrace Tok
	Items  []Expr // an item may itself be an *InitList or a *DesignatedInit
	Comma  Tok
	Rbrace Tok
}

// DesignatedInit is `.name = value` or `.name{...}`.
type DesignatedInit struct {
	Span
	Dot    Tok
	Name   *Ident
	Assign Tok // NoTok for the braced form
	Value  Expr
}

// TypeTraitExpr represents a built-in type trait expression (e.g. __is_base_of(A, B)).
type TypeTraitExpr struct {
	Span
	Name   *Ident
	Lparen Tok
	Args   []*TypeId
	Rparen Tok
}

func (*BadExpr) exprNode()            {}
func (*BasicLit) exprNode()           {}
func (*StringLit) exprNode()          {}
func (*ThisExpr) exprNode()           {}
func (*ParenExpr) exprNode()          {}
func (*IndexExpr) exprNode()          {}
func (*CallExpr) exprNode()           {}
func (*MemberExpr) exprNode()         {}
func (*IncDecExpr) exprNode()         {}
func (*CastExpr) exprNode()           {}
func (*NamedCastExpr) exprNode()      {}
func (*FunctionalCastExpr) exprNode() {}
func (*TypeidExpr) exprNode()         {}
func (*UnaryExpr) exprNode()          {}
func (*SizeofExpr) exprNode()         {}
func (*AlignofExpr) exprNode()        {}
func (*NoexceptExpr) exprNode()       {}
func (*NewExpr) exprNode()            {}
func (*DeleteExpr) exprNode()         {}
func (*ThrowExpr) exprNode()          {}
func (*CoAwaitExpr) exprNode()        {}
func (*CoYieldExpr) exprNode()        {}
func (*BinaryExpr) exprNode()         {}
func (*CondExpr) exprNode()           {}
func (*AssignExpr) exprNode()         {}
func (*PackExpansion) exprNode()      {}
func (*FoldExpr) exprNode()           {}
func (*LambdaExpr) exprNode()         {}
func (*RequiresExpr) exprNode()       {}
func (*InitList) exprNode()           {}
func (*DesignatedInit) exprNode()     {}
func (*TypeTraitExpr) exprNode()      {}
