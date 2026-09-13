package preprocessor

import (
	"strings"

	"github.com/vertex-language/vcx/token"
)

// Token represents a preprocessing token with origin, hide-set, and expansion tracking.
type Token struct {
	Kind  token.Kind
	Flags token.Flags
	Pos   token.Pos
	End   token.Pos

	Origin *Origin
	Hide   *HideSet
	Exp    *Expansion
}

// Text returns the token's spelling.
//
// For an ordinary token that is token.File.Slice — the translated bytes, what
// the scanner read — because a macro name is compared against the spelling
// after line splicing, that being what the scanner made a token out of.
//
// A raw string literal is the exception, and it is the reason this is not a
// one-liner. Inside one, phases 1 and 2 are reverted ([lex.pptoken]/3), so
// its spelling is the raw bytes and Slice would hand back a literal with a
// splice silently removed from the middle of its contents. FlagRaw says which
// rule applies; token.File.Raw supplies the answer.
func (t Token) Text() string {
	switch {
	case t.Origin == nil:
		return ""
	case t.Origin.File != nil:
		if t.Flags.Has(token.FlagRaw) {
			return string(t.Origin.File.Raw(t.Pos, t.End))
		}
		return string(t.Origin.File.Slice(t.Pos, t.End))
	case t.Origin.Gen != nil:
		return t.Origin.Gen.slice(t.Pos, t.End)
	default:
		return ""
	}
}

// Spaced reports whether the token was separated from the one before it by
// whitespace or a comment. It is the inverse of token.FlagAdjacent, and it is
// what --emit ii consults when deciding whether to print a space.
func (t Token) Spaced() bool { return !t.Flags.Has(token.FlagAdjacent) }

// StartsLine reports whether a line terminator preceded the token. A '#' with
// this flag set opens a directive; a '#' without it is the punctuator.
//
// This is exactly why scanner.ScanPP seeds nlBefore true: a '#' in column 1
// of line 1 opens a logical line, and nothing precedes it to say so.
func (t Token) StartsLine() bool { return t.Flags.Has(token.FlagNLBefore) }

// Is reports whether the token is an identifier spelled name.
//
// Directive keywords (define, ifdef, include, …) are not token kinds — they
// are ordinary identifiers that mean something only after a line-opening '#'.
// So are module and import, which is why they were kept out of token.Lookup:
// [cpp.pre] gives them meaning by position, and a program with a variable
// called import is well-formed.
//
// C++ keywords are checked too: `#define int x` is ill-formed, not a parse
// failure, and `#if sizeof` is 0 rather than an error, so both paths need to
// recognize a keyword by spelling.
func (t Token) Is(name string) bool {
	if t.Kind != token.IDENT && !t.Kind.IsKeyword() {
		return false
	}
	return t.Text() == name
}

// IsName reports whether the token may be used as a macro name or parameter:
// an identifier, or a keyword (which is ill-formed to define, but a violation
// the caller must diagnose rather than fail to parse).
func (t Token) IsName() bool {
	return t.Kind == token.IDENT || t.Kind.IsKeyword()
}

// IsPPNumber reports whether the token is a preprocessing number.
//
// A pp-number is not yet a literal: 0779 and 1e+ are legal pp-numbers and
// illegal literals, and #if 0 may legally hide either. scanner.scanNumber
// already consumes the whole run as one token, which is the pp-number rule;
// what it additionally does is classify and validate. Phase 4 keeps the
// classification and drops the diagnostics sited inside excluded groups.
func (t Token) IsPPNumber() bool {
	return t.Kind == token.INT_LIT || t.Kind == token.FLOAT_LIT
}

// Gen provides the arena and position space for synthesized tokens (# and ##).
type Gen struct {
	buf    []byte
	origin *Origin
}

// NewGen returns an empty generated arena.
func NewGen() *Gen {
	g := &Gen{buf: make([]byte, 0, 256)}
	g.origin = &Origin{Gen: g}
	return g
}

// Origin returns the arena's position space, for tokens minted from it.
func (g *Gen) Origin() *Origin { return g.origin }

// slice mirrors token.File.Slice: Pos is offset+1, so the zero value is NoPos
// and a real position at offset 0 is distinguishable from it.
func (g *Gen) slice(pos, end token.Pos) string {
	lo, hi := int(pos)-1, int(end)-1
	if lo < 0 || hi > len(g.buf) || lo > hi {
		return ""
	}
	return string(g.buf[lo:hi])
}

// intern appends s and returns its span.
func (g *Gen) intern(s string) (pos, end token.Pos) {
	pos = token.Pos(len(g.buf) + 1)
	g.buf = append(g.buf, s...)
	return pos, token.Pos(len(g.buf) + 1)
}

// Mint appends the spelling and returns a token of the given kind spanning
// it. The caller supplies flags, hide set and expansion chain.
func (g *Gen) Mint(kind token.Kind, spelling string) Token {
	pos, end := g.intern(spelling)
	return Token{Kind: kind, Pos: pos, End: end, Origin: g.origin}
}

// Stringize implements the '#' preprocessor operator ([cpp.stringize]/2),
// producing a STRING_LIT token from macro argument tokens.
func (g *Gen) Stringize(arg []Token) Token {
	var b strings.Builder
	b.WriteByte('"')
	for i, t := range arg {
		if i > 0 && t.Spaced() {
			b.WriteByte(' ')
		}
		s := t.Text()
		if t.Kind == token.STRING_LIT || t.Kind == token.CHAR_LIT {
			for j := 0; j < len(s); j++ {
				if s[j] == '\\' || s[j] == '"' {
					b.WriteByte('\\')
				}
				b.WriteByte(s[j])
			}
			continue
		}
		b.WriteString(s)
	}
	b.WriteByte('"')
	return g.Mint(token.STRING_LIT, b.String())
}

// Paste appends the concatenation of two spellings and returns the span,
// which the caller must re-scan: [cpp.concat]/3 requires the result be a
// single valid preprocessing token, and only the scanner can say whether it
// is.
func (g *Gen) Paste(l, r Token) (pos, end token.Pos) {
	return g.intern(l.Text() + r.Text())
}

// Buffer exposes the arena's bytes so a diagnostic can be rendered against a
// generated token, and so tests can assert on what expansion built.
func (g *Gen) Buffer() []byte { return g.buf }
