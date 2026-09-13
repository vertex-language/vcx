package scanner

import (
	"strings"
	"testing"

	"github.com/vertex-language/vcx/token"
)

func scanStd(src string, std token.Std, mode Mode) (*token.File, []token.Token, []token.Diagnostic) {
	f := token.NewFile("a.cpp", []byte(src))
	toks, diags := Scan(f, std, mode)
	return f, toks, diags
}

func scan(src string) (*token.File, []token.Token, []token.Diagnostic) {
	return scanStd(src, token.Cxx23, 0)
}

// dump renders the token kinds, dropping the trailing EOF.
func dump(toks []token.Token) string {
	var b []string
	for _, t := range toks {
		if t.Kind == token.EOF {
			break
		}
		b = append(b, t.Kind.String())
	}
	return strings.Join(b, " ")
}

func wantTokens(t *testing.T, src, want string) {
	t.Helper()
	_, toks, diags := scan(src)
	if got := dump(toks); got != want {
		t.Errorf("scan(%q) = %q, want %q", src, got, want)
	}
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Errorf("scan(%q): unexpected %s", src, d.Message)
		}
	}
}

func wantError(t *testing.T, src, substr string) {
	t.Helper()
	f, _, diags := scan(src)
	for _, d := range diags {
		if d.Severity == token.Error && strings.Contains(d.Message, substr) {
			return
		}
	}
	var got []string
	for _, d := range diags {
		got = append(got, d.Print(f))
	}
	t.Errorf("scan(%q): want an error containing %q, got %v", src, substr, got)
}

func wantClean(t *testing.T, src string) {
	t.Helper()
	f, _, diags := scan(src)
	for _, d := range diags {
		t.Errorf("scan(%q): unexpected %s", src, d.Print(f))
	}
}

// ---- the shape of the stream ----

func TestEOFAlwaysLast(t *testing.T) {
	for _, src := range []string{"", "   ", "int x;", "// comment"} {
		_, toks, _ := scan(src)
		if len(toks) == 0 || toks[len(toks)-1].Kind != token.EOF {
			t.Fatalf("scan(%q) does not end in EOF: %v", src, toks)
		}
		last := toks[len(toks)-1]
		if last.Pos != last.End {
			t.Errorf("EOF should be the one zero-width span, got [%d,%d)", last.Pos, last.End)
		}
	}
}

func TestSpansAreNonEmpty(t *testing.T) {
	_, toks, _ := scan("int x = 1'000; auto s = \"a\"_sv; @")
	for _, tk := range toks {
		if tk.Kind == token.EOF {
			continue
		}
		if tk.End <= tk.Pos {
			t.Errorf("%v has empty span [%d,%d)", tk.Kind, tk.Pos, tk.End)
		}
	}
}

func TestAdjacencyAndNewline(t *testing.T) {
	_, toks, _ := scan("a b\nc+d")
	// a, b, c, +, d
	if toks[0].Flags.Has(token.FlagAdjacent) {
		t.Error("first token should not be adjacent")
	}
	if toks[1].Flags.Has(token.FlagAdjacent) {
		t.Error("b follows a space")
	}
	if !toks[2].Flags.Has(token.FlagNLBefore) {
		t.Error("c opens a new line")
	}
	if !toks[3].Flags.Has(token.FlagAdjacent) {
		t.Error("+ is adjacent to c")
	}
}

// ---- identifiers and keywords ----

func TestKeywordsAndIdents(t *testing.T) {
	wantTokens(t, "class Foo final : public Bar {};",
		"class IDENT IDENT : public IDENT { } ;")
	wantTokens(t, "template <typename T> requires std::integral<T>",
		"template < typename IDENT > requires IDENT :: IDENT < IDENT >")
	wantTokens(t, "co_await x; co_return y;",
		"co_await IDENT ; co_return IDENT ;")
}

