package parser

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/preprocessor"
	"github.com/vertex-language/vcx/token"
)

// Mode controls optional parser behavior.
type Mode uint

const (
	// ParseComments retains comment tokens on the File.
	ParseComments Mode = 1 << iota

	// SkipBodies skips function bodies balanced, not parsed.
	SkipBodies

	// Tolerant keeps parsing past the resync budget.
	Tolerant
)

const DefaultMode Mode = 0

// Diagnostic is one report from parsing.
type Diagnostic struct {
	Severity token.Severity
	Site     preprocessor.Site
	Message  string
	Notes    []Note
}

func (d Diagnostic) String() string {
	if d.Site.Origin != nil && d.Site.Origin.File != nil {
		p := d.Site.Origin.File.Position(d.Site.Pos)
		return fmt.Sprintf("%s:%d:%d: %s: %s", p.Filename, p.Line, p.Column, d.Severity, d.Message)
	}
	return fmt.Sprintf("%s: %s", d.Severity, d.Message)
}

// Note is a secondary position attached below a diagnostic.
type Note struct {
	Site    preprocessor.Site
	Message string
}

// SortDiagnostics orders diagnostics by position, then extent, then message.
func SortDiagnostics(ds []Diagnostic) {
	sort.SliceStable(ds, func(i, j int) bool {
		if ds[i].Site.Pos != ds[j].Site.Pos {
			return ds[i].Site.Pos < ds[j].Site.Pos
		}
		if ds[i].Site.End != ds[j].Site.End {
			return ds[i].Site.End < ds[j].Site.End
		}
		return ds[i].Message < ds[j].Message
	})
}

const (
	defaultMaxResync = 100
	defaultMaxDepth  = 1000
)

type parser struct {
	u    *Unit
	cur  ast.Tok
	mode Mode

	// memberSelector is set while the name after `.` or `->` is parsed:
	// a member's name is the class's, not one the name table knows.
	memberSelector bool

	// qualifiedPart is set while the component after a `::` is parsed,
	// whose meaning is the named scope's rather than the current one's.
	qualifiedPart  bool
	diags          []Diagnostic
	errTok         ast.Tok
	resyncs        int
	maxResync      int
	depth          int
	maxDepth       int
	inTemplateArgs int

	// halfGtr tracks when the first '>' of a '>>' token closed a template argument list.
	halfGtr bool

	// names is what the parser has learned about identifiers: which are
	// types and which are not. See names.go.
	names []map[string]nameKind

	// declaringTemplate is set between a template-head and the name it
	// declares, and declaredTemplate holds every name a template-head
	// declared, in any scope, for the `<` after it (see namesATemplate).
	declaringTemplate bool
	declaredTemplate  map[string]bool

	// namespaces holds each named namespace's name scope by qualified
	// name, so that reopening it reopens the scope; nsPath is the
	// namespace the parser is in.
	namespaces    map[string]map[string]nameKind
	nsPath        []string
	templateHeads int // template-heads being read
	templateOuter int // the names scope enclosing the outermost

	// pack is the alignment ceiling #pragma pack has in effect, packStack
	// what push saved, and packs the record handed to the tree.
	pack      int64
	packStack []int64
	packs     []ast.PackAt

	// inNewTypeId indicates parsing the type-id of a new-expression.
	inNewTypeId bool

	// inConversionTypeId indicates parsing the type of a conversion-function-id.
	inConversionTypeId bool

	// inForRangeDecl indicates parsing a for-range-declaration.
	inForRangeDecl bool
}

func newParser(u *Unit, mode Mode) *parser {
	return &parser{
		u:         u,
		cur:       0,
		mode:      mode,
		errTok:    ast.NoTok,
		maxResync: defaultMaxResync,
		maxDepth:  defaultMaxDepth,
		names:     []map[string]nameKind{{}},
	}
}

func (p *parser) pos() ast.Tok {
	return p.cur
}

func (p *parser) atEOF() bool {
	return int(p.cur) >= p.u.Len() || p.u.Kind(p.cur) == token.EOF
}

func (p *parser) peek() token.Kind {
	if p.halfGtr {
		return token.GTR
	}
	return p.u.Kind(p.cur)
}

