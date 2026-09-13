package ast

import "github.com/vertex-language/vcx/token"

// BadDecl covers tokens the parser gave up on.
type BadDecl struct {
	Span
}

// SimpleDecl represents a declaration of variables, types, or function prototypes: `specs declarators ;`.
type SimpleDecl struct {
	Span
	Attrs []*AttrGroup
	Specs *DeclSpecs
	Inits []*InitDeclarator
	Semi  Tok
}

// InitDeclarator represents a declarator and its optional initializer.
type InitDeclarator struct {
	Span
	Decl Declarator

	Assign Tok // `= e`, or NoTok
	Value  Expr

	Lparen Tok // `( args )`, or NoTok
	Args   []Expr
	Rparen Tok

	Braced *InitList // `{ ... }`, or nil

	// Requires is a trailing requires-clause on a declarator that is not a
	// function — rare, but the grammar admits it.
	Requires *RequiresClause

	// Asm is GNU's asm label (e.g. __asm("...")) or NoTok if absent.
	Asm      Tok
	AsmLabel []Tok
}

// FuncDecl represents a function definition.
type FuncDecl struct {
	Span
	Attrs []*AttrGroup
	Specs *DeclSpecs
	Decl  Declarator

	// Inits is a constructor's ctor-initializer list.
	Colon Tok
	Inits []*MemInit

	Body Stmt

	Assign    Tok // `= default` / `= delete`
	Defaulted Tok
	Deleted   Tok
	// Reason is the C++26 `= delete("because")` message.
	DelLparen Tok
	Reason    *StringLit
	DelRparen Tok

	Semi Tok
}

// MemInit is one entry of a ctor-initializer: `base(args)`, `member{args}`,
// or a pack expansion of either.
type MemInit struct {
	Span
	Name     Name
	Lparen   Tok
	Args     []Expr
	Rparen   Tok
	Braced   *InitList
	Ellipsis Tok
}

// StructuredBinding is `auto [a, b, c] = e`.
type StructuredBinding struct {
	Span
	Attrs   []*AttrGroup
	Specs   *DeclSpecs
	Ref     Tok // the `&` or `&&` of `auto&& [a, b]`, or NoTok
	RefKind token.Kind
	Lbrack  Tok
	Names   []*Ident
	Rbrack  Tok

	Assign Tok
	Value  Expr
	Lparen Tok
	Args   []Expr
	Rparen Tok
	Braced *InitList
	Semi   Tok
}

// NamespaceDecl represents a namespace definition (including inline and nested forms).
type NamespaceDecl struct {
	Span
	Inline  Tok
	Keyword Tok
	Attrs   []*AttrGroup
	Names   []*Ident // more than one for `namespace a::b::c`
	Inlines []Tok
	Lbrace  Tok
	Decls   []Decl
	Rbrace  Tok
}

// NamespaceAliasDecl is `namespace N = A::B;`.
type NamespaceAliasDecl struct {
	Span
	Keyword Tok
	Name    *Ident
	Assign  Tok
	Target  Name
	Semi    Tok
}

// UsingDecl is `using A::b;` or `using enum E;`.
type UsingDecl struct {
	Span
	Keyword  Tok
	Enum     Tok // the `enum` of `using enum E;`, or NoTok
	Typename Tok
	Names    []Name // a using-declarator-list
	Semi     Tok
}

// UsingDirectiveDecl is `using namespace N;`.
type UsingDirectiveDecl struct {
	Span
	Attrs     []*AttrGroup
	Keyword   Tok
	Namespace Tok
	Name      Name
	Semi      Tok
}

// AliasDecl is `using T = U;`, including the template form, which arrives
// wrapped in a TemplateDecl.
type AliasDecl struct {
	Span
	Attrs   []*AttrGroup
	Keyword Tok
	Name    *Ident
	Assign  Tok
	Type    *TypeId
	Semi    Tok
}

// StaticAssertDecl is `static_assert(cond)` or `static_assert(cond, msg)`.
type StaticAssertDecl struct {
	Span
	Keyword Tok
	Lparen  Tok
	Cond    Expr
	Comma   Tok
	Msg     Expr
	Rparen  Tok
	Semi    Tok
}

// AccessDecl is `public:`, `protected:` or `private:` inside a class.
type AccessDecl struct {
	Span
	Keyword Tok
	Kind    token.Kind
	Colon   Tok
}

// LinkageDecl is `extern "C" { ... }` or `extern "C" decl`.
type LinkageDecl struct {
	Span
	Keyword Tok
	Lang    *StringLit
	Lbrace  Tok // NoTok for the single-declaration form
	Decls   []Decl
	Rbrace  Tok
}

// EmptyDecl is a stray `;` at namespace or class scope.
type EmptyDecl struct {
	Span
	Semi Tok
}

// AsmDecl is `asm("...");` at namespace scope.
type AsmDecl struct {
	Span
	Stmt *AsmStmt
}

// ---- modules ----

// ModuleDecl is `export_opt module Name;`, `module;`, or `module :private;`.
type ModuleDecl struct {
	Span
	Export  Tok
	Keyword Tok
	Name    []*Ident // the dotted name; empty for a fragment
	Colon   Tok      // the partition or private colon, or NoTok
	Part    []*Ident // the partition name
	Private Tok      // the `private` of a private module fragment
	Attrs   []*AttrGroup
	Semi    Tok
}

// ImportDecl is `export_opt import X;` (a module, partition, or header unit).
type ImportDecl struct {
	Span
	Export  Tok
	Keyword Tok
	Name    []*Ident
	Colon   Tok
	Part    []*Ident
	Header  Span // the HEADER_NAME token's extent, zero when absent
	Attrs   []*AttrGroup
	Semi    Tok
}

// ExportDecl is `export decl` or `export { decls }`.
type ExportDecl struct {
	Span
	Keyword Tok
	Lbrace  Tok
	Decls   []Decl
	Rbrace  Tok
}

func (*BadDecl) declNode()            {}
func (*SimpleDecl) declNode()         {}
func (*FuncDecl) declNode()           {}
func (*StructuredBinding) declNode()  {}
func (*NamespaceDecl) declNode()      {}
func (*NamespaceAliasDecl) declNode() {}
func (*UsingDecl) declNode()          {}
func (*UsingDirectiveDecl) declNode() {}
func (*AliasDecl) declNode()          {}
func (*StaticAssertDecl) declNode()   {}
func (*AccessDecl) declNode()         {}
func (*LinkageDecl) declNode()        {}
func (*EmptyDecl) declNode()          {}
func (*AsmDecl) declNode()            {}
func (*ModuleDecl) declNode()         {}
func (*ImportDecl) declNode()         {}
func (*ExportDecl) declNode()         {}
