// Package token defines the lexical vocabulary of C++ and source position mapping.
//
// Invariants:
//  1. Tokens hold spans, not decoded text; literals resolve through File.
//  2. Pos is per-File.
//  3. Every token span has End > Pos, except EOF.
//
// This package imports only the standard library.
package token

// Std is the language version. It reaches this package because a
// spelling's kind depends on it: contract_assert is an identifier in
// C++23 and a keyword in C++26, and ^^ is two tokens in C++23 and one
// in C++26. Everything above resolves a spelling through Lookup, so
// the version has to be where spellings become kinds.
type Std uint8

const (
	// Cxx23 is ISO/IEC 14882:2024 — the language vcx implements.
	Cxx23 Std = iota
	// Cxx26 is the working draft, feature by feature as it settles.
	Cxx26
)

func (s Std) String() string {
	switch s {
	case Cxx23:
		return "c++23"
	case Cxx26:
		return "c++26"
	}
	return "std(" + itoa(int(s)) + ")"
}

// Pos is a compact position within one File: byte offset into the
// translated text, plus one, so the zero value NoPos is invalid.
type Pos int32

// NoPos is the invalid position. Fields like a delimiter that was
// never written hold NoPos.
const NoPos Pos = 0

func (p Pos) IsValid() bool { return p > NoPos }

// Flags carry lexical facts the parser mostly ignores but that
// diagnostics, the preprocessor, and v++ fmt need. Each one records a
// choice the author made that the Kind alone cannot hold.
type Flags uint8

const (
	// FlagAdjacent: no whitespace or comment separates this token
	// from the previous one.
	FlagAdjacent Flags = 1 << iota

	// FlagNLBefore: a line terminator appeared before this token.
	FlagNLBefore

	// FlagDigraph: this punctuator was spelled as a digraph
	// (<: :> <% %> %: %:%:); Kind holds the canonical punctuator.
	FlagDigraph

	// FlagAltToken: this operator was spelled as one of the eleven
	// alternative tokens of [lex.digraph] — and, bitor, compl, kin.
	// Kind holds the operator, because the standard says these are
	// the operator and not a macro for it. The flag is what lets
	// v++ fmt print back what was written.
	FlagAltToken

	// FlagRaw: this string literal was written R"delim(...)delim".
	// Escape sequences inside it are not escapes, and the delimiter
	// is part of the span, so phases 5-6 must know before decoding.
	FlagRaw

	// FlagUserDefined: this literal carries a ud-suffix — 1_km,
	// "s"sv, 'c'_x. The suffix is inside the span and undecoded; the
	// parser needs the bit to know a literal-operator call is coming
	// and that string concatenation has a suffix to reconcile.
	FlagUserDefined
)

func (f Flags) Has(g Flags) bool { return f&g != 0 }

// Token represents a token kind and its byte span within a File.
type Token struct {
	Kind  Kind
	Flags Flags
	Pos   Pos // inclusive
	End   Pos // exclusive
}
