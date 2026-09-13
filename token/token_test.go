package token

import (
	"bytes"
	"testing"
)

// [lex.key] table 5. C++23 adds none to C++20's list, and the count is
// here so that adding a kind to the standard block is a deliberate act.
func TestKeywordCount(t *testing.T) {
	if n := int(std_keyword_end - keyword_beg - 1); n != 81 {
		t.Fatalf("got %d standard keywords, want 81", n)
	}
}

// Every kind in a named range must have a spelling. A hole here means
// an index typo in the names table, which would otherwise surface as a
// keyword that silently lexes as an identifier.
func TestEveryKindNamed(t *testing.T) {
	for k := Kind(0); k < keyword_end; k++ {
		switch k {
		case literal_beg, literal_end, punct_beg, punct_end,
			keyword_beg, std_keyword_end, cxx26_keyword_end:
			continue
		}
		if int(k) >= len(names) || names[k] == "" {
			t.Errorf("Kind(%d) has no name", k)
		}
	}
}

func TestLookup(t *testing.T) {
	for name, want := range map[string]Kind{
		// Standard keywords, including the ones C does not have.
		"class": CLASS, "typename": TYPENAME, "co_await": CO_AWAIT,
		"consteval": CONSTEVAL, "requires": REQUIRES, "char8_t": CHAR8_T,
		"reinterpret_cast": REINTERPRET_CAST, "nullptr": NULLPTR,

		// Identifiers with special meaning are identifiers. Making any
		// of these a keyword would break a conforming program.
		"module": IDENT, "import": IDENT, "final": IDENT,
		"override": IDENT, "pre": IDENT, "post": IDENT,

		// Ordinary identifiers, including near-misses.
		"T": IDENT, "Class": IDENT, "class_": IDENT, "": IDENT,

		// C keywords C++ never had.
		"restrict": IDENT, "_Bool": IDENT, "_Generic": IDENT,
	} {
		if got := Lookup(name, Cxx23); got != want {
			t.Errorf("Lookup(%q) = %v, want %v", name, got, want)
		}
	}
}

// The eleven alternative tokens of [lex.digraph] are the operator, not
// a macro for it, so they resolve to the operator's kind.
func TestAlternativeTokens(t *testing.T) {
	for spelling, want := range map[string]Kind{
		"and": LAND, "and_eq": AND_ASSIGN, "bitand": AND, "bitor": OR,
		"compl": TILDE, "not": NOT, "not_eq": NEQ, "or": LOR,
		"or_eq": OR_ASSIGN, "xor": XOR, "xor_eq": XOR_ASSIGN,

		// Near-misses stay identifiers.
		"ands": IDENT, "And": IDENT, "not_eq_": IDENT,
	} {
		if got := Lookup(spelling, Cxx23); got != want {
			t.Errorf("Lookup(%q) = %v, want %v", spelling, got, want)
		}
		if IsAltToken(spelling) != (want != IDENT) {
			t.Errorf("IsAltToken(%q) = %v", spelling, IsAltToken(spelling))
		}
	}
}

// An extension spelling of a kind already in the table resolves to that
// kind, which is what makes it the same operator rather than a name to
// be discarded.
func TestExtensionKeywordsAndAliases(t *testing.T) {
	for spelling, want := range map[string]Kind{
		"__restrict": RESTRICT, "__restrict__": RESTRICT,
		"__attribute__": ATTRIBUTE, "__declspec": DECLSPEC,
		"__int128": INT128, "__int64": INT64,
		"__try": SEH_TRY, "__except": EXCEPT, "__uuidof": UUIDOF,

		"__alignof": ALIGNOF, "__alignof__": ALIGNOF,
		"__thread": THREAD_LOCAL, "__const": CONST,
		"__volatile__": VOLATILE, "__forceinline": INLINE,
		"__asm__": ASM, "__typeof__": TYPEOF, "typeof": TYPEOF,

		// The keywords themselves are unaffected, and an identifier
		// that merely looks like one is still an identifier.
		"alignof": ALIGNOF, "inline": INLINE, "asm": ASM,
		"__alignofx": IDENT, "__int1288": IDENT, "thread": IDENT,
	} {
		if got := Lookup(spelling, Cxx23); got != want {
			t.Errorf("Lookup(%q) = %v, want %v", spelling, got, want)
		}
	}

	if !RESTRICT.IsExtension() || !SEH_TRY.IsExtension() {
		t.Error("extension keywords should report IsExtension")
	}
	if CLASS.IsExtension() || CONTRACT_ASSERT.IsExtension() {
		t.Error("standard keywords should not report IsExtension")
	}
}

