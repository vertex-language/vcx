package preprocessor

import (
	"strings"
	"testing"

	"github.com/vertex-language/vcx/token"
)

func mods(src string) (*Preprocessor, []ModuleDirective, string, []Diagnostic) {
	p := New(Config{})
	toks, diags := p.Run(token.NewFile("m.cpp", []byte(src)))
	return p, p.Modules(), norm(toks), diags
}

// describe renders a directive the way a test can state it in one line.
func describe(d ModuleDirective) string {
	var b strings.Builder
	if d.Exported {
		b.WriteString("export ")
	}
	b.WriteString(d.Kind.String())
	switch {
	case d.Name != "":
		b.WriteString(" " + d.Name)
	case d.Header != "":
		if d.Angled {
			b.WriteString(" <" + d.Header + ">")
		} else {
			b.WriteString(` "` + d.Header + `"`)
		}
	}
	return b.String()
}

func wantMods(t *testing.T, src string, want ...string) {
	t.Helper()
	_, got, _, diags := mods(src)
	if len(got) != len(want) {
		var lines []string
		for _, d := range got {
			lines = append(lines, describe(d))
		}
		t.Fatalf("run(%q) found %v, want %v", src, lines, want)
	}
	for i := range want {
		if describe(got[i]) != want[i] {
			t.Errorf("directive %d = %q, want %q", i, describe(got[i]), want[i])
		}
	}
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Errorf("run(%q): unexpected %s", src, d)
		}
	}
}

func TestModuleDeclarations(t *testing.T) {
	wantMods(t, "module;\n", "global-fragment")
	wantMods(t, "export module app;\n", "export interface app")
	wantMods(t, "module app;\n", "implementation app")
	wantMods(t, "export module app.core.util;\n", "export interface app.core.util")
	wantMods(t, "export module app:part;\n", "export interface app:part")
	wantMods(t, "module app:part;\n", "implementation app:part")
	wantMods(t, "module :private;\n", "private-fragment")
}

func TestImports(t *testing.T) {
	wantMods(t, "import app;\n", "import app")
	wantMods(t, "import app.core;\n", "import app.core")
	wantMods(t, "import :part;\n", "import :part")
	wantMods(t, "import app:part;\n", "import app:part")
	wantMods(t, "export import app;\n", "export import app")
	wantMods(t, "import <vector>;\n", "import-header <vector>")
	wantMods(t, "import <sys/socket.h>;\n", "import-header <sys/socket.h>")
	wantMods(t, `import "local.h";`+"\n", `import-header "local.h"`)
	wantMods(t, "export import <vector>;\n", "export import-header <vector>")
}

func TestModuleUnitShape(t *testing.T) {
	src := "module;\n#include <cstdio>\nexport module app;\nimport std;\nexport import :api;\n"
	p := New(Config{})
	_, diags := p.Run(token.NewFile("m.cpp", []byte(src)))
	// The #include fails — there is no search list — but the module
	// directives around it are still all recognized.
	got := p.Modules()
	want := []string{"global-fragment", "export interface app", "import std", "export import :api"}
	if len(got) != len(want) {
		t.Fatalf("got %d directives, want %d", len(got), len(want))
	}
	for i := range want {
		if describe(got[i]) != want[i] {
			t.Errorf("directive %d = %q, want %q", i, describe(got[i]), want[i])
		}
	}
	_ = diags
}

// module and import are identifiers. What makes a directive is position and
// shape, so an ordinary use of either name is left alone.
func TestNotModuleDirectives(t *testing.T) {
	for _, src := range []string{
		"int module = 1;\n",
		"module = 2;\n",
		"int import = 1;\n",
		"import = 2;\n",
		"a.module;\n",
		"export int x;\n",  // export without module or import
		"export module;\n", // nothing to export
		"module :part;\n",  // a partition needs its primary module name
		"x; import app;\n", // not at the start of a logical line
	} {
		_, got, _, _ := mods(src)
		if len(got) != 0 {
			t.Errorf("run(%q) found %v, want no module directive", src, got)
		}
	}
}