// The alternative tokens are the operator, and the spelling survives in
// a flag so v++ fmt can print back what was written.
func TestAlternativeTokens(t *testing.T) {
	_, toks, _ := scan("a and b bitor c compl d")
	if got := dump(toks); got != "IDENT && IDENT | IDENT ~ IDENT" {
		t.Fatalf("got %q", got)
	}
	for i, tk := range toks {
		alt := tk.Flags.Has(token.FlagAltToken)
		if want := i == 1 || i == 3 || i == 5; alt != want {
			t.Errorf("token %d (%v): FlagAltToken = %v, want %v", i, tk.Kind, alt, want)
		}
	}
}

func TestUTF8Identifier(t *testing.T) {
	wantTokens(t, "int café = 1; int 変数 = 2;",
		"int IDENT = INT_LIT ; int IDENT = INT_LIT ;")
	wantClean(t, "int café = 1;")
	wantError(t, "int caf\xc3 = 1;", "invalid UTF-8")
}

func TestUCNIdentifier(t *testing.T) {
	wantTokens(t, "int \\u00C1 = 1;", "int IDENT = INT_LIT ;")
	wantError(t, "int \\u00C = 1;", "malformed universal character name")
	wantError(t, "int a\\b;", `stray '\'`)
}

// ---- punctuators ----

func TestCxxPunctuators(t *testing.T) {
	wantTokens(t, "a <=> b", "IDENT <=> IDENT")
	wantTokens(t, "a.*p; a->*p;", "IDENT .* IDENT ; IDENT ->* IDENT ;")
	wantTokens(t, "A::B::c", "IDENT :: IDENT :: IDENT")
	wantTokens(t, "a<=b>=c", "IDENT <= IDENT >= IDENT")
	wantTokens(t, "x>>=1; y<<=2;", "IDENT >>= INT_LIT ; IDENT <<= INT_LIT ;")
	// >> stays one token; the parser splits it for template arguments.
	wantTokens(t, "vector<vector<int>>", "IDENT < IDENT < int >>")
}

// [lex.pptoken]/3.2 — the rule that keeps vector<::std::string> from
// opening with a bracket.
func TestLessColonColonDisambiguation(t *testing.T) {
	wantTokens(t, "A<::B>", "IDENT < :: IDENT >")
	wantTokens(t, "A<::>", "IDENT [ ]")

	// Followed by ':' the exception does not apply, so <: is the
	// digraph for [ — and the bracket is then genuinely unbalanced,
	// which is the point of the rule the other cases exercise.
	_, toks, _ := scan("A<:::B>")
	if got := dump(toks); got != "IDENT [ :: IDENT >" {
		t.Errorf("A<:::B> = %q", got)
	}
}

func TestDigraphs(t *testing.T) {
	_, toks, _ := scan("<% x<:0:> %>")
	if got := dump(toks); got != "{ IDENT [ INT_LIT ] }" {
		t.Fatalf("got %q", got)
	}
	for i, tk := range toks {
		if tk.Kind == token.EOF {
			break
		}
		if dg := tk.Flags.Has(token.FlagDigraph); dg != (i != 1 && i != 3) {
			t.Errorf("token %d (%v): FlagDigraph = %v", i, tk.Kind, dg)
		}
	}
}

// ^^ is one token under C++26 and two under C++23, because `a ^ ^b`
// written tightly is legal in both and maximal munch cannot decide.
func TestReflectionOperatorIsStdGated(t *testing.T) {
	_, toks, _ := scanStd("a ^^ b", token.Cxx23, 0)
	if got := dump(toks); got != "IDENT ^ ^ IDENT" {
		t.Errorf("under c++23: %q", got)
	}
	_, toks, _ = scanStd("a ^^ b", token.Cxx26, 0)
	if got := dump(toks); got != "IDENT ^^ IDENT" {
		t.Errorf("under c++26: %q", got)
	}
}

func TestIllegalCharacter(t *testing.T) {
	wantError(t, "int x = @;", "illegal character")
	// Under ScanPP it is a legal pp-token with no conversion, and
	// silent: an excluded #if group may hold one.
	_, _, diags := scanStd("int x = @;", token.Cxx23, ScanPP)
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Errorf("ScanPP should not report: %s", d.Message)
		}
	}
}

// ---- numbers ----

