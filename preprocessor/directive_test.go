package preprocessor

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/vertex-language/vcx/token"
)

// ---- harness ----

func runCfg(cfg Config, src string) (*Preprocessor, string, []Diagnostic) {
	p := New(cfg)
	toks, diags := p.Run(token.NewFile("main.cpp", []byte(src)))
	return p, norm(toks), diags
}

func run(src string) (*Preprocessor, string, []Diagnostic) {
	return runCfg(Config{}, src)
}

func diagStrings(ds []Diagnostic) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.String()
	}
	return out
}

func wantOut(t *testing.T, src, want string) {
	t.Helper()
	_, got, diags := run(src)
	if got != want {
		t.Errorf("run(%q)\n got %q\nwant %q", src, got, want)
	}
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Errorf("run(%q): unexpected %s", src, d)
		}
	}
}

func wantDiag(t *testing.T, cfg Config, src, substr string) {
	t.Helper()
	_, _, diags := runCfg(cfg, src)
	for _, d := range diags {
		if strings.Contains(d.Msg, substr) {
			return
		}
	}
	t.Errorf("run(%q): want a diagnostic containing %q, got %v", src, substr, diagStrings(diags))
}

// ---- the directive walk ----

func TestRunDefineAndText(t *testing.T) {
	wantOut(t, "#define A 1\nint x = A;\n", "int x = 1 ;")
	wantOut(t, "#define F(a) [a]\nF(2)\n", "[ 2 ]")
	wantOut(t, "#define A 1\n#undef A\nA\n", "A")
	// The null directive is not an error.
	wantOut(t, "#\nint x;\n", "int x ;")
}

// A macro invocation cannot straddle a directive: the argument list runs out
// of tokens rather than swallowing the #endif.
func TestInvocationCannotCrossDirective(t *testing.T) {
	wantDiag(t, Config{}, "#define F(a) a\nF(1\n#endif\n)\n", "unterminated argument list")
}

func TestConditionals(t *testing.T) {
	wantOut(t, "#if 1\nyes\n#else\nno\n#endif\n", "yes")
	wantOut(t, "#if 0\nyes\n#else\nno\n#endif\n", "no")
	wantOut(t, "#if 0\na\n#elif 1\nb\n#elif 1\nc\n#else\nd\n#endif\n", "b")
	wantOut(t, "#define X\n#ifdef X\nyes\n#endif\n", "yes")
	wantOut(t, "#ifndef X\nyes\n#endif\n", "yes")
	wantOut(t, "#if 1\n#if 0\na\n#endif\nb\n#endif\n", "b")
	wantOut(t, "#if 0\n#if 1\na\n#endif\nb\n#endif\n", "")
}

// C++23, P2334.
func TestElifdef(t *testing.T) {
	wantOut(t, "#define B\n#ifdef A\na\n#elifdef B\nb\n#else\nc\n#endif\n", "b")
	wantOut(t, "#ifdef A\na\n#elifdef B\nb\n#else\nc\n#endif\n", "c")
	wantOut(t, "#ifdef A\na\n#elifndef B\nb\n#else\nc\n#endif\n", "b")
	wantOut(t, "#define A\n#ifdef A\na\n#elifndef B\nb\n#endif\n", "a")
}

// A skipped group's directives are checked for nesting and nothing else, so
// an unevaluatable condition inside one is not a mistake.
func TestSkippedGroupIsNotEvaluated(t *testing.T) {
	wantOut(t, "#if 0\n#if @@@ !!!\nx\n#endif\n#endif\nok\n", "ok")
	wantOut(t, "#if 0\n#include \"nonexistent.h\"\n#endif\nok\n", "ok")
	wantOut(t, "#if 0\n#error boom\n#endif\nok\n", "ok")
}

func TestConditionalErrors(t *testing.T) {
	wantDiag(t, Config{}, "#if 1\n", "unterminated #if")
	wantDiag(t, Config{}, "#endif\n", "#endif without #if")
	wantDiag(t, Config{}, "#else\n", "#else without #if")
	wantDiag(t, Config{}, "#if 1\n#else\n#else\n#endif\n", "#else after #else")
	wantDiag(t, Config{}, "#if 1\n#else\n#elif 1\n#endif\n", "#elif after #else")
	wantDiag(t, Config{}, "#foo\n", "invalid preprocessing directive #foo")
	wantDiag(t, Config{}, "#assert x\n", "GNU directive")
}

