package cli

import (
	"flag"
	"fmt"
	"io"
)

// cmdSymbols prints the mangled symbol name and source spelling for every definition in a file.
func cmdSymbols(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("symbols", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var pp ppFlags
	pp.register(fs)

	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "v++ symbols: one file at a time")
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

	syms, diags, err := c.Symbols(in)
	if err != nil {
		fmt.Fprintln(stderr, "v++:", err)
		return exitUsage
	}
	hadErrors := printDiags(stderr, diags)

	width := 0
	for _, s := range syms {
		if len(s.Name) > width {
			width = len(s.Name)
		}
	}
	for _, s := range syms {
		fmt.Fprintf(stdout, "%-*s  %s\n", width, s.Name, s.Source)
	}
	if hadErrors {
		return exitDiags
	}
	return exitOK
}