// The introducing tokens are protected from macro replacement and the rest of
// the line is not. Checked against cl.exe /EP /Zc:preprocessor, which reports
// `module other` and `import other.core` for exactly this input.
func TestModuleNameIsExpanded(t *testing.T) {
	_, ds, out, _ := mods("#define app other\nexport module app;\n")
	if out != "export module other ;" {
		t.Errorf("output = %q, want the name replaced", out)
	}
	if len(ds) != 1 || ds[0].Name != "other" {
		t.Errorf("recorded %v, want the name after replacement", ds)
	}

	_, ds, out, _ = mods("#define app other\nimport app.core;\n")
	if out != "import other . core ;" {
		t.Errorf("output = %q", out)
	}
	if ds[0].Name != "other.core" {
		t.Errorf("name = %q, want other.core", ds[0].Name)
	}

	// A macro cannot produce the introducer, though: `module` and `import`
	// become keyword tokens, which is what makes the directive findable
	// without expanding anything.
	_, ds, _, _ = mods("#define MOD module\nMOD app;\n")
	if len(ds) != 0 {
		t.Errorf("a macro-introduced directive was accepted: %v", ds)
	}
}

// A header name is formed before replacement, for the same reason #include's
// is: the characters between the angle brackets are not tokens.
func TestHeaderNameIsNotExpanded(t *testing.T) {
	_, ds, out, _ := mods("#define vector deque\nimport <vector>;\n")
	if len(ds) != 1 || ds[0].Header != "vector" {
		t.Fatalf("recorded %v, want the header name as written", ds)
	}
	if out != "import <vector> ;" {
		t.Errorf("output = %q", out)
	}
	_ = strings.TrimSpace
}

// A header-unit import leaves one HEADER_NAME token, not three punctuation
// tokens the parser would have to reassemble.
func TestHeaderNameIsOneToken(t *testing.T) {
	p := New(Config{})
	toks, _ := p.Run(token.NewFile("m.cpp", []byte("import <sys/socket.h>;\n")))
	if len(toks) != 3 {
		t.Fatalf("got %d tokens: %q", len(toks), norm(toks))
	}
	if toks[1].Kind != token.HEADER_NAME {
		t.Errorf("token 1 is %v, want HEADER_NAME", toks[1].Kind)
	}
	if got := toks[1].Text(); got != "<sys/socket.h>" {
		t.Errorf("header name = %q, want the slash and dot intact", got)
	}
	if toks[2].Kind != token.SEMI {
		t.Errorf("token 2 is %v, want ';'", toks[2].Kind)
	}
}

// [cpp.pre]: the tokens introducing a module directive shall not come from
// macro expansion. Recognition runs before replacement, so such a line is not
// a directive at all — the rule is enforced by the order of the two steps
// rather than by a check.
func TestModuleDirectiveFromMacroIsNotOne(t *testing.T) {
	_, got, out, _ := mods("#define IMP import\nIMP app;\n")
	if len(got) != 0 {
		t.Errorf("a macro-introduced directive was accepted: %v", got)
	}
	if out != "import app ;" {
		t.Errorf("output = %q, want the line replaced as ordinary text", out)
	}
}

// Attributes after the directive are ordinary pp-tokens and are expanded.
func TestTrailingTokensAreExpanded(t *testing.T) {
	_, ds, out, _ := mods("#define DEP [[deprecated]]\nexport module app DEP;\n")
	if len(ds) != 1 || ds[0].Name != "app" {
		t.Fatalf("recorded %v", ds)
	}
	if out != "export module app [ [ deprecated ] ] ;" {
		t.Errorf("output = %q", out)
	}
}

// A module directive inside a skipped group is not one.
func TestModuleDirectiveInSkippedGroup(t *testing.T) {
	_, got, _, _ := mods("#if 0\nexport module app;\n#endif\nimport real;\n")
	if len(got) != 1 || got[0].Name != "real" {
		t.Errorf("got %v, want only the live import", got)
	}
}