func TestErrorAndWarningDirectives(t *testing.T) {
	wantDiag(t, Config{}, "#error something broke\n", "#error something broke")
	// #warning is ISO in C++23 (P2437), so it is a warning and nothing more.
	p, _, diags := run("#warning careful\n")
	_ = p
	if len(diags) != 1 || diags[0].Severity != token.Warn {
		t.Fatalf("want one warning, got %v", diagStrings(diags))
	}
	if !strings.Contains(diags[0].Msg, "#warning careful") {
		t.Errorf("msg = %q", diags[0].Msg)
	}
}

// ---- the controlling expression ----

func TestEvalArithmetic(t *testing.T) {
	for _, tc := range []struct {
		expr string
		want string
	}{
		{"1 + 2 == 3", "yes"},
		{"2 * 3 > 5", "yes"},
		{"(1 << 4) == 16", "yes"},
		{"-1 < 0", "yes"},
		{"~0 == -1", "yes"},
		{"!0", "yes"},
		{"1 ? 1 : 0", "yes"},
		{"0 ? 1 : 1", "yes"},
		{"7 / 2 == 3", "yes"},
		{"7 % 2 == 1", "yes"},
		{"0xff == 255", "yes"},
		{"0b1010 == 10", "yes"},
		{"010 == 8", "yes"},
		{"1'000'000 == 1000000", "yes"}, // digit separators
		{"1z == 1", "yes"},              // C++23's size_t suffix
		{"'A' == 65", "yes"},
		// The usual arithmetic conversions make -1 huge, so the comparison
		// goes the way the reader does not expect. It is the standard answer.
		{"1u > -1", ""},
		{"-1 > 1u", "yes"},
		{"0 && 1/0", ""}, // short-circuit: no division by zero reported
		{"1 || 1/0", "yes"},
	} {
		src := "#if " + tc.expr + "\nyes\n#endif\n"
		_, got, diags := run(src)
		if got != tc.want {
			t.Errorf("#if %s = %q, want %q", tc.expr, got, tc.want)
		}
		for _, d := range diags {
			if d.Severity == token.Error {
				t.Errorf("#if %s: unexpected %s", tc.expr, d)
			}
		}
	}
}

// true and false keep their values in a C++ controlling expression, where
// every other identifier becomes 0. This is the clearest single difference
// from the C preprocessor.
func TestEvalTrueFalse(t *testing.T) {
	wantOut(t, "#if true\nyes\n#endif\n", "yes")
	wantOut(t, "#if false\nyes\n#endif\n", "")
	wantOut(t, "#if !false\nyes\n#endif\n", "yes")
	// An undefined identifier is still 0.
	wantOut(t, "#if UNDEFINED_THING\nyes\n#endif\n", "")
	wantOut(t, "#if !UNDEFINED_THING\nyes\n#endif\n", "yes")
}

// The alternative tokens are operators, so they work here for free.
func TestEvalAlternativeTokens(t *testing.T) {
	wantOut(t, "#if 1 and 1\nyes\n#endif\n", "yes")
	wantOut(t, "#if 1 or 0\nyes\n#endif\n", "yes")
	wantOut(t, "#if not 0\nyes\n#endif\n", "yes")
	wantOut(t, "#if (1 bitand 1) == 1\nyes\n#endif\n", "yes")
}

func TestEvalDefined(t *testing.T) {
	wantOut(t, "#define A\n#if defined A\nyes\n#endif\n", "yes")
	wantOut(t, "#define A\n#if defined(A)\nyes\n#endif\n", "yes")
	wantOut(t, "#if !defined(A)\nyes\n#endif\n", "yes")
	// defined must not expand its operand.
	wantOut(t, "#define A B\n#define B 0\n#if defined A\nyes\n#endif\n", "yes")
	wantOut(t, "#if defined A && A > 2\nyes\n#endif\n", "")
	wantDiag(t, Config{}, "#if defined\nyes\n#endif\n", `"defined" requires an identifier`)
}