func TestNumericLiterals(t *testing.T) {
	for _, src := range []string{
		"0", "1", "0x1f", "0X1F", "0b1011", "007",
		"1'000'000", "0x1'000", "0b1010'1010",
		"1.0", "1.", ".5", "1e10", "1e-10", "1.5E+3",
		"0x1p3", "0x1.8p3", "0x.8p1",
		"1u", "1U", "1l", "1LL", "1ull", "1ULL", "1z", "1uz", "1ZU",
		"1.0f", "1.0F", "1.0l", "1.0L",
		"1.0f16", "1.0F128", "1.0bf16",
		"0xffffffffffffffffui64", "1i32",
	} {
		wantClean(t, "auto v = "+src+";")
	}
}

func TestNumericClassification(t *testing.T) {
	for src, want := range map[string]string{
		"0":       "INT_LIT",
		"0b1011":  "INT_LIT",
		"1'000":   "INT_LIT",
		"1.0":     "FLOAT_LIT",
		"1e10":    "FLOAT_LIT",
		"0x1p3":   "FLOAT_LIT",
		"10.12.1": "FLOAT_LIT", // a pp-number no phase gives a value
	} {
		_, toks, _ := scan(src)
		if got := dump(toks); got != want {
			t.Errorf("scan(%q) = %q, want %q", src, got, want)
		}
	}
}

func TestUserDefinedNumericLiteral(t *testing.T) {
	_, toks, diags := scan("auto d = 100_km + 2.5_mi;")
	if got := dump(toks); got != "auto IDENT = INT_LIT + FLOAT_LIT ;" {
		t.Fatalf("got %q", got)
	}
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Errorf("unexpected %s", d.Message)
		}
	}
	if !toks[3].Flags.Has(token.FlagUserDefined) || !toks[5].Flags.Has(token.FlagUserDefined) {
		t.Error("both literals should carry FlagUserDefined")
	}
	// A standard suffix is not a ud-suffix.
	_, toks, _ = scan("1ull")
	if toks[0].Flags.Has(token.FlagUserDefined) {
		t.Error("1ull is not a user-defined literal")
	}
}

func TestNumericErrors(t *testing.T) {
	wantError(t, "auto v = 0779;", "invalid digit in octal literal")
	wantError(t, "auto v = 0x;", "hexadecimal literal requires at least one digit")
	wantError(t, "auto v = 0b;", "binary literal requires at least one digit")
	wantError(t, "auto v = 1e;", "exponent requires digits")
	wantError(t, "auto v = 0x1.8;", "requires a binary exponent")
	wantError(t, "auto v = 1'000'u;", "digit separator must appear between two digits")
	// A ' the pp-number grammar cannot take ends the number, and what
	// follows opens a character-literal. cl.exe rejects this too, as
	// "bad suffix on number" — it takes the quote into the number
	// where the grammar does not. Both refuse it; only the message
	// differs.
	wantError(t, "auto v = 1'000';", "unterminated character literal")
	// The pp-number grammar takes the sign after e, so this is one
	// token that classifies as nothing — which is what g++ does.
	wantError(t, "auto v = 0x1e+2;", "invalid suffix")
	// Value-level reports defer under ScanPP.
	_, _, diags := scanStd("auto v = 0779;", token.Cxx23, ScanPP)
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Errorf("ScanPP should defer: %s", d.Message)
		}
	}
}

// ---- character and string literals ----

func TestLiteralPrefixes(t *testing.T) {
	wantTokens(t, `"a" L"a" u8"a" u"a" U"a"`,
		"STRING_LIT STRING_LIT STRING_LIT STRING_LIT STRING_LIT")
	wantTokens(t, `'a' L'a' u8'a' u'a' U'a'`,
		"CHAR_LIT CHAR_LIT CHAR_LIT CHAR_LIT CHAR_LIT")
	// u8 alone is an identifier, and so is R.
	wantTokens(t, "u8 R L u U", "IDENT IDENT IDENT IDENT IDENT")
	// The prefix is inside the span.
	f, toks, _ := scan(`u8"hi"`)
	if got := string(f.Slice(toks[0].Pos, toks[0].End)); got != `u8"hi"` {
		t.Errorf("span = %q, want the prefix included", got)
	}
}

