package parser_test

// The syntax corpus: does it parse?
//
// tests/syntax holds one file per chapter of the C++23 grammar, and the only
// question asked of them is whether the parser accepts what the grammar
// allows. Nothing there has to mean anything, and several files are
// deliberately nonsense that happens to be well-formed -- a class declared
// and never defined, an operator declared for a type with no members, an
// expression statement that computes a value and drops it.
//
// Keeping that separate from tests/check is the point. A parser bug and a
// lookup bug read the same to whoever gets the diagnostic and come from
// opposite ends of the compiler, so the two questions are asked by two
// suites and a failure here names the grammar rather than the program.
//
// There is no oracle. The obvious one is cl.exe, and for phase 4 it is a
// good one -- it agrees token for token on the [cpp.rescan] examples and on
// __VA_OPT__ -- but for C++23 core-language syntax it is not: it rejects
// P2223's whitespace-before-newline splice, has no delimited escapes, and
// takes `0x1e+2` for one pp-number where the grammar makes it ill-formed. So
// each file cites the paragraph it covers, and the paragraph is what a
// reader checks it against.
//
// A file belongs here once the parser accepts it. What the parser does not
// accept is written into the file as a comment naming the production and
// saying so -- an omission that names itself is a bug report, and a file
// quietly trimmed until it passes is not.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vertex-language/vcx/parser"
	"github.com/vertex-language/vcx/preprocessor"
	"github.com/vertex-language/vcx/token"
)

func TestSyntaxCorpus(t *testing.T) {
	files, err := filepath.Glob("../tests/syntax/*.cpp")
	if err != nil || len(files) == 0 {
		t.Fatal("no files found in tests/syntax/*.cpp")
	}

	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".cpp")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}

			// Phase 4 first, then phase 7. The parser reads preprocessing
			// tokens rather than text, and one of the files is about the
			// directives themselves -- handing the scanner a `#` and
			// calling the result a parse would test the wrong thing.
			f := token.NewFile(filepath.Base(file), src)
			pp := preprocessor.New(preprocessor.Config{Std: token.Cxx23})
			toks, ppDiags := pp.Run(f)

			var b strings.Builder
			for _, d := range ppDiags {
				if d.Severity != token.Error {
					continue
				}
				b.WriteString("\n  phase 4: ")
				b.WriteString(d.Msg)
			}

			_, diags := parser.Parse(parser.NewUnit(toks), parser.DefaultMode)
			for _, d := range diags {
				if d.Severity != token.Error {
					continue
				}
				b.WriteString("\n  ")
				b.WriteString(d.String())
			}

			if b.Len() > 0 {
				t.Errorf("the parser refused a file it is expected to accept:%s", b.String())
			}
		})
	}
}