func TestEvalErrors(t *testing.T) {
	wantDiag(t, Config{}, "#if\n#endif\n", "#if with no expression")
	wantDiag(t, Config{}, "#if 1/0\n#endif\n", "division by zero")
	wantDiag(t, Config{}, "#if 1.5\n#endif\n", "floating literal")
	wantDiag(t, Config{}, "#if \"s\"\n#endif\n", "string literal")
	wantDiag(t, Config{}, "#if (1\n#endif\n", "missing ')'")
}

// ---- predefined macros ----

func TestPredefined(t *testing.T) {
	wantOut(t, "__cplusplus\n", "202302L")
	_, got, _ := runCfg(Config{Std: token.Cxx26}, "__cplusplus\n")
	if got != "202400L" {
		t.Errorf("__cplusplus under c++26 = %q", got)
	}
	wantOut(t, "#if __cplusplus >= 202302L\nyes\n#endif\n", "yes")
	wantOut(t, "__STDC_HOSTED__\n", "0")
	_, got, _ = runCfg(Config{Hosted: true}, "__STDC_HOSTED__\n")
	if got != "1" {
		t.Errorf("__STDC_HOSTED__ hosted = %q", got)
	}
}

// __LINE__ answers for the invocation, not the definition, which is why it
// cannot be an ordinary macro.
func TestLineAndFile(t *testing.T) {
	wantOut(t, "a\n__LINE__\n__LINE__\n", `a 2 3`)
	wantOut(t, "#define AT __LINE__\nAT\n\nAT\n", "2 4")
	wantOut(t, "__FILE__\n", `"main.cpp"`)
	wantOut(t, "__COUNTER__ __COUNTER__ __COUNTER__\n", "0 1 2")
	// #line moves what __LINE__ reports without moving any span.
	wantOut(t, "#line 100\n__LINE__\n", "100")
	wantOut(t, "#line 100 \"other.cpp\"\n__FILE__\n", `"other.cpp"`)
}

// ---- __has_include and __has_cpp_attribute ----

func TestHasInclude(t *testing.T) {
	fsys := fstest.MapFS{
		"exists.h": &fstest.MapFile{Data: []byte("int e;\n")},
	}
	cfg := Config{Search: []Mount{{Name: "inc", FS: fsys}}}

	for _, tc := range []struct{ src, want string }{
		{"#if __has_include(<exists.h>)\nyes\n#endif\n", "yes"},
		{"#if __has_include(<missing.h>)\nyes\n#endif\n", ""},
		{"#if __has_include(\"exists.h\")\nyes\n#endif\n", "yes"},
		// [cpp.cond]/7: defined and #ifdef treat both operators as the
		// names of defined macros, though neither is one.
		{"#if defined(__has_include)\nyes\n#endif\n", "yes"},
		{"#ifdef __has_cpp_attribute\nyes\n#endif\n", "yes"},
		{"#if defined(__has_feature)\nyes\n#endif\n", ""}, // no vendor here
	} {
		p := New(cfg)
		toks, diags := p.Run(token.NewFile("main.cpp", []byte(tc.src)))
		if got := norm(toks); got != tc.want {
			t.Errorf("%q = %q, want %q", tc.src, got, tc.want)
		}
		for _, d := range diags {
			if d.Severity == token.Error {
				t.Errorf("%q: unexpected %s", tc.src, d)
			}
		}
	}
}

func TestHasCppAttribute(t *testing.T) {
	wantOut(t, "#if __has_cpp_attribute(nodiscard) >= 201907L\nyes\n#endif\n", "yes")
	wantOut(t, "#if __has_cpp_attribute(fallthrough)\nyes\n#endif\n", "yes")
	// A vendor attribute vcx does not implement answers 0, truthfully.
	wantOut(t, "#if __has_cpp_attribute(gnu::always_inline)\nyes\n#endif\n", "")
	wantOut(t, "#if !__has_cpp_attribute(no_such_attribute)\nyes\n#endif\n", "yes")
}

// ---- #include ----

