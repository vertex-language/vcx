package ast

import "github.com/vertex-language/vcx/token"

// BadStmt covers tokens the parser gave up on.
type BadStmt struct {
	Span
}

// DeclStmt is a declaration in statement position.
type DeclStmt struct {
	Span
	Decl Decl
}

// EmptyStmt is a lone `;`.
type EmptyStmt struct {
	Span
	Semi Tok
}

// ExprStmt is `X ;`.
type ExprStmt struct {
	Span
	X    Expr
	Semi Tok
}

// CompoundStmt is `{ ... }`.
type CompoundStmt struct {
	Span
	Lbrace Tok
	Stmts  []Stmt
	Rbrace Tok
}

// LabeledStmt is `name : S`, `case X : S`, or `default : S`.
type LabeledStmt struct {
	Span
	Attrs    []*Attr
	Name     *Ident // nil for case/default
	Keyword  Tok    // the `case` or `default`, or NoTok
	Kind     token.Kind
	Value    Expr // the case value
	Ellipsis Tok  // GNU's `case lo ... hi`, or NoTok
	High     Expr
	Colon    Tok
	Stmt     Stmt
}

// IfStmt is `if constexpr_opt ( init_opt cond ) then else_opt`.
type IfStmt struct {
	Span
	Keyword   Tok
	Constexpr Tok // or NoTok
	Consteval Tok // `if consteval`, or NoTok
	Not       Tok // the `!` of `if !consteval`, or NoTok
	Lparen    Tok
	Init      Stmt // the init-statement, or nil
	Cond      Node // an Expr, or a *VarDecl condition
	Rparen    Tok
	Then      Stmt
	ElsePos   Tok
	Else      Stmt
}

// SwitchStmt is `switch ( init_opt cond ) body`.
type SwitchStmt struct {
	Span
	Keyword Tok
	Lparen  Tok
	Init    Stmt
	Cond    Node
	Rparen  Tok
	Body    Stmt
}

// WhileStmt is `while ( cond ) body`.
type WhileStmt struct {
	Span
	Keyword Tok
	Lparen  Tok
	Cond    Node
	Rparen  Tok
	Body    Stmt
}

// DoStmt is `do body while ( cond ) ;`.
type DoStmt struct {
	Span
	Keyword  Tok
	Body     Stmt
	WhilePos Tok
	Lparen   Tok
	Cond     Expr
	Rparen   Tok
	Semi     Tok
}

// ForStmt is the three-clause `for ( init cond ; post ) body`.
type ForStmt struct {
	Span
	Keyword Tok
	Lparen  Tok
	Init    Stmt // a DeclStmt, an ExprStmt, or an EmptyStmt
	Cond    Node
	Semi    Tok
	Post    Expr
	Rparen  Tok
	Body    Stmt
}

// RangeForStmt is `for ( init_opt decl : range ) body`.
type RangeForStmt struct {
	Span
	Keyword Tok
	Lparen  Tok
	Init    Stmt // C++20's init-statement, or nil
	Decl    Decl
	Colon   Tok
	Range   Expr
	Rparen  Tok
	Body    Stmt
}

// BreakStmt is `break ;`.
type BreakStmt struct {
	Span
	Keyword Tok
	Semi    Tok
}

// ContinueStmt is `continue ;`.
type ContinueStmt struct {
	Span
	Keyword Tok
	Semi    Tok
}

// ReturnStmt is `return X_opt ;`.
type ReturnStmt struct {
	Span
	Keyword Tok
	X       Expr
	Semi Tok
}

// CoReturnStmt is `co_return X_opt ;`.
type CoReturnStmt struct {
	Span
	Keyword Tok
	X       Expr
	Semi    Tok
}

// GotoStmt is `goto label ;`.
type GotoStmt struct {
	Span
	Keyword Tok
	Label   *Ident
	Semi    Tok
}

// TryStmt is `try block handlers...`.
type TryStmt struct {
	Span
	Keyword  Tok
	Init     []*MemInit // a function-try-block's ctor-initializer
	Body     *CompoundStmt
	Handlers []*CatchClause
}

// CatchClause is `catch ( param ) block`.
type CatchClause struct {
	Span
	Keyword  Tok
	Lparen   Tok
	Param    *ParamDecl
	Ellipsis Tok
	Rparen   Tok
	Body     *CompoundStmt
}

// AsmStmt represents an inline assembly statement asm(...).
type AsmStmt struct {
	Span
	Keyword Tok
	Quals   []DeclSpec // volatile, inline, goto
	Lparen  Tok
	Body    Span
	Rparen  Tok
	Semi    Tok
}

// AttrStmt is an attribute-specifier-seq applied to a statement.
type AttrStmt struct {
	Span
	Attrs []*AttrGroup
	Stmt  Stmt
}

func (*BadStmt) stmtNode()      {}
func (*DeclStmt) stmtNode()     {}
func (*EmptyStmt) stmtNode()    {}
func (*ExprStmt) stmtNode()     {}
func (*CompoundStmt) stmtNode() {}
func (*LabeledStmt) stmtNode()  {}
func (*IfStmt) stmtNode()       {}
func (*SwitchStmt) stmtNode()   {}
func (*WhileStmt) stmtNode()    {}
func (*DoStmt) stmtNode()       {}
func (*ForStmt) stmtNode()      {}
func (*RangeForStmt) stmtNode() {}
func (*BreakStmt) stmtNode()    {}
func (*ContinueStmt) stmtNode() {}
func (*ReturnStmt) stmtNode()   {}
func (*CoReturnStmt) stmtNode() {}
func (*GotoStmt) stmtNode()     {}
func (*TryStmt) stmtNode()      {}
func (*AsmStmt) stmtNode()      {}
func (*AttrStmt) stmtNode()     {}
