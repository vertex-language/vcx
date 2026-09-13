// Package ast defines the syntax tree built by the parser.
// Nodes embed a Span of token indices (Tok) rather than byte offsets,
// preserving token origins and expansion chains from preprocessing.
package ast

import "github.com/vertex-language/vcx/token"

// Tok is an index into the translation unit's preprocessing-token slice. It
// is not a byte offset: see the package comment.
type Tok int32

// NoTok is the absent index — an optional keyword that was not written, a
// delimiter that does not exist in this form of the production.
const NoTok Tok = -1

// IsValid reports whether t names a token.
func (t Tok) IsValid() bool { return t >= 0 }

// Node is the interface all tree nodes implement.
type Node interface {
	Pos() Tok // index of the first token
	End() Tok // one past the index of the last
}

// Span is the stored extent every node embeds: a half-open range of tokens.
type Span struct {
	Lo Tok // inclusive
	Hi Tok // exclusive
}

func (s Span) Pos() Tok { return s.Lo }
func (s Span) End() Tok { return s.Hi }

// The hierarchies. Marker methods are unexported, so each is closed.

type Expr interface {
	Node
	exprNode()
}

type Stmt interface {
	Node
	stmtNode()
}

type Decl interface {
	Node
	declNode()
}

// Declarator is the type-syntax hierarchy. DeclName returns the declared
// name, or nil — an abstract declarator is a declarator with no name, which
// is what a type-id and a parameter without a name both are.
type Declarator interface {
	Node
	DeclName() Name
	declaratorNode()
}

// Name is anything that can appear where the grammar says id-expression or
// declarator-id: a plain identifier, a qualified name, a template-id, an
// operator-function-id, a conversion-function-id, a literal-operator-id, or a
// destructor-id.
//
// It is a hierarchy rather than a string because none of those can be reduced
// to one without losing the question sema has to answer. `A::B<int>::~B` is
// four decisions, and the parser is the only phase that can see them as
// syntax.
type Name interface {
	Expr
	nameNode()
}

// Position is a resolved location, in raw as-typed coordinates.
type Position struct {
	Filename string
	Line     int
	Column   int
}

// Unit resolves token indices. The parser supplies one; this package never
// learns what a file is, which is what keeps it importing nothing but token.
//
// The interface is defined here rather than taken from preprocessor because
// the tree is what has the requirement: a consumer needs a spelling, a kind
// and a place, and nothing else about phase 4 is any of ast's business.
type Unit interface {
	// Len is the number of tokens.
	Len() int
	// Text is the spelling of one token.
	Text(Tok) string
	// Kind is one token's lexical class.
	Kind(Tok) token.Kind
	// Position resolves one token to the file, line and column it was
	// written at — through macro expansion, the use site.
	Position(Tok) Position
	// Between returns the raw text separating two tokens: the trivia
	// v++ fmt prints back between them.
	Between(a, b Tok) string
}

// Ident is two token indices; spelling resolves through the Unit.
type Ident struct {
	Span
}

// Text returns the identifier's spelling.
func (id *Ident) Text(u Unit) string {
	if id == nil || !id.Lo.IsValid() {
		return ""
	}
	return u.Text(id.Lo)
}

func (*Ident) exprNode() {}
func (*Ident) nameNode() {}

// Releaser is the one-method window through which ast sees the parser's
// arena.
type Releaser interface {
	Release()
}

// File is one translation unit's tree.
//
// A C++ translation unit has more shape than a C one: a module unit is a
// global module fragment, then a module declaration, then declarations, then
// optionally a private module fragment. Module and Fragments record what
// phase 4 already recognized, so nothing here has to re-derive it.
type File struct {
	Span

	// Unit is what every index in the tree resolves through.
	Unit Unit

	// Module is the unit's module declaration, or nil for a
	// non-module unit.
	Module *ModuleDecl

	// Decls are the top-level declarations, in written order. Import
	// declarations appear here too, in place.
	Decls []Decl

	// Comments holds the indices of comment tokens, retained under
	// parser.ParseComments.
	Comments []Tok

	// Packs records `#pragma pack` directives and their effective token positions.
	Packs []PackAt

	rel Releaser
}

// PackAt is one #pragma pack: from token At on, members align to at most
// Pack bytes. Zero restores the default.
type PackAt struct {
	At   Tok
	Pack int64
}

// PackAt is the ceiling in effect at a position: the last pragma before
// it, or zero where there was none.
func (f *File) PackAt(pos Tok) int64 {
	var pack int64
	for _, p := range f.Packs {
		if p.At > pos {
			break
		}
		pack = p.Pack
	}
	return pack
}

// SetReleaser attaches the tree's backing storage.
func (f *File) SetReleaser(r Releaser) { f.rel = r }

// Release frees the tree's backing storage. Nodes should not be used after Release.
func (f *File) Release() {
	if f.rel != nil {
		r := f.rel
		f.rel = nil
		r.Release()
	}
}