func (p *parser) peekAt(offset int) token.Kind {
	if offset == 0 {
		return p.peek()
	}
	idx := p.cur + ast.Tok(offset)
	return p.u.Kind(idx)
}

func (p *parser) peekTok(offset int) ast.Tok {
	return p.cur + ast.Tok(offset)
}

func (p *parser) text(t ast.Tok) string {
	return p.u.Text(t)
}

func (p *parser) next() ast.Tok {
	t := p.cur
	p.halfGtr = false
	if !p.atEOF() {
		p.cur++
	}
	return t
}

// skipPragmaLines handles or skips `#pragma` directive lines passed through by the preprocessor.
func (p *parser) skipPragmaLines() {
	for p.peek() == token.HASH && p.u.Kind(p.cur+1) == token.IDENT && p.u.Text(p.cur+1) == "pragma" {
		p.next() // #
		p.next() // pragma
		if p.peek() == token.IDENT && p.text(p.cur) == "pack" {
			p.pragmaPack()
		}
		for !p.atEOF() && !p.u.At(p.cur).StartsLine() {
			p.next()
		}
	}
}

// pragmaPack reads `#pragma pack(...)` and records what it did to the
// alignment ceiling from here on.
//
// The forms: `pack(n)` sets the ceiling, `pack()` restores the default,
// `pack(push)` saves the current, `pack(push, n)` saves and sets,
// `pack(pop)` restores the last saved, and an identifier in either may
// name the entry -- `pack(push, id, n)`, `pack(pop, id)` -- which is
// honoured as a plain push or pop; a named pop past unrelated entries is
// rare enough not to have been met yet. The stack is the parser's, since
// what the pragma means is decided where it is read.
func (p *parser) pragmaPack() {
	at := p.cur
	p.next() // pack
	if p.peek() != token.LPAREN {
		return
	}
	p.next()
	var words []string
	for !p.atEOF() && p.peek() != token.RPAREN && !p.u.At(p.cur).StartsLine() {
		if p.peek() != token.COMMA {
			words = append(words, p.text(p.cur))
		}
		p.next()
	}
	if p.peek() == token.RPAREN {
		p.next()
	}

	value := func(w string) (int64, bool) {
		n, err := strconv.ParseInt(w, 0, 64)
		return n, err == nil && n > 0
	}
	switch {
	case len(words) == 0:
		p.pack = 0
	case words[0] == "push":
		p.packStack = append(p.packStack, p.pack)
		for _, w := range words[1:] {
			if n, ok := value(w); ok {
				p.pack = n
			}
		}
	case words[0] == "pop":
		if n := len(p.packStack); n > 0 {
			p.pack, p.packStack = p.packStack[n-1], p.packStack[:n-1]
		} else {
			p.pack = 0
		}
	case words[0] == "show":
	default:
		if n, ok := value(words[0]); ok {
			p.pack = n
		}
	}
	p.packs = append(p.packs, ast.PackAt{At: at, Pack: p.pack})
}

// consumeGreater takes one `>` off the cursor, splitting a `>>` in place.
//
// A template-argument-list that closes on `>>` has consumed only the first
// of the two; leaving the second is what makes `Tmpl<Tmpl<int>>` close two
// lists rather than one. Everything else about the cursor is unchanged, so
// the second `>` reports the position of the `>>` it came from -- which is
// where it was written.
func (p *parser) consumeGreater() ast.Tok {
	if !p.halfGtr && p.u.Kind(p.cur) == token.SHR {
		p.halfGtr = true
		return p.cur
	}
	return p.next()
}

func (p *parser) match(k token.Kind) bool {
	if p.peek() == k {
		p.next()
		return true
	}
	return false
}

func (p *parser) expect(k token.Kind) ast.Tok {
	if p.peek() == k {
		return p.next()
	}
	p.error(p.cur, fmt.Sprintf("expected %s, found %s", k, p.peek()))
	return ast.NoTok
}

func (p *parser) error(tok ast.Tok, msg string) {
	p.errorAt(tok, tok+1, msg)
}

