package preprocessor

import (
	"strings"
	"testing"

	"github.com/vertex-language/vcx/token"
)

// ---- harness ----

func newPP(t *testing.T, defs ...string) *Preprocessor {
	t.Helper()
	p := New(Config{})
	for _, d := range defs {
		if strings.HasPrefix(d, "-U") {
			p.UndefText(d[2:])
			continue
		}
		if p.DefineText(d) == nil {
			t.Fatalf("could not define %q: %v", d, p.Diagnostics())
		}
	}
	return p
}

// expand scans src as a text line and macro-replaces it.
func expand(p *Preprocessor, src string) []Token {
	toks := trimEOF(p.Scan(token.NewFile("t.cpp", []byte(src+"\n"))))
	return p.Expand(toks)
}

// norm renders tokens space-separated, so a test states semantics without
// pinning whitespace.
func norm(ts []Token) string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Text()
	}
	return strings.Join(out, " ")
}

// render preserves the spacing phase 4 decided, which is what --emit ii
// prints.
func render(ts []Token) string {
	var b strings.Builder
	for i, t := range ts {
		if i > 0 && t.Spaced() {
			b.WriteByte(' ')
		}
		b.WriteString(t.Text())
	}
	return b.String()
}

func errorsOf(p *Preprocessor) []string {
	var out []string
	for _, d := range p.Diagnostics() {
		if d.Severity == token.Error {
			out = append(out, d.Msg)
		}
	}
	return out
}

func wantNorm(t *testing.T, p *Preprocessor, src, want string) {
	t.Helper()
	got := norm(expand(p, src))
	if got != want {
		t.Errorf("expand(%q)\n got %q\nwant %q", src, got, want)
	}
	if e := errorsOf(p); len(e) != 0 {
		t.Errorf("expand(%q): unexpected errors %v", src, e)
	}
}

func wantErr(t *testing.T, p *Preprocessor, src, substr string) {
	t.Helper()
	expand(p, src)
	for _, e := range errorsOf(p) {
		if strings.Contains(e, substr) {
			return
		}
	}
	t.Errorf("expand(%q): want an error containing %q, got %v", src, substr, errorsOf(p))
}

// ---- the replacement-list grammar ----

func TestDefineText(t *testing.T) {
	p := newPP(t, "NDEBUG", "VERSION=3", "MAX(a,b)=((a)>(b)?(a):(b))")

	// A bare -D takes the value 1.
	if m := p.Macros().Lookup("NDEBUG"); m == nil || norm(m.Body) != "1" {
		t.Errorf("NDEBUG body = %v", m)
	}
	if m := p.Macros().Lookup("VERSION"); m == nil || norm(m.Body) != "3" {
		t.Errorf("VERSION body = %v", m)
	}
	m := p.Macros().Lookup("MAX")
	if m == nil || m.ObjLike || len(m.Params) != 2 {
		t.Fatalf("MAX = %+v", m)
	}
	wantNorm(t, p, "MAX(1,2)", "( ( 1 ) > ( 2 ) ? ( 1 ) : ( 2 ) )")
}

// The space is the whole difference between an object-like macro whose body
// starts with a parenthesis and a function-like one.
func TestAdjacentParenDecidesKind(t *testing.T) {
	p := newPP(t)
	toks := trimEOF(p.Scan(token.NewFile("d.cpp", []byte("M (x) y\n"))))
	m := p.parseDefine(toks)
	if m == nil || !m.ObjLike {
		t.Fatalf("`M (x) y` should be object-like, got %+v", m)
	}
	toks = trimEOF(p.Scan(token.NewFile("d.cpp", []byte("N(x) y\n"))))
	if m := p.parseDefine(toks); m == nil || m.ObjLike {
		t.Fatalf("`N(x) y` should be function-like, got %+v", m)
	}
}

