package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/vertex-language/vcx"
	"github.com/vertex-language/vcx/preprocessor"
	"github.com/vertex-language/vcx/scanner"
	"github.com/vertex-language/vcx/token"
)

func cmdTokens(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("tokens", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var pp ppFlags
	pp.register(fs)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "v++ tokens: one file at a time")
		return exitUsage
	}

	c, err := pp.compiler()
	if err != nil {
		fmt.Fprintln(stderr, "v++:", err)
		return exitUsage
	}
	in, err := input(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "v++:", err)
		return exitUsage
	}

	f, diags, err := c.Source(in)
	if err != nil {
		fmt.Fprintln(stderr, "v++:", err)
		return exitUsage
	}
	hadErrors := printDiags(stderr, diags)

	toks, scanDiags := scanner.Scan(f, token.Cxx23, scanner.ScanComments)
	for _, d := range scanDiags {
		diags = append(diags, vcx.Diagnostic{
			Severity: d.Severity,
			Site:     preprocessor.Site{Origin: &preprocessor.Origin{File: f}, Pos: d.Pos, End: d.End},
			Message:  d.Message,
		})
	}
	hadErrors = printDiags(stderr, diags) || hadErrors

	for _, t := range toks {
		p := f.Position(t.Pos)
		fmt.Fprintf(stdout, "%3d:%-3d %-16s", p.Line, p.Column, t.Kind)
		switch t.Kind {
		case token.IDENT, token.INT_LIT, token.FLOAT_LIT,
			token.CHAR_LIT, token.STRING_LIT, token.COMMENT, token.ILLEGAL:
			fmt.Fprintf(stdout, " %s", f.Slice(t.Pos, t.End))
		}
		fmt.Fprintf(stdout, "%s\n", flagString(t.Flags))
	}

	if hadErrors {
		return exitDiags
	}
	return exitOK
}

func flagString(f token.Flags) string {
	s := ""
	if f.Has(token.FlagAdjacent) {
		s += " [adj]"
	}
	if f.Has(token.FlagNLBefore) {
		s += " [nl]"
	}
	if f.Has(token.FlagDigraph) {
		s += " [digraph]"
	}
	return s
}