func (p *parser) errorAt(lo, hi ast.Tok, msg string) {
	if lo == p.errTok && len(p.diags) > 0 {
		return
	}
	p.errTok = lo
	site := p.u.Span(lo, hi)
	p.diags = append(p.diags, Diagnostic{
		Severity: token.Error,
		Site:     site,
		Message:  msg,
	})
}

func (p *parser) advanceTo(syncs ...token.Kind) {
	p.resyncs++
	if p.mode&Tolerant == 0 && p.resyncs > p.maxResync {
		p.cur = ast.Tok(p.u.Len())
		return
	}
	for !p.atEOF() {
		k := p.peek()
		for _, s := range syncs {
			if k == s {
				return
			}
		}
		switch k {
		case token.LPAREN:
			p.skipBalanced(token.LPAREN, token.RPAREN)
		case token.LBRACK:
			p.skipBalanced(token.LBRACK, token.RBRACK)
		case token.LBRACE:
			p.skipBalanced(token.LBRACE, token.RBRACE)
		default:
			p.next()
		}
	}
}

func (p *parser) skipBalanced(open, close token.Kind) {
	if p.peek() == open {
		p.next()
	}
	depth := 1
	for !p.atEOF() && depth > 0 {
		switch p.peek() {
		case open:
			depth++
		case close:
			depth--
		}
		p.next()
	}
}

// Parse parses the token stream into an ast.File.
func Parse(u *Unit, mode Mode) (*ast.File, []Diagnostic) {
	p := newParser(u, mode)
	file := p.parseFile()
	SortDiagnostics(p.diags)
	return file, p.diags
}

// ParseFile runs phase 4 on f and parses the result.
func ParseFile(f *token.File, mode Mode) (*ast.File, []Diagnostic) {
	cfg := preprocessor.Config{
		Source: preprocessor.Mount{
			Name: f.Name(),
		},
	}
	if mode&ParseComments != 0 {
		cfg.KeepComments = true
	}
	pp := preprocessor.New(cfg)
	toks, ppDiags := pp.Run(f)
	unit := NewUnit(toks)
	file, parseDiags := Parse(unit, mode)

	var diags []Diagnostic
	for _, d := range ppDiags {
		diags = append(diags, Diagnostic{
			Severity: d.Severity,
			Site:     d.Site,
			Message:  d.Msg,
		})
	}
	diags = append(diags, parseDiags...)
	SortDiagnostics(diags)
	return file, diags
}

func (p *parser) isIdentText(tok ast.Tok, s string) bool {
	return p.u.Kind(tok) == token.IDENT && p.u.Text(tok) == s
}

func (p *parser) isModuleKeyword() bool {
	return p.isIdentText(p.cur, "module")
}

func (p *parser) isModuleKeywordAt(offset int) bool {
	return p.isIdentText(p.cur+ast.Tok(offset), "module")
}

func (p *parser) isImportKeyword() bool {
	return p.isIdentText(p.cur, "import")
}

func (p *parser) isImportKeywordAt(offset int) bool {
	return p.isIdentText(p.cur+ast.Tok(offset), "import")
}

func (p *parser) parseFile() *ast.File {
	start := p.pos()
	f := &ast.File{
		Unit: p.u,
	}

	// Retain comments if requested
	if p.mode&ParseComments != 0 {
		for i := 0; i < p.u.Len(); i++ {
			if p.u.Kind(ast.Tok(i)) == token.COMMENT {
				f.Comments = append(f.Comments, ast.Tok(i))
			}
		}
	}

	// Module declaration or global module fragment
	if p.isModuleKeyword() || (p.peek() == token.EXPORT && p.isModuleKeywordAt(1)) {
		decl := p.parseModuleDecl()
		if mod, ok := decl.(*ast.ModuleDecl); ok {
			f.Module = mod
		}
		f.Decls = append(f.Decls, decl)
	}

	for !p.atEOF() {
		prev := p.pos()
		d := p.parseDecl()
		if d != nil {
			f.Decls = append(f.Decls, d)
		}
		if p.pos() == prev {
			// No progress made; advance to prevent infinite loop
			p.advanceTo(token.SEMI, token.RBRACE)
			if p.peek() == token.SEMI {
				p.next()
			}
		}
	}

	f.Span = ast.Span{Lo: start, Hi: p.pos()}
	f.Packs = p.packs
	return f
}