func includeFS() fstest.MapFS {
	return fstest.MapFS{
		"a.h":       &fstest.MapFile{Data: []byte("#pragma once\nint a;\n")},
		"guard.h":   &fstest.MapFile{Data: []byte("#ifndef GUARD_H\n#define GUARD_H\nint g;\n#endif\n")},
		"nested.h":  &fstest.MapFile{Data: []byte("#include \"guard.h\"\nint n;\n")},
		"sub/s.h":   &fstest.MapFile{Data: []byte("#include \"t.h\"\nint s;\n")},
		"sub/t.h":   &fstest.MapFile{Data: []byte("int tt;\n")},
		"unguarded": &fstest.MapFile{Data: []byte("int u;\n")},
	}
}

func TestInclude(t *testing.T) {
	cfg := Config{Search: []Mount{{Name: "inc", FS: includeFS()}}, TrackDeps: true}
	p := New(cfg)
	toks, diags := p.Run(token.NewFile("main.cpp", []byte(
		"#include <guard.h>\n#include <nested.h>\nint m;\n")))
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Fatalf("unexpected %s", d)
		}
	}
	// guard.h is read once: the second reach is skipped by its include guard.
	if got := norm(toks); got != "int g ; int n ; int m ;" {
		t.Errorf("got %q", got)
	}
	if d := p.Deps(); d == nil || len(d.Files) != 3 {
		t.Errorf("deps = %+v, want the target and two headers", d)
	}
}

// A quoted include looks beside the including file first; an angled one does
// not.
func TestIncludeQuotedIsRelative(t *testing.T) {
	cfg := Config{Search: []Mount{{Name: "inc", FS: includeFS()}}}
	p := New(cfg)
	toks, diags := p.Run(token.NewFile("main.cpp", []byte("#include <sub/s.h>\n")))
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Fatalf("unexpected %s", d)
		}
	}
	// s.h's `#include "t.h"` must find sub/t.h, not fail.
	if got := norm(toks); got != "int tt ; int s ;" {
		t.Errorf("got %q", got)
	}
}

func TestPragmaOnce(t *testing.T) {
	cfg := Config{Search: []Mount{{Name: "inc", FS: includeFS()}}}
	p := New(cfg)
	toks, _ := p.Run(token.NewFile("main.cpp", []byte("#include <a.h>\n#include <a.h>\n")))
	if got := norm(toks); got != "int a ;" {
		t.Errorf("got %q, want the file read once", got)
	}
}

// An unguarded header really is read twice — the optimization is an
// inference, and it must not fire without evidence.
func TestUnguardedHeaderIsReadTwice(t *testing.T) {
	cfg := Config{Search: []Mount{{Name: "inc", FS: includeFS()}}}
	p := New(cfg)
	toks, _ := p.Run(token.NewFile("main.cpp", []byte("#include <unguarded>\n#include <unguarded>\n")))
	if got := norm(toks); got != "int u ; int u ;" {
		t.Errorf("got %q", got)
	}
}

func TestIncludeErrors(t *testing.T) {
	cfg := Config{Search: []Mount{{Name: "inc", FS: includeFS()}}}
	wantDiag(t, cfg, "#include <missing.h>\n", `"missing.h" file not found`)
	wantDiag(t, cfg, "#include\n", `expected "FILENAME" or <FILENAME>`)
	wantDiag(t, cfg, "#include <unterminated\n", "missing '>'")
	wantDiag(t, cfg, "#include \"/etc/passwd\"\n", "absolute path in #include")
	// An encoding-prefixed string is a string literal, not a header name.
	wantDiag(t, cfg, "#include L\"a.h\"\n", `expected "FILENAME" or <FILENAME>`)
}

// A macro may expand to a header name ([cpp.include]/4).
func TestIncludeThroughMacro(t *testing.T) {
	cfg := Config{Search: []Mount{{Name: "inc", FS: includeFS()}}}
	p := New(cfg)
	toks, diags := p.Run(token.NewFile("main.cpp", []byte(
		"#define HDR <guard.h>\n#include HDR\n")))
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Fatalf("unexpected %s", d)
		}
	}
	if got := norm(toks); got != "int g ;" {
		t.Errorf("got %q", got)
	}
}