func TestDefineErrors(t *testing.T) {
	for _, tc := range []struct{ def, want string }{
		{"defined=1", `"defined" cannot be defined`},
		{"__cplusplus=1", `"__cplusplus" cannot be defined`},
		{"__VA_ARGS__=1", `"__VA_ARGS__" cannot be defined`},
		{"M(a,a)=a", "duplicate macro parameter"},
		{"M(a)=## a", "'##' cannot appear at the start"},
		{"M(a)=a ##", "'##' cannot appear at the end"},
		{"M(a)=# b", "'#' must be followed by a macro parameter"},
		{"M(a)=__VA_ARGS__", "__VA_ARGS__ can only appear in a variadic macro"},
		{"M(a)=__VA_OPT__(x)", "__VA_OPT__ can only appear in a variadic macro"},
		{"M(a,...)=__VA_OPT__ x", "__VA_OPT__ must be followed by a parenthesized group"},
		{"M(a,...)=__VA_OPT__(__VA_OPT__(x))", "__VA_OPT__ cannot be nested"},
		{"M(a,...)=__VA_OPT__(## x)", "'##' cannot appear at either end of a __VA_OPT__"},
		{"M(a b)=a", "expected ',' or ')'"},
		{"M(...,a)=a", "'...' must be the last macro parameter"},
	} {
		p := New(Config{})
		if m := p.DefineText(tc.def); m != nil {
			t.Errorf("DefineText(%q) should have failed", tc.def)
			continue
		}
		found := false
		for _, e := range errorsOf(p) {
			if strings.Contains(e, tc.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("DefineText(%q): want %q, got %v", tc.def, tc.want, errorsOf(p))
		}
	}
}

func TestSameDefinition(t *testing.T) {
	def := func(s string) *Macro {
		p := New(Config{})
		return p.DefineText(s)
	}
	same := []struct{ a, b string }{
		{"M=a + b", "M=a + b"},
		{"F(x)=x", "F(x)=x"},
	}
	for _, c := range same {
		if !SameDefinition(def(c.a), def(c.b)) {
			t.Errorf("%q and %q should be the same definition", c.a, c.b)
		}
	}
	diff := []struct{ a, b string }{
		{"M=a + b", "M=a+b"},   // interior whitespace is part of the identity
		{"F(x)=x", "F(y)=y"},   // parameter spelling counts
		{"F(x)=x", "F=x"},      // kind counts
		{"M=a + b", "M=a - b"}, // spelling counts
	}
	for _, c := range diff {
		if SameDefinition(def(c.a), def(c.b)) {
			t.Errorf("%q and %q should differ", c.a, c.b)
		}
	}
}

// ---- replacement ----

func TestObjectLike(t *testing.T) {
	p := newPP(t, "A=1", "B=A + A")
	wantNorm(t, p, "B", "1 + 1")
	wantNorm(t, p, "x B y", "x 1 + 1 y")
}

func TestFunctionLikeNeedsParen(t *testing.T) {
	p := newPP(t, "F(x)=[x]")
	wantNorm(t, p, "F(1)", "[ 1 ]")
	// Without a '(' the name is just a name.
	wantNorm(t, p, "F + 1", "F + 1")
	wantNorm(t, p, "&F", "& F")
}

func TestArity(t *testing.T) {
	p := newPP(t, "F(a,b)=a b", "Z()=z", "O(a)=[a]")
	wantNorm(t, p, "F(1,2)", "1 2")
	wantNorm(t, p, "Z()", "z")
	// M() with one parameter passes one empty argument, not zero.
	wantNorm(t, p, "O()", "[ ]")

	wantErr(t, newPP(t, "F(a,b)=a b"), "F(1)", `macro "F" requires 2 arguments`)
	wantErr(t, newPP(t, "F(a,b)=a b"), "F(1,2,3)", `macro "F" passed 3 arguments`)
	wantErr(t, newPP(t, "F(a,b)=a b"), "F(1,2", "unterminated argument list")
}

// Only parentheses protect a comma. Angle brackets are punctuators to phase 4.
func TestTemplateCommaIsNotProtected(t *testing.T) {
	p := newPP(t, "CHECK(x)=[x]")
	wantErr(t, p, "CHECK(std::pair<int, int>)", `macro "CHECK" passed 2 arguments`)
	// The usual workaround, which works because parentheses do protect.
	p2 := newPP(t, "CHECK(x)=[x]")
	wantNorm(t, p2, "CHECK((std::pair<int, int>))", "[ ( std :: pair < int , int > ) ]")
}

func TestHideSetStopsRecursion(t *testing.T) {
	wantNorm(t, newPP(t, "f(x)=f(x)"), "f(1)", "f ( 1 )")
	wantNorm(t, newPP(t, "a=b", "b=a"), "a", "a")
	wantNorm(t, newPP(t, "F=G", "G=F"), "F G", "F G")
	// Indirect self-reference through an argument.
	wantNorm(t, newPP(t, "f(x)=x", "g=f(g)"), "g", "g")
}

func TestArgumentPreExpansion(t *testing.T) {
	p := newPP(t, "A=1", "F(x)=[x]")
	wantNorm(t, p, "F(A)", "[ 1 ]")
	// But not when the parameter is an operand of # or ##.
	p2 := newPP(t, "A=1", "S(x)=#x")
	wantNorm(t, p2, "S(A)", `"A"`)
	p3 := newPP(t, "A=1", "C(x)=x ## _t")
	wantNorm(t, p3, "C(A)", "A_t")
}

func TestStringize(t *testing.T) {
	p := newPP(t, "S(x)=#x")
	wantNorm(t, p, "S(a + b)", `"a + b"`)
	wantNorm(t, p, "S(  a   +   b  )", `"a + b"`) // runs collapse, ends trim
	wantNorm(t, p, `S("q\n")`, `"\"q\\n\""`)      // \ and " are escaped
	wantNorm(t, p, "S()", `""`)
}

// A raw string's spelling is its raw bytes, so stringizing one escapes the
// delimiters and the backslash a reader can see in the source.
func TestStringizeRawString(t *testing.T) {
	p := newPP(t, "S(x)=#x")
	wantNorm(t, p, `S(R"(a\b)")`, `"R\"(a\\b)\""`)
}

func TestPaste(t *testing.T) {
	p := newPP(t, "CAT(a,b)=a ## b")
	wantNorm(t, p, "CAT(x,y)", "xy")
	wantNorm(t, p, "CAT(1,2)", "12")
	wantNorm(t, p, "CAT(+,+)", "++")
	// A paste with an empty operand is a placemarker, not an error.
	wantNorm(t, p, "CAT(x,)", "x")
	wantNorm(t, p, "CAT(,y)", "y")
	wantNorm(t, p, "CAT(,)", "")

	wantErr(t, newPP(t, "CAT(a,b)=a ## b"), "CAT(x,+)",
		"does not give a valid preprocessing token")
}

// ---- variadic macros ----

func TestVariadic(t *testing.T) {
	p := newPP(t, "LOG(fmt,...)=printf(fmt, __VA_ARGS__)")
	wantNorm(t, p, `LOG("%d", 1)`, `printf ( "%d" , 1 )`)
	wantNorm(t, p, `LOG("%d", 1, 2)`, `printf ( "%d" , 1 , 2 )`)
	// The variadic slot may be empty, which leaves a dangling comma — the
	// reason __VA_OPT__ exists.
	wantNorm(t, p, `LOG("hi")`, `printf ( "hi" , )`)
}

func TestVAOpt(t *testing.T) {
	p := newPP(t, "LOG(fmt,...)=printf(fmt __VA_OPT__(,) __VA_ARGS__)")
	wantNorm(t, p, `LOG("hi")`, `printf ( "hi" )`)
	wantNorm(t, p, `LOG("%d", 1)`, `printf ( "%d" , 1 )`)
	wantNorm(t, p, `LOG("%d", 1, 2)`, `printf ( "%d" , 1 , 2 )`)

	// The whole tail inside the group.
	q := newPP(t, "F(a,...)=g(a __VA_OPT__(, __VA_ARGS__))")
	wantNorm(t, q, "F(1)", "g ( 1 )")
	wantNorm(t, q, "F(1,2,3)", "g ( 1 , 2 , 3 )")

	// An empty group is legal and produces nothing either way.
	r := newPP(t, "E(...)=[__VA_OPT__()]")
	wantNorm(t, r, "E()", "[ ]")
	wantNorm(t, r, "E(1)", "[ ]")
}

// An absent __VA_OPT__ is a placemarker, so a ## beside it has nothing left
// to paste.
func TestVAOptPlacemarker(t *testing.T) {
	p := newPP(t, "F(a,...)=a ## __VA_OPT__(x)")
	wantNorm(t, p, "F(y)", "y")
	wantNorm(t, p, "F(y,1)", "yx")
}

func TestVAOptStringize(t *testing.T) {
	p := newPP(t, "S(...)=#__VA_OPT__(a __VA_ARGS__ b)")
	wantNorm(t, p, "S()", `""`)
	wantNorm(t, p, "S(1,2)", `"a 1,2 b"`)
}

// The GNU comma swallow is not ISO, and it is in enough C reached from C++
// that refusing it means refusing those headers.
func TestGNUCommaSwallow(t *testing.T) {
	p := newPP(t, "LOG(fmt,...)=printf(fmt, ## __VA_ARGS__)")
	wantNorm(t, p, `LOG("hi")`, `printf ( "hi" )`)
	wantNorm(t, p, `LOG("%d", 1)`, `printf ( "%d" , 1 )`)
}

// ---- the standard's own example ----

// [cpp.rescan]/2 — the example that exists because every part of it once
// broke a real implementation. It exercises hide sets, argument
// pre-expansion, both operators, and placemarkers at once.
//
// The expected strings are the standard's own, normalized to one space per
// token boundary. They were also checked against a second implementation:
// `cl /EP /Zc:preprocessor` — MSVC's conforming preprocessor, not its
// traditional one — agrees token for token on all four lines, and on the
// __VA_OPT__ and comma-swallow cases below.
func TestStandardRescanExample(t *testing.T) {
	p := newPP(t,
		"x=3",
		"f(a)=f(x * (a))",
		"-Ux",
		"x=2",
		"g=f",
		"z=z[0]",
		"h=g(~",
		"m(a)=a(w)",
		"w=0,1",
		"t(a)=a",
		"p()=int",
		"q(x)=x",
		"r(x,y)=x ## y",
		"str(x)=# x",
	)

	for _, tc := range []struct{ src, want string }{
		{
			"f(y+1) + f(f(z)) % t(t(g)(0) + t)(1);",
			"f ( 2 * ( y + 1 ) ) + f ( 2 * ( f ( 2 * ( z [ 0 ] ) ) ) ) % f ( 2 * ( 0 ) ) + t ( 1 ) ;",
		},
		{
			"g(x+(3,4)-w) | h 5) & m (f)^m(m);",
			"f ( 2 * ( 2 + ( 3 , 4 ) - 0 , 1 ) ) | f ( 2 * ( ~ 5 ) ) & f ( 2 * ( 0 , 1 ) ) ^ m ( 0 , 1 ) ;",
		},
		{
			"p() i[q()] = { q(1), r(2,3), r(4,), r(,5), r(,) };",
			"int i [ ] = { 1 , 23 , 4 , 5 , } ;",
		},
		{
			"char c[2][6] = { str(hello), str() };",
			`char c [ 2 ] [ 6 ] = { "hello" , "" } ;`,
		},
	} {
		got := norm(expand(p, tc.src))
		if got != tc.want {
			t.Errorf("expand(%q)\n got %s\nwant %s", tc.src, got, tc.want)
		}
	}
	if e := errorsOf(p); len(e) != 0 {
		t.Errorf("unexpected errors: %v", e)
	}
}

// ---- spacing, positions ----

// The first token of an expansion inherits the invocation's spacing, so
// --emit ii round-trips through the same rules a human would use.
func TestSpacingIsPreserved(t *testing.T) {
	p := newPP(t, "FOO=bar")
	if got := render(expand(p, "x=FOO")); got != "x=bar" {
		t.Errorf("render = %q, want %q", got, "x=bar")
	}
	if got := render(expand(p, "x = FOO")); got != "x = bar" {
		t.Errorf("render = %q, want %q", got, "x = bar")
	}
	q := newPP(t, "P(x)=x + y +x")
	if got := render(expand(q, "P(1)")); got != "1 + y +1" {
		t.Errorf("render = %q, want %q", got, "1 + y +1")
	}
}

// A diagnostic about a macro-produced token points at the invocation, which
// is what the user typed; the notes walk back to the definition.
func TestExpansionSites(t *testing.T) {
	p := newPP(t, "INNER=boom", "OUTER=INNER")
	ts := expand(p, "a OUTER b")
	var boom Token
	for _, tk := range ts {
		if tk.Text() == "boom" {
			boom = tk
		}
	}
	if boom.Exp == nil {
		t.Fatal("expanded token carries no expansion chain")
	}
	if got := boom.Site().String(); !strings.HasPrefix(got, "t.cpp:1:3") {
		t.Errorf("site = %q, want the invocation at t.cpp:1:3", got)
	}
	if root := boom.Exp.Root(); root == nil || root.Macro != "OUTER" {
		t.Errorf("root expansion = %v, want OUTER", root)
	}
	if n := len(boom.Notes()); n != 2 {
		t.Errorf("got %d notes, want one per expansion", n)
	}
}

// A token that no file contains still resolves through its Origin.
func TestGeneratedTokensHaveText(t *testing.T) {
	p := newPP(t, "CAT(a,b)=a ## b")
	ts := expand(p, "CAT(foo,bar)")
	if len(ts) != 1 || ts[0].Text() != "foobar" {
		t.Fatalf("got %v", norm(ts))
	}
	if ts[0].Origin == nil || ts[0].Origin.File != nil {
		t.Error("a pasted token should live in the generated arena")
	}
	if got := ts[0].Site().String(); got != "t.cpp:1:1" {
		t.Errorf("site = %q, want the invocation", got)
	}
}

func TestUsedFlag(t *testing.T) {
	p := newPP(t, "USED=1", "UNUSED=2")
	expand(p, "USED")
	if !p.Macros().Lookup("USED").Used {
		t.Error("USED should be marked used")
	}
	if p.Macros().Lookup("UNUSED").Used {
		t.Error("UNUSED should not be")
	}
}

func TestUndef(t *testing.T) {
	p := newPP(t, "A=1")
	wantNorm(t, p, "A", "1")
	if !p.UndefText("A") {
		t.Error("UndefText should report that A was bound")
	}
	if p.UndefText("A") {
		t.Error("undefining a name that is not defined is not an error, but is not a change")
	}
	wantNorm(t, p, "A", "A")
}
