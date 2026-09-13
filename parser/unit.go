package parser

import (
	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/preprocessor"
	"github.com/vertex-language/vcx/token"
)

// Unit represents a translation unit's preprocessed token stream,
// implementing ast.Unit so AST nodes can resolve tokens back to source positions.
type Unit struct {
	toks []preprocessor.Token
	// eof is returned for an out-of-range index so that a caller reading
	// past the end gets a token rather than a panic.
	eof preprocessor.Token
}

// NewUnit wraps phase 4's output. The slice is not copied and must not be
// modified afterwards; the tree indexes into it for as long as it lives.
func NewUnit(toks []preprocessor.Token) *Unit {
	u := &Unit{toks: toks}
	if n := len(toks); n > 0 {
		u.eof = toks[n-1]
	}
	return u
}

// Len is the number of tokens.
func (u *Unit) Len() int { return len(u.toks) }

// At returns one token. An index outside the stream yields the last one,
// which is what a parser reading past the end should see.
func (u *Unit) At(i ast.Tok) preprocessor.Token {
	if i < 0 || int(i) >= len(u.toks) {
		return u.eof
	}
	return u.toks[i]
}

// Text is the spelling of one token.
func (u *Unit) Text(i ast.Tok) string {
	if !i.IsValid() || int(i) >= len(u.toks) {
		return ""
	}
	return u.toks[i].Text()
}

// Kind is one token's lexical class.
func (u *Unit) Kind(i ast.Tok) token.Kind {
	if !i.IsValid() || int(i) >= len(u.toks) {
		return token.EOF
	}
	return u.toks[i].Kind
}

// Site is the phase-4 site of one token: for a token a macro produced, the
// invocation the user wrote.
func (u *Unit) Site(i ast.Tok) preprocessor.Site {
	if !i.IsValid() || int(i) >= len(u.toks) {
		return preprocessor.Site{}
	}
	return u.toks[i].Site()
}

// Span returns a site covering a node from lo to hi when both tokens share an origin.
func (u *Unit) Span(lo, hi ast.Tok) preprocessor.Site {
	s := u.Site(lo)
	if hi <= lo {
		return s
	}
	e := u.Site(hi - 1)
	if s.Origin == nil || e.Origin != s.Origin {
		return s
	}
	s.End = e.End
	return s
}

// Position resolves one token to where it was written.
func (u *Unit) Position(i ast.Tok) ast.Position {
	s := u.Site(i)
	if s.Origin == nil || s.Origin.File == nil {
		return ast.Position{Filename: s.Origin.Name()}
	}
	p := s.Origin.File.Position(s.Pos)
	return ast.Position{Filename: p.Filename, Line: p.Line, Column: p.Column}
}

// Between returns the source text between two tokens if they share the same source file.
func (u *Unit) Between(a, b ast.Tok) string {
	x, y := u.Site(a), u.Site(b)
	if x.Origin == nil || x.Origin != y.Origin || x.Origin.File == nil {
		return ""
	}
	f := x.Origin.File
	prev := token.Token{Pos: x.Pos, End: x.End}
	next := token.Token{Pos: y.Pos, End: y.End}
	return string(f.Between(prev, next))
}

// FromMacro reports whether a token was produced by macro expansion. It is
// what a diagnostic consults before deciding to print the expansion chain.
func (u *Unit) FromMacro(i ast.Tok) bool {
	if !i.IsValid() || int(i) >= len(u.toks) {
		return false
	}
	return u.toks[i].Exp != nil
}

var _ ast.Unit = (*Unit)(nil)