// A C++26 keyword is a name a conforming C++23 program may have used,
// so the version decides.
func TestStdGatedKeywords(t *testing.T) {
	if got := Lookup("contract_assert", Cxx23); got != IDENT {
		t.Errorf("contract_assert under c++23 = %v, want IDENT", got)
	}
	if got := Lookup("contract_assert", Cxx26); got != CONTRACT_ASSERT {
		t.Errorf("contract_assert under c++26 = %v, want CONTRACT_ASSERT", got)
	}
	if got := Lookup("class", Cxx26); got != CLASS {
		t.Errorf("class under c++26 = %v", got)
	}
}

func TestPrecedence(t *testing.T) {
	// Loosest to tightest, one representative per level. The two
	// levels C does not have are SPACESHIP and PERIOD_STAR.
	order := []Kind{
		COMMA, LOR, LAND, OR, XOR, AND, EQL, LSS, SPACESHIP, SHL,
		ADD, MUL, PERIOD_STAR,
	}
	if len(order) != HighestPrec {
		t.Fatalf("%d levels listed, HighestPrec is %d", len(order), HighestPrec)
	}
	for i := 1; i < len(order); i++ {
		if order[i-1].Precedence() >= order[i].Precedence() {
			t.Errorf("%v (%d) should bind looser than %v (%d)",
				order[i-1], order[i-1].Precedence(), order[i], order[i].Precedence())
		}
	}
	for _, k := range []Kind{ASSIGN, QUESTION, INC, IDENT, NOT, SCOPE, THROW} {
		if k.Precedence() != LowestPrec {
			t.Errorf("%v.Precedence() = %d, want LowestPrec", k, k.Precedence())
		}
	}
}

func TestFastPath(t *testing.T) {
	f := NewFile("a.cpp", []byte("int x = 1;\n"))
	if f.rawLo != nil {
		t.Fatal("expected fast path (nil mapping)")
	}
	if got := string(f.Slice(f.Pos(0), f.Pos(3))); got != "int" {
		t.Errorf("Slice = %q", got)
	}
	if got := string(f.Raw(f.Pos(0), f.Pos(3))); got != "int" {
		t.Errorf("Raw = %q", got)
	}
	if len(f.Diagnostics()) != 0 {
		t.Errorf("unexpected diagnostics: %v", f.Diagnostics())
	}
}

// Trigraphs were removed by P0170 in C++17. Phase 1 must leave them
// alone — and must not warn, because it cannot tell a trigraph in code
// from three characters inside a string literal.
func TestNoTrigraphs(t *testing.T) {
	src := []byte("const char* s = \"what??!\";\nint a[1]; a??(0??) = 1;\n")
	f := NewFile("a.cpp", src)
	if !bytes.Equal(f.Text(), src) {
		t.Errorf("phase 1 changed the text: %q", f.Text())
	}
	if f.rawLo != nil {
		t.Error("a file with no backslash should take the fast path")
	}
	if ds := f.Diagnostics(); len(ds) != 0 {
		t.Errorf("unexpected diagnostics: %v", ds)
	}
}

// ??/ was a trigraph for backslash; it is not one now, so it does not
// splice. The line survives.
func TestTrigraphBackslashDoesNotSplice(t *testing.T) {
	f := NewFile("a.cpp", []byte("in??/\nt;\n"))
	if got := string(f.Text()); got != "in??/\nt;\n" {
		t.Fatalf("Text = %q, want no splice", got)
	}
}

func TestBOM(t *testing.T) {
	src := append(append([]byte{}, bom...), []byte("int x;\n")...)
	f := NewFile("a.cpp", src)
	if got := string(f.Text()); got != "int x;\n" {
		t.Fatalf("Text = %q, want the BOM removed", got)
	}
	p := f.Position(f.Pos(0))
	if p.Line != 1 || p.Column != 4 {
		t.Errorf("Position of int = %d:%d, want 1:4 (the BOM is three raw bytes)", p.Line, p.Column)
	}
	if ds := f.Diagnostics(); len(ds) != 0 {
		t.Errorf("unexpected diagnostics: %v", ds)
	}
}