func TestUserDefinedStringAndChar(t *testing.T) {
	_, toks, _ := scan(`auto s = "abc"sv; auto c = 'x'_ch;`)
	if got := dump(toks); got != "auto IDENT = STRING_LIT ; auto IDENT = CHAR_LIT ;" {
		t.Fatalf("got %q", got)
	}
	if !toks[3].Flags.Has(token.FlagUserDefined) {
		t.Error(`"abc"sv should carry FlagUserDefined`)
	}
	if !toks[8].Flags.Has(token.FlagUserDefined) {
		t.Error(`'x'_ch should carry FlagUserDefined`)
	}
	// Separated by a space, the suffix is its own identifier.
	_, toks, _ = scan(`"abc" sv`)
	if got := dump(toks); got != "STRING_LIT IDENT" {
		t.Errorf("got %q", got)
	}
	if toks[0].Flags.Has(token.FlagUserDefined) {
		t.Error("a separated suffix is not a ud-suffix")
	}
}

func TestLiteralErrors(t *testing.T) {
	wantError(t, `auto s = "abc;`, "unterminated string literal")
	wantError(t, "auto c = 'a;", "unterminated character literal")
	wantError(t, "auto c = '';", "empty character literal")
	wantError(t, `auto s = "a\q";`, "unknown escape sequence")
	wantError(t, `auto s = "a\x";`, `\x escape requires hexadecimal digits`)
}

func TestEscapes(t *testing.T) {
	for _, src := range []string{
		`"\n\t\\\"\'\?\a\b\f\r\v"`,
		`"\0\1\12\123"`,
		`"\x41\xff"`,
		`"Á\U0001F600"`,
		// C++23 delimited and named escapes (P2290, P2071).
		`"\x{41}"`, `"\o{101}"`, `"\u{1F600}"`,
		`"\N{LATIN SMALL LETTER A}"`,
	} {
		wantClean(t, "auto s = "+src+";")
	}
	wantError(t, `auto s = "\o101";`, `\o escape requires a braced sequence`)
	wantError(t, `auto s = "\x{}";`, `\x escape requires hexadecimal digits`)
	wantError(t, `auto s = "\u{41";`, `unterminated \u escape`)
	wantError(t, `auto s = "\N{}";`, `\N escape requires a character name`)
}

// ---- raw strings ----

func TestRawString(t *testing.T) {
	f, toks, diags := scan(`auto s = R"(no \n escape "here")";`)
	if got := dump(toks); got != "auto IDENT = STRING_LIT ;" {
		t.Fatalf("got %q", got)
	}
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Fatalf("unexpected %s", d.Print(f))
		}
	}
	lit := toks[3]
	if !lit.Flags.Has(token.FlagRaw) {
		t.Error("missing FlagRaw")
	}
	if got := string(f.Slice(lit.Pos, lit.End)); got != `R"(no \n escape "here")"` {
		t.Errorf("span = %q", got)
	}
}

func TestRawStringWithDelimiter(t *testing.T) {
	wantClean(t, `auto s = R"cpp(int x = ")"; )cpp";`)
	_, toks, _ := scan(`auto s = R"cpp(a)cpp" "b";`)
	if got := dump(toks); got != "auto IDENT = STRING_LIT STRING_LIT ;" {
		t.Errorf("got %q", got)
	}
}

// Inside a raw string the phase 1-2 transformations are reverted
// ([lex.pptoken]/3), so a backslash-newline is two characters of
// content and not a splice. The token's content is Raw, not Slice.
func TestRawStringRevertsSplicing(t *testing.T) {
	src := "auto s = R\"(a\\\nb)\";\n"
	f, toks, diags := scan(src)
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Fatalf("unexpected %s", d.Print(f))
		}
	}
	if got := dump(toks); got != "auto IDENT = STRING_LIT ;" {
		t.Fatalf("got %q", got)
	}
	lit := toks[3]
	if got := string(f.Raw(lit.Pos, lit.End)); got != "R\"(a\\\nb)\"" {
		t.Errorf("Raw = %q, want the backslash and newline kept", got)
	}
	if got := string(f.Slice(lit.Pos, lit.End)); got != `R"(ab)"` {
		t.Errorf("Slice = %q, want phase 2's view", got)
	}
	// The token after it is still in the right place.
	if got := string(f.Raw(toks[4].Pos, toks[4].End)); got != ";" {
		t.Errorf("token after the raw string = %q", got)
	}
}

