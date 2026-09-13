package sema_test

// The plumbing the corpus runners share: one file in, the whole front end
// run over it, diagnostics out. It is the same ladder `v++ check` climbs --
// phase 4, then the parse, then the analysis -- assembled here rather than
// imported, because the root package imports this one.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vertex-language/vcx/parser"
	"github.com/vertex-language/vcx/preprocessor"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// diagnostic is a diagnostic from any phase, flattened so the runners can
// treat phase 4, phase 7 and the analysis alike.
type diagnostic struct {
	phase    string
	severity token.Severity
	message  string
}

func analyzeFile(t *testing.T, path string) []diagnostic {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var out []diagnostic
	f := token.NewFile(filepath.Base(path), src)

	pp := preprocessor.New(preprocessor.Config{Std: token.Cxx23})
	toks, ppDiags := pp.Run(f)
	for _, d := range ppDiags {
		out = append(out, diagnostic{"phase 4", d.Severity, d.Msg})
	}

	tree, parseDiags := parser.Parse(parser.NewUnit(toks), parser.DefaultMode)
	for _, d := range parseDiags {
		out = append(out, diagnostic{"parse", d.Severity, d.Message})
	}

	// LP64 is the model the corpus is written against: the sizes asserted in
	// tests/eval are that model's, and running it under another would make
	// those files fail for the right reason on the wrong target.
	_, semaDiags := sema.Analyze(tree, types.LP64())
	for _, d := range semaDiags {
		out = append(out, diagnostic{"check", d.Severity, d.Message})
	}
	return out
}

func errorsOnly(ds []diagnostic) []diagnostic {
	var errs []diagnostic
	for _, d := range ds {
		if d.severity == token.Error {
			errs = append(errs, d)
		}
	}
	return errs
}

func render(ds []diagnostic) string {
	var b strings.Builder
	for _, d := range ds {
		b.WriteString("\n  ")
		b.WriteString(d.phase)
		b.WriteString(": ")
		b.WriteString(d.message)
	}
	return b.String()
}