func TestSplice(t *testing.T) {
	f := NewFile("a.cpp", []byte("in\\\nt x;\n"))
	if got := string(f.Text()); got != "int x;\n" {
		t.Fatalf("Text = %q", got)
	}
	if got := string(f.Slice(f.Pos(0), f.Pos(3))); got != "int" {
		t.Errorf("Slice = %q", got)
	}
	if got := string(f.Raw(f.Pos(0), f.Pos(3))); got != "in\\\nt" {
		t.Errorf("Raw = %q, want the splice widened in", got)
	}
	p := f.Position(f.Pos(2)) // the 't'
	if p.Line != 2 || p.Column != 1 {
		t.Errorf("Position of t = %d:%d, want 2:1", p.Line, p.Column)
	}
	if ds := f.Diagnostics(); len(ds) != 0 {
		t.Errorf("unexpected diagnostics: %v", ds)
	}
}

// P2223 (C++23): horizontal whitespace between the backslash and the
// newline splices, and is worth a warning because it did not before.
func TestSpliceWithTrailingWhitespace(t *testing.T) {
	f := NewFile("a.cpp", []byte("in\\  \t\nt x;\n"))
	if got := string(f.Text()); got != "int x;\n" {
		t.Fatalf("Text = %q", got)
	}
	ds := f.Diagnostics()
	if len(ds) != 1 || ds[0].Severity != Warn {
		t.Fatalf("want one Warn, got %v", ds)
	}
	// Sited on line 1, at the last character before the splice: the
	// backslash itself no longer has a translated position.
	if got := ds[0].Print(f); got != "a.cpp:1:2: warning: backslash and newline separated by whitespace" {
		t.Errorf("Print = %q", got)
	}
}

// [lex.phases]/2: if splicing produces a universal-character-name the
// program is ill-formed. This is the one way phase 2 can hand phase 3
// something the author did not write.
func TestSplicedUCNIsIllFormed(t *testing.T) {
	f := NewFile("a.cpp", []byte("int \\\\\nu00C1 = 1;\n"))
	ds := f.Diagnostics()
	if len(ds) != 1 || ds[0].Severity != Error {
		t.Fatalf("want one Error, got %v", ds)
	}
	if got := ds[0].Message; got != "universal-character-name produced by line splicing" {
		t.Errorf("Message = %q", got)
	}

	// A UCN the author actually wrote is not diagnosed.
	g := NewFile("b.cpp", []byte("int \\u00C1 = 1;\nint y = 1\\\n;\n"))
	if ds := g.Diagnostics(); len(ds) != 0 {
		t.Errorf("unexpected diagnostics: %v", ds)
	}
}

func TestSpliceAtEOF(t *testing.T) {
	f := NewFile("a.cpp", []byte("int x;\\\n"))
	ds := f.Diagnostics()
	if len(ds) != 1 || ds[0].Severity != Warn {
		t.Fatalf("want one Warn, got %v", ds)
	}
	if got := ds[0].Message; got != "backslash-newline at end of file" {
		t.Errorf("Message = %q", got)
	}
}

func TestBetween(t *testing.T) {
	f := NewFile("a.cpp", []byte("a \\\n b;\n"))
	// translated text is "a  b;\n": 'a' at [0,1), 'b' at [3,4)
	prev := Token{Kind: IDENT, Pos: f.Pos(0), End: f.Pos(1)}
	next := Token{Kind: IDENT, Pos: f.Pos(3), End: f.Pos(4)}
	if got := f.Between(prev, next); !bytes.Equal(got, []byte(" \\\n ")) {
		t.Errorf("Between = %q, want the splice kept as trivia", got)
	}
}

func TestSortDiagnostics(t *testing.T) {
	ds := []Diagnostic{
		{Pos: 5, End: 6, Message: "b"},
		{Pos: 1, End: 3, Message: "z"},
		{Pos: 5, End: 6, Message: "a"},
		{Pos: 1, End: 2, Message: "y"},
	}
	SortDiagnostics(ds)
	want := []string{"y", "z", "a", "b"}
	for i, m := range want {
		if ds[i].Message != m {
			t.Fatalf("order = %v", ds)
		}
	}
}

func TestFlags(t *testing.T) {
	f := FlagAdjacent | FlagRaw
	if !f.Has(FlagRaw) || f.Has(FlagUserDefined) {
		t.Errorf("Flags.Has is wrong: %b", f)
	}
}

func TestStdString(t *testing.T) {
	if Cxx23.String() != "c++23" || Cxx26.String() != "c++26" {
		t.Errorf("Std.String = %q, %q", Cxx23, Cxx26)
	}
}