func TestIncludeNext(t *testing.T) {
	lower := fstest.MapFS{"limits.h": &fstest.MapFile{Data: []byte("int lower;\n")}}
	upper := fstest.MapFS{"limits.h": &fstest.MapFile{Data: []byte(
		"int upper;\n#include_next <limits.h>\n")}}
	cfg := Config{Search: []Mount{
		{Name: "up", FS: upper},
		{Name: "low", FS: lower},
	}}
	p := New(cfg)
	toks, diags := p.Run(token.NewFile("main.cpp", []byte("#include <limits.h>\n")))
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Fatalf("unexpected %s", d)
		}
	}
	if got := norm(toks); got != "int upper ; int lower ;" {
		t.Errorf("got %q", got)
	}
}

// ---- warnings that are about the program, not the language ----

func TestKeywordMacroWarning(t *testing.T) {
	wantDiag(t, Config{}, "#define new my_new\n", `"new" is a keyword`)
	// An alternative token is an operator rather than an identifier
	// ([lex.operators]), so it fails earlier and for a different reason.
	wantDiag(t, Config{}, "#define and &&\n",
		`"and" is an alternative spelling of an operator`)
	// An ordinary name is not.
	_, _, diags := run("#define MY_THING 1\n")
	for _, d := range diags {
		t.Errorf("unexpected %s", d)
	}
}

func TestRedefinitionWarning(t *testing.T) {
	wantDiag(t, Config{}, "#define A 1\n#define A 2\n", `"A" redefined`)
	// An identical redefinition is silent.
	_, _, diags := run("#define A 1\n#define A 1\n")
	for _, d := range diags {
		t.Errorf("unexpected %s", d)
	}
	// Whitespace separation is part of the identity.
	wantDiag(t, Config{}, "#define A 1 + 2\n#define A 1+2\n", `"A" redefined`)
}

func TestReservedNames(t *testing.T) {
	wantDiag(t, Config{}, "#define __cplusplus 1\n", "cannot be defined")
	wantDiag(t, Config{}, "#undef __FILE__\n", "cannot be undefined")
	wantDiag(t, Config{}, "#define defined 1\n", "cannot be defined")
}

func TestExtraTokens(t *testing.T) {
	wantDiag(t, Config{}, "#if 1\n#endif junk\n", "extra tokens at end of #endif")
	wantDiag(t, Config{}, "#define A\n#ifdef A junk\n#endif\n", "extra tokens at end of #ifdef")
}

// ---- output shape ----

func TestPragmaPassesThrough(t *testing.T) {
	_, got, _ := run("#pragma pack(push, 1)\nint x;\n")
	if got != "# pragma pack ( push , 1 ) int x ;" {
		t.Errorf("got %q, want the pragma re-minted for phase 7", got)
	}
}

func TestDiagnosticsAreSortedAndStable(t *testing.T) {
	cfg := Config{Search: []Mount{{Name: "inc", FS: fstest.MapFS{
		"bad.h": &fstest.MapFile{Data: []byte("#error in header\n")},
	}}}}
	var first []string
	for i := 0; i < 3; i++ {
		_, _, diags := runCfg(cfg, "#error before\n#include <bad.h>\n#error after\n")
		got := diagStrings(diags)
		if i == 0 {
			first = got
			continue
		}
		if strings.Join(got, "\n") != strings.Join(first, "\n") {
			t.Fatalf("run %d differs:\n%v\nvs\n%v", i, got, first)
		}
	}
	if len(first) != 3 {
		t.Errorf("got %d diagnostics: %v", len(first), first)
	}
}

// A diagnostic inside a header carries the chain that reached it.
func TestOriginChain(t *testing.T) {
	cfg := Config{Search: []Mount{{Name: "inc", FS: fstest.MapFS{
		"bad.h": &fstest.MapFile{Data: []byte("\n#error boom\n")},
	}}}}
	_, _, diags := runCfg(cfg, "#include <bad.h>\n")
	if len(diags) != 1 {
		t.Fatalf("got %v", diagStrings(diags))
	}
	d := diags[0]
	if got := d.Site.String(); got != "inc/bad.h:2:1" {
		t.Errorf("site = %q", got)
	}
	if d.Site.Origin.Parent == nil || d.Site.Origin.Parent.Name() != "main.cpp" {
		t.Error("the header's origin should name the file that included it")
	}
	if d.Site.Origin.Depth() != 1 {
		t.Errorf("depth = %d, want 1", d.Site.Origin.Depth())
	}
}
