package cli

import (
	"bytes"
	"fmt"
	"io"

	"github.com/vertex-language/vcx"
	"github.com/vertex-language/vcx/preprocessor"
	"github.com/vertex-language/vcx/token"
)

// printDiags renders each diagnostic with caret line snippet and reports whether any was an error.
func printDiags(w io.Writer, diags []vcx.Diagnostic) bool {
	for _, d := range diags {
		fmt.Fprintln(w, d.String())
		printSiteSnippet(w, d.Site)
		for _, n := range d.Notes {
			fmt.Fprintf(w, "%s: note: %s\n", vcx.SiteString(n.Site), n.Msg)
			printSiteSnippet(w, n.Site)
		}
		printIncludeChain(w, d.Site)
	}
	return vcx.HasErrors(diags)
}

func printSiteSnippet(w io.Writer, s preprocessor.Site) {
	if s.Origin == nil || s.Origin.File == nil || !s.Pos.IsValid() {
		return
	}
	printSnippetAt(w, s.Origin.File, s.Pos, s.End)
}

func printIncludeChain(w io.Writer, s preprocessor.Site) {
	if s.Origin == nil {
		return
	}
	for child := s.Origin; child.Parent != nil; child = child.Parent {
		parent := child.Parent
		if parent.File == nil {
			continue
		}
		p := parent.File.Position(child.IncludePos)
		fmt.Fprintf(w, "    in file included from %s:%d\n", parent.Name(), p.Line)
	}
}

func printSnippetAt(w io.Writer, f *token.File, pos, end token.Pos) {
	src := f.Source()
	p := f.Position(pos)

	lo := p.Offset - (p.Column - 1)
	hi := p.Offset
	for hi < len(src) && src[hi] != '\n' && src[hi] != '\r' {
		hi++
	}
	line := src[lo:hi]

	width := len(f.Raw(pos, end))
	if p.Column-1+width > len(line) {
		width = len(line) - (p.Column - 1)
	}
	if width < 1 {
		width = 1
	}

	pad := make([]byte, p.Column-1)
	for i := range pad {
		if line[i] == '\t' {
			pad[i] = '\t'
		} else {
			pad[i] = ' '
		}
	}

	fmt.Fprintf(w, "    %s\n    %s%s\n", line, pad, bytes.Repeat([]byte("^"), width))
}