func TestRawStringErrors(t *testing.T) {
	wantError(t, `auto s = R"(abc;`, "unterminated raw string literal")
	wantError(t, `auto s = R"de lim(x)de lim";`, "invalid character in raw string delimiter")
	wantError(t, `auto s = R"01234567890123456(x)01234567890123456";`,
		"raw string delimiter is 17 characters")
}

func TestRawStringUserDefined(t *testing.T) {
	_, toks, _ := scan(`auto s = R"(x)"_raw;`)
	lit := toks[3]
	if !lit.Flags.Has(token.FlagRaw) || !lit.Flags.Has(token.FlagUserDefined) {
		t.Errorf("flags = %b, want raw and user-defined", lit.Flags)
	}
}

// ---- comments, directives, brackets ----

func TestComments(t *testing.T) {
	wantTokens(t, "a // line\nb /* block */ c", "IDENT IDENT IDENT")
	wantError(t, "a /* unterminated", "unterminated /* comment")

	_, toks, _ := scanStd("a // x\nb", token.Cxx23, ScanComments)
	if got := dump(toks); got != "IDENT COMMENT IDENT" {
		t.Errorf("with ScanComments: %q", got)
	}
}

func TestDirectiveLine(t *testing.T) {
	f, _, diags := scan("#define X 1\nint x;\n")
	if len(diags) != 1 || diags[0].Severity != token.Warn {
		t.Fatalf("want one warning, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, "run the preprocessor first") {
		t.Errorf("message = %q", diags[0].Message)
	}
	_ = f

	// A line marker is what .ii input is made of; it is not a mistake.
	_, _, diags = scan("# 42 \"foo.cpp\" 3\nint x;\n")
	if len(diags) != 0 {
		t.Errorf("line marker should be silent, got %v", diags)
	}

	// Under ScanPP the directive is tokens, because the preprocessor
	// needs them.
	_, toks, diags := scanStd("#define X 1\n", token.Cxx23, ScanPP)
	if got := dump(toks); got != "# IDENT IDENT INT_LIT" {
		t.Errorf("ScanPP: %q", got)
	}
	if len(diags) != 0 {
		t.Errorf("ScanPP should be silent, got %v", diags)
	}
}

func TestBrackets(t *testing.T) {
	wantClean(t, "int f() { return a[0]; }")
	wantError(t, "int f() { return 1;", "unclosed { at end of file")
	wantError(t, "int f(] {}", "unclosed (, closed by ]")
	wantError(t, "int f() } {}", "unmatched }")

	// Off under ScanPP: a macro may open a brace it does not close.
	_, _, diags := scanStd("#define BEGIN {\n", token.Cxx23, ScanPP)
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Errorf("ScanPP should not check brackets: %s", d.Message)
		}
	}
}

// ---- a realistic line ----

func TestRealisticSource(t *testing.T) {
	src := `
export module app;
import std;

template <std::integral T>
constexpr auto square(T v) noexcept -> T { return v * v; }

int main() {
    constexpr auto n = 1'000uz;
    auto s = R"(raw \n)"sv;
    return square(n) <=> 0 != 0 ? 1 : 0;
}
`
	f, toks, diags := scan(src)
	for _, d := range diags {
		t.Errorf("unexpected %s", d.Print(f))
	}
	if len(toks) < 40 {
		t.Errorf("only %d tokens", len(toks))
	}
	// export and module are different things: one is a keyword, the
	// other an identifier with special meaning.
	if toks[0].Kind != token.EXPORT || toks[1].Kind != token.IDENT {
		t.Errorf("export module = %v %v, want export IDENT", toks[0].Kind, toks[1].Kind)
	}
}
