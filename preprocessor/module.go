package preprocessor

import (
	"strings"

	"github.com/vertex-language/vcx/token"
)

// ModuleKind classifies a module directive ([cpp.pre]).
type ModuleKind uint8

const (
	// GlobalFragment is `module;` — everything up to the module declaration
	// belongs to the global module.
	GlobalFragment ModuleKind = iota
	// PrivateFragment is `module :private;`.
	PrivateFragment
	// Interface is `export module M;` — this unit provides M.
	Interface
	// Implementation is `module M;` — this unit belongs to M.
	Implementation
	// Import is `import M;` or `import :part;`.
	Import
	// ImportHeader is `import <vector>;` or `import "foo.h";`.
	ImportHeader
)

func (k ModuleKind) String() string {
	switch k {
	case GlobalFragment:
		return "global-fragment"
	case PrivateFragment:
		return "private-fragment"
	case Interface:
		return "interface"
	case Implementation:
		return "implementation"
	case Import:
		return "import"
	case ImportHeader:
		return "import-header"
	}
	return "module-kind(" + itoa(int(k)) + ")"
}

// ModuleDirective is one module or import directive, recorded as a value.
//
// This is the whole of what `v++ scan --format p1689` needs, and it is
// produced by phase 4 because phase 4 is the only phase that can produce it:
// the directives are recognized by position among preprocessing tokens, and a
// dependency scan must not require parsing the file it is scanning.
type ModuleDirective struct {
	Kind     ModuleKind
	Exported bool   // `export import M;` re-exports M
	Name     string // "app", "app:part", ":part"; empty for a header import
	Header   string // header-unit name; empty otherwise
	Angled   bool   // <vector> rather than "foo.h"
	Site     Site
}

// Modules returns the module directives the translation unit declared, in
// source order.
func (p *Preprocessor) Modules() []ModuleDirective { return p.modules }

// moduleDirective recognizes and consumes a module directive at the head of
// the reader, and reports whether it did ([cpp.pre]).
// Recognition occurs by position at line start (optionally after `export`).
func (p *Preprocessor) moduleDirective(r *reader) bool {
	head, ok := r.peek()
	if !ok || !head.StartsLine() {
		return false
	}
	if head.Kind != token.EXPORT && !head.Is("module") && !head.Is("import") {
		return false
	}

	line := r.peekLine()
	i := 0
	var d ModuleDirective
	if line[i].Kind == token.EXPORT {
		d.Exported = true
		i++
		if i >= len(line) {
			return false
		}
	}
	var isModule bool
	switch {
	case line[i].Is("module"):
		isModule = true
	case line[i].Is("import"):
	default:
		return false
	}
	d.Site = line[i].Site()
	intro := line[:i+1]
	rest := line[i+1:]

	// Commit only on a shape the grammar admits, because committing means
	// macro-replacing the rest of the line, and a line that turns out not to
	// be a directive must not be expanded twice.
	if !opensModuleTail(rest) {
		return false
	}

	tail, ok := p.parseModuleTail(&d, isModule, rest)
	if !ok {
		return false
	}
	p.modules = append(p.modules, d)
	p.out = append(p.out, intro...)
	p.out = append(p.out, tail...)
	r.i += len(line)
	return true
}

// opensModuleTail reports whether what follows the introducer can begin a
// module directive at all. An identifier may be a macro that expands to the
// name; a '=' or a '(' cannot be anything but ordinary code.
func opensModuleTail(rest []Token) bool {
	if len(rest) == 0 {
		return false
	}
	switch t := rest[0]; {
	case t.IsName(), t.Kind == token.COLON, t.Kind == token.SEMI, t.Kind == token.LSS:
		return true
	case t.Kind == token.STRING_LIT && plainString(t):
		return true
	}
	return false
}

// parseModuleTail matches [cpp.module] and [cpp.import] against everything
// after the introducer, and returns the tokens to emit in its place.
func (p *Preprocessor) parseModuleTail(d *ModuleDirective, isModule bool, rest []Token) ([]Token, bool) {
	// A header name is recovered before replacement, like #include's.
	if !isModule && (rest[0].Kind == token.LSS || rest[0].Kind == token.STRING_LIT) {
		hdr, angled, consumed, ok := p.headerName(rest, d.Site)
		if !ok {
			return nil, false
		}
		d.Kind = ImportHeader
		d.Header, d.Angled = hdr, angled

		// One HEADER_NAME token, which is the whole reason the kind exists:
		// <sys/socket.h> arrived as six punctuation tokens and only this call
		// site knew otherwise.
		spelling := `"` + hdr + `"`
		if angled {
			spelling = "<" + hdr + ">"
		}
		h := p.gen.Mint(token.HEADER_NAME, spelling)
		h.Exp = &Expansion{Macro: "import", Use: d.Site}
		out := append([]Token{h}, p.expandClosed(rest[consumed:])...)
		return out, true
	}

	if isModule {
		switch {
		case rest[0].Kind == token.SEMI:
			// `module;` opens the global module fragment. `export module;` is
			// not a directive — there is nothing to export.
			if d.Exported {
				return nil, false
			}
			d.Kind = GlobalFragment
			return rest, true

		case rest[0].Kind == token.COLON:
			if len(rest) > 1 && rest[1].Is("private") {
				if d.Exported {
					return nil, false
				}
				d.Kind = PrivateFragment
				return rest, true
			}
			// `module :part;` is not a thing — a partition declaration needs
			// its primary module name.
			return nil, false
		}
	}

	tail := p.expandClosed(rest)
	name, _, ok := moduleName(tail, 0)
	if !ok {
		what := "import"
		if isModule {
			what = "module"
		}
		p.errorf(d.Site, "expected a module name after %q", what)
		return nil, false
	}
	d.Name = name
	switch {
	case !isModule:
		d.Kind = Import
	case d.Exported:
		d.Kind = Interface
	default:
		d.Kind = Implementation
	}
	return tail, true
}

// moduleName reads `a.b.c`, `a.b:part`, or a bare `:part`, and returns the
// spelling and the index just past it. The dots and the colon are part of the
// name and carry no other meaning here.
func moduleName(line []Token, i int) (string, int, bool) {
	var b strings.Builder
	part := func() bool {
		if i >= len(line) || !line[i].IsName() {
			return false
		}
		b.WriteString(line[i].Text())
		i++
		for i+1 < len(line) && line[i].Kind == token.PERIOD && line[i+1].IsName() {
			b.WriteByte('.')
			b.WriteString(line[i+1].Text())
			i += 2
		}
		return true
	}

	if i < len(line) && line[i].Kind == token.COLON {
		// `import :part;` — a partition of the current module.
		b.WriteByte(':')
		i++
		if !part() {
			return "", 0, false
		}
		return b.String(), i, true
	}
	if !part() {
		return "", 0, false
	}
	if i < len(line) && line[i].Kind == token.COLON {
		b.WriteByte(':')
		i++
		if !part() {
			return "", 0, false
		}
	}
	return b.String(), i, true
}

// peekLine returns the tokens of the logical line at the head of the reader,
// without consuming them. The first token opens the line by construction, so
// the scan for the next one that does starts past it.
func (r *reader) peekLine() []Token {
	start := r.i
	i := r.i + 1
	for i < len(r.toks) && r.toks[i].Kind != token.EOF && !r.toks[i].StartsLine() {
		i++
	}
	return r.toks[start:i]
}
