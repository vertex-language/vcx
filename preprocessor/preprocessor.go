// Package preprocessor implements translation phase 4: directive execution,
// macro replacement, and include resolution.
package preprocessor

import (
	"fmt"
	"strings"

	"github.com/vertex-language/vcx/scanner"
	"github.com/vertex-language/vcx/token"
)

// Diagnostic is phase 4's diagnostic across included files.
type Diagnostic struct {
	Severity token.Severity
	Site     Site
	Msg      string
	Name     string // the warning name printed in brackets; empty for errors
	Notes    []Note
}

// Note is a secondary position attached below a diagnostic: the definition
// site of a macro, the previous definition, the #if that was never closed.
type Note struct {
	Site Site
	Msg  string
}

// reader is the per-file state of the directive walk.
type reader struct {
	org   *Origin
	toks  []Token
	i     int
	conds []cond

	// Multiple-include optimization. miValid stays true only while nothing
	// has happened that could invalidate an include guard: no token outside a
	// conditional, and no directive other than an opening conditional or the
	// null directive. sawToken records the first text token, which closes the
	// window in which a guard may begin.
	miValid      bool
	sawToken     bool
	guardFound   string
	pendingGuard string // guard of the outermost #endif just closed

	// once records that this file asked not to be read again, with #pragma
	// once. It is the other half of the multiple-include optimization and is
	// not subject to miValid: the pragma states the conclusion outright
	// instead of leaving it to be inferred from the file's shape, so a token
	// before it takes nothing away.
	once bool
}

// Preprocessor holds one translation unit's phase-4 state.
type Preprocessor struct {
	cfg    Config
	macros *Table
	gen    *Gen
	deps   *Deps

	files map[string]*cached
	stack []*reader
	out   []Token
	diags []Diagnostic

	// depth counts nested expansions, against Config.MaxExpansionDepth.
	depth int

	// pushedMacros are #pragma push_macro's saved definitions, by name,
	// nil for a name that was not defined when it was pushed.
	pushedMacros map[string][]*Macro

	// counter is __COUNTER__'s next value. It counts over the whole
	// translation unit, never resetting per file, which is what makes the
	// name it builds unique.
	counter int

	modules []ModuleDirective

	lineDelta int    // #line's offset from the physical line
	fileName  string // #line's name, empty until one is set

	pasteCache map[string]pasteResult
	warned     map[string]bool
}

type pasteResult struct {
	kind token.Kind
	ok   bool
}

// New returns a preprocessor configured for one translation unit, with the
// predefined macros and the caller's -D and -U already applied.
func New(cfg Config) *Preprocessor {
	p := &Preprocessor{
		cfg:        cfg.Default(),
		macros:     NewTable(),
		gen:        NewGen(),
		files:      map[string]*cached{},
		pasteCache: map[string]pasteResult{},
		warned:     map[string]bool{},
	}
	if p.cfg.TrackDeps {
		p.deps = &Deps{}
	}
	p.installPredefines()
	return p
}

// Run preprocesses one primary source file and returns the token stream phase
// 5 reads, plus every diagnostic phases 1 through 4 produced.
//
// The result is a single sequence drawn from many files; each token carries
// the Origin its span belongs to, so nothing downstream has to guess.
func (p *Preprocessor) Run(f *token.File) ([]Token, []Diagnostic) {
	org := &Origin{File: f}
	if p.cfg.Source.FS != nil {
		// Give the primary file the same standing a header has: a mount and a
		// path within it, so a quoted #include resolves beside it.
		org.Mount = &p.cfg.Source
		org.Path = baseName(f.Name())
	}
	toks, diags := p.scanRaw(f)
	for i := range toks {
		toks[i].Origin = org
	}
	for _, d := range diags {
		p.fromScan(org, d)
	}
	if p.deps != nil {
		p.deps.Target = f.Name()
		p.deps.add(f.Name())
	}

	r := &reader{org: org, toks: toks, miValid: true}
	p.stack = append(p.stack, r)
	for _, inc := range p.cfg.PreIncludes {
		p.include(r, inc, false, Site{Origin: org, Pos: f.Pos(0), End: f.Pos(1)}, false)
	}
	p.run(r)
	p.stack = p.stack[:len(p.stack)-1]

	sortDiagnostics(p.diags)
	return p.out, p.diags
}

// run walks one file: directive lines are executed, text lines are expanded.
func (p *Preprocessor) run(r *reader) {
	for {
		t, ok := r.peek()
		if !ok {
			break
		}
		if t.Kind == token.HASH && t.StartsLine() {
			r.next()
			p.directive(r, t, r.takeDirectiveLine())
			continue
		}
		if r.skipping() {
			r.skipLine()
			continue
		}
		r.sawToken = true
		r.miValid = false
		// A module directive is recognized before expansion, because its
		// introducing tokens may not come from a macro and its name may not
		// be replaced by one.
		if p.moduleDirective(r) {
			continue
		}
		if p.pragmaOperator(r) {
			continue
		}
		if p.msPragmaOperator(r) {
			continue
		}
		p.expandText(r)
	}
	r.finishConds(p)

	// The file was fully read with a guard still standing: record it so the
	// next #include of this file can skip opening it.
	if r.miValid && r.guardFound == "" {
		r.guardFound = r.pendingGuard
	}
}

// pragmaOperator handles `_Pragma ( string-literal )`.
func (p *Preprocessor) pragmaOperator(r *reader) bool {
	if !isPragmaOperator(r, 0) {
		return false
	}

	at := r.peekAt(0).Site()
	text := destringize(r.peekAt(2).Text())
	r.next() // _Pragma
	r.next() // (
	r.next() // the string
	r.next() // )

	word := text
	if i := strings.IndexAny(word, " 	"); i >= 0 {
		word = word[:i]
	}
	switch word {
	case "once":
		r.once = true
		if at.Origin == nil || !at.Origin.System {
			if p.warn("pragma-once", at, "_Pragma(\"once\") is not ISO C++") {
				p.note(at, "vcx honours it; the portable spelling is an #ifndef "+
					"include guard, which vcx detects on its own")
			}
		}
	case "GCC", "clang":
		if p.warn("vendor-pragma", at, "_Pragma(%q) is a vendor extension and has no effect", text) {
			p.note(at, "warning state is a build decision, not a source one")
		}
	}
	return true
}

// isPragmaOperator reports whether the four tokens at r's cursor plus n
// spell `_Pragma ( string-literal )`. Anything short of the whole shape is
// an ordinary identifier -- reserved, but not this operator.
func isPragmaOperator(r *reader, n int) bool {
	t := r.peekAt(n)
	return t.Kind == token.IDENT && t.Text() == "_Pragma" &&
		r.peekAt(n+1).Kind == token.LPAREN &&
		r.peekAt(n+2).Kind == token.STRING_LIT &&
		r.peekAt(n+3).Kind == token.RPAREN
}

// destringize strips delimiters and unescapes quotes and backslashes in `_Pragma` strings.
func destringize(lit string) string {
	for _, prefix := range []string{"u8", "L", "u", "U"} {
		if strings.HasPrefix(lit, prefix) {
			lit = lit[len(prefix):]
			break
		}
	}
	lit = strings.TrimPrefix(lit, `"`)
	lit = strings.TrimSuffix(lit, `"`)

	const backslash = 0x5c // a backslash, by code point, so this line needs no escape
	var b strings.Builder
	for i := 0; i < len(lit); i++ {
		if lit[i] == backslash && i+1 < len(lit) && (lit[i+1] == '"' || lit[i+1] == backslash) {
			i++
		}
		b.WriteByte(lit[i])
	}
	return b.String()
}

// expandText expands tokens from the current position to the next directive.
func (p *Preprocessor) expandText(r *reader) {
	s := &stream{more: func() (Token, bool) {
		t, ok := r.peek()
		if !ok || (t.Kind == token.HASH && t.StartsLine()) {
			return Token{}, false
		}
		// A `_Pragma(...)` ends this run of text for the same reason a
		// line-opening `#` does: it is a directive wearing an operator's
		// clothes, and the loop above is where directives are executed.
		// Without this it is only recognized when it happens to be the
		// first text token in the file.
		if isPragmaOperator(r, 0) || isMSPragmaOperator(r, 0) {
			return Token{}, false
		}
		r.next()
		return t, true
	}}
	before := len(p.out)
	p.out = p.expandInto(s, p.out)
	// Anything the expander read but did not consume goes back.
	r.unread(s.buf[s.i:])

	// A pragma operator that a macro's replacement list produced -- which
	// is where Microsoft's headers write every one of theirs -- came out
	// of the expander as ordinary tokens. It is a directive all the same,
	// and is taken out of the output and executed here.
	p.out = p.pragmasInOutput(r, p.out, before)
}

// isMSPragmaOperator reports whether the tokens at r's cursor plus n open
// `__pragma (`.
func isMSPragmaOperator(r *reader, n int) bool {
	t := r.peekAt(n)
	return t.Kind == token.IDENT && t.Text() == "__pragma" && r.peekAt(n+1).Kind == token.LPAREN
}

// pragmasInOutput finds `__pragma ( ... )` and `_Pragma ( "..." )` among
// tokens the expander produced, from index from on, executes each, and
// returns the output with them removed.
func (p *Preprocessor) pragmasInOutput(r *reader, out []Token, from int) []Token {
	for i := from; i < len(out); i++ {
		t := out[i]
		if t.Kind != token.IDENT || i+1 >= len(out) || out[i+1].Kind != token.LPAREN {
			continue
		}
		switch t.Text() {
		case "__pragma":
			depth := 0
			end := -1
			for j := i + 1; j < len(out); j++ {
				if out[j].Kind == token.LPAREN {
					depth++
				} else if out[j].Kind == token.RPAREN {
					depth--
					if depth == 0 {
						end = j
						break
					}
				}
			}
			if end < 0 {
				p.errorf(t.Site(), "unterminated __pragma")
				return out
			}
			line := append([]Token(nil), out[i+2:end]...)
			rest := append([]Token(nil), out[end+1:]...)
			if len(rest) > 0 {
				rest[0].Flags |= token.FlagNLBefore // see msPragmaOperator
			}
			out = append(out[:i], rest...)
			// doPragma appends a `#pragma` line to p.out for whatever it
			// does not handle itself, so the output is rebuilt around it.
			saved := p.out
			p.out = out[:i:i]
			p.doPragma(r, line, t.Site())
			inserted := len(p.out) - i
			out = append(p.out, out[i:]...)
			p.out = saved
			i += inserted - 1
		case "_Pragma":
			if i+3 < len(out) && out[i+2].Kind == token.STRING_LIT && out[i+3].Kind == token.RPAREN {
				text := destringize(out[i+2].Text())
				rest := append([]Token(nil), out[i+4:]...)
				out = append(out[:i], rest...)
				word := text
				if k := strings.IndexAny(word, " 	"); k >= 0 {
					word = word[:k]
				}
				if word == "once" {
					r.once = true
				}
				i--
			}
		}
	}
	return out
}

func (r *reader) peek() (Token, bool) {
	for r.i < len(r.toks) && r.toks[r.i].Kind == token.EOF {
		r.i++
	}
	if r.i >= len(r.toks) {
		return Token{}, false
	}
	return r.toks[r.i], true
}

// peekAt looks n tokens past the cursor, skipping the EOF markers that sit
// between concatenated files the way peek does. An index past the end yields
// the zero Token, whose Kind is EOF and matches nothing a caller tests for.
func (r *reader) peekAt(n int) Token {
	j := r.i
	for {
		for j < len(r.toks) && r.toks[j].Kind == token.EOF {
			j++
		}
		if j >= len(r.toks) {
			return Token{}
		}
		if n == 0 {
			return r.toks[j]
		}
		n--
		j++
	}
}

func (r *reader) next() (Token, bool) {
	t, ok := r.peek()
	if ok {
		r.i++
	}
	return t, ok
}

func (r *reader) unread(ts []Token) {
	if len(ts) == 0 {
		return
	}
	rest := append(append([]Token(nil), ts...), r.toks[r.i:]...)
	r.toks, r.i = rest, 0
}

// takeDirectiveLine returns the remaining tokens on the directive line.
func (r *reader) takeDirectiveLine() []Token {
	start := r.i
	for r.i < len(r.toks) && r.toks[r.i].Kind != token.EOF && !r.toks[r.i].StartsLine() {
		r.i++
	}
	return r.toks[start:r.i]
}

// skipLine discards one line of a skipped group. Unlike takeDirectiveLine it
// starts *on* a line-opening token, so it consumes one unconditionally: a
// version that could return nothing would not advance, and the walk would
// not terminate.
func (r *reader) skipLine() {
	r.i++
	for r.i < len(r.toks) && r.toks[r.i].Kind != token.EOF && !r.toks[r.i].StartsLine() {
		r.i++
	}
}

// Macros returns the macro table. It is live: a caller that defines into it
// directly gets the same result -D would have given, which is what the target
// model's contribution needs.
func (p *Preprocessor) Macros() *Table { return p.macros }

// Gen returns the arena holding tokens that # and ## built.
func (p *Preprocessor) Gen() *Gen { return p.gen }

// Deps returns the recorded dependency set, or nil when TrackDeps was false.
func (p *Preprocessor) Deps() *Deps { return p.deps }

// Diagnostics returns everything phases 1 through 4 have reported so far.
func (p *Preprocessor) Diagnostics() []Diagnostic { return p.diags }

// Scan runs phases 1 through 3 over a file and returns its preprocessing tokens.
func (p *Preprocessor) Scan(f *token.File) []Token {
	org := &Origin{File: f}
	toks, diags := p.scanRaw(f)
	for i := range toks {
		toks[i].Origin = org
	}
	for _, d := range diags {
		p.fromScan(org, d)
	}
	return toks
}

// scanRaw runs the scanner and lifts its output into phase 4's token type,
// with no Origin attached — the caller supplies one, because a cached file's
// tokens are shared across inclusions that have different ones.
//
// ScanPP differs from the ordinary mode because a pp-token is not yet a C++
// token: malformed-literal diagnostics are deferred to phase 7 (an excluded
// #if group may legally hold what phase 7 would reject), the bracket stack is
// off, and a line-opening '#' is returned as HASH rather than swallowed.
func (p *Preprocessor) scanRaw(f *token.File) ([]Token, []token.Diagnostic) {
	mode := scanner.ScanPP
	if p.cfg.KeepComments {
		mode |= scanner.ScanComments
	}
	toks, diags := scanner.Scan(f, p.cfg.Std, mode)
	out := make([]Token, 0, len(toks))
	for _, t := range toks {
		if t.Kind == token.COMMENT && !p.cfg.KeepComments {
			continue
		}
		out = append(out, Token{Kind: t.Kind, Flags: t.Flags, Pos: t.Pos, End: t.End})
	}
	return out, diags
}

// rescan reports whether a spelling forms a single valid preprocessing token.
func (p *Preprocessor) rescan(spelling string) (token.Kind, bool) {
	if r, ok := p.pasteCache[spelling]; ok {
		return r.kind, r.ok
	}
	f := token.NewFile("<paste>", []byte(spelling+"\n"))
	toks, diags := scanner.Scan(f, p.cfg.Std, scanner.ScanPP)
	var res pasteResult
	if len(diags) == 0 && len(toks) == 2 && toks[1].Kind == token.EOF {
		res.kind, res.ok = toks[0].Kind, true
	}
	p.pasteCache[spelling] = res
	return res.kind, res.ok
}

// errorf reports an error at a site.
func (p *Preprocessor) errorf(s Site, format string, a ...any) {
	p.diags = append(p.diags, Diagnostic{
		Severity: token.Error,
		Site:     s,
		Msg:      fmt.Sprintf(format, a...),
	})
}

// warn reports a warning and reports whether it was emitted. A warning sited
// in a system header is the header's mistake, not each #include of it: report
// once per header, per name. Callers attaching a note must check the return —
// note() binds to the last diagnostic appended, whatever that is, so a note
// after a suppressed warning would attach to something unrelated.
func (p *Preprocessor) warn(name string, s Site, format string, a ...any) bool {
	if s.Origin != nil && s.Origin.System {
		key := s.Origin.Name() + "\x00" + name
		if p.warned[key] {
			return false
		}
		p.warned[key] = true
	}
	p.diags = append(p.diags, Diagnostic{
		Severity: token.Warn,
		Site:     s,
		Name:     name,
		Msg:      fmt.Sprintf(format, a...),
	})
	return true
}

// note attaches to the diagnostic just reported.
func (p *Preprocessor) note(s Site, format string, a ...any) {
	if len(p.diags) == 0 {
		return
	}
	d := &p.diags[len(p.diags)-1]
	d.Notes = append(d.Notes, Note{Site: s, Msg: fmt.Sprintf(format, a...)})
}

// fromScan lifts a phases 1-3 diagnostic into phase 4's position space under
// the given Origin, preserving severity. Warnings route through warn() under
// the name "scan", so one in a system header reports once per header, the
// same treatment every phase-4 warning gets.
func (p *Preprocessor) fromScan(org *Origin, d token.Diagnostic) {
	site := Site{Origin: org, Pos: d.Pos, End: d.End}
	if d.Severity == token.Warn {
		p.warn("scan", site, "%s", d.Message)
		return
	}
	p.diags = append(p.diags, Diagnostic{
		Severity: d.Severity,
		Site:     site,
		Msg:      d.Message,
	})
}

// sortDiagnostics orders by file, then position, then extent, stably — the
// same contract token.SortDiagnostics holds within one file, extended across
// the include graph so a run is byte-identical every time.
func sortDiagnostics(ds []Diagnostic) {
	// Insertion sort: diagnostic counts are small, and stability is required.
	for i := 1; i < len(ds); i++ {
		for j := i; j > 0 && lessDiag(ds[j], ds[j-1]); j-- {
			ds[j], ds[j-1] = ds[j-1], ds[j]
		}
	}
}

func lessDiag(a, b Diagnostic) bool {
	an, bn := a.Site.Origin.Name(), b.Site.Origin.Name()
	if an != bn {
		return an < bn
	}
	if a.Site.Pos != b.Site.Pos {
		return a.Site.Pos < b.Site.Pos
	}
	return a.Site.End < b.Site.End
}

// baseName is path.Base for a slash-separated name, without importing path
// for one call in the one place a file name is not already relative.
func baseName(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' || s[i] == '\\' {
			return s[i+1:]
		}
	}
	return s
}

// String renders the diagnostic as file:line:col: severity: message.
func (d Diagnostic) String() string {
	name := ""
	if d.Name != "" {
		name = " [" + d.Name + "]"
	}
	return fmt.Sprintf("%s: %s: %s%s", d.Site, d.Severity, d.Msg, name)
}

// String renders a site as file:line:col, or names the arena of a token that
// no file produced.
func (s Site) String() string {
	switch {
	case s.Origin == nil:
		return "<unknown>"
	case s.Origin.File == nil:
		return s.Origin.Name()
	default:
		p := s.Origin.File.Position(s.Pos)
		return fmt.Sprintf("%s:%d:%d", s.Origin.Name(), p.Line, p.Column)
	}
}

// msPragmaOperator handles Microsoft's `__pragma ( pp-tokens )`.
func (p *Preprocessor) msPragmaOperator(r *reader) bool {
	t := r.peekAt(0)
	if t.Kind != token.IDENT || t.Text() != "__pragma" || r.peekAt(1).Kind != token.LPAREN {
		return false
	}
	at := t.Site()
	r.next() // __pragma
	r.next() // (
	var line []Token
	depth := 1
	for {
		tok, ok := r.next()
		if !ok {
			p.errorf(at, "unterminated __pragma")
			return true
		}
		if tok.Kind == token.LPAREN {
			depth++
		} else if tok.Kind == token.RPAREN {
			depth--
			if depth == 0 {
				break
			}
		}
		line = append(line, tok)
	}
	p.doPragma(r, line, at)
	// The pragma became a line of its own in the output, and what follows
	// it in the source -- on the same line, as `__pragma(...) extern "C"
	// {` has it -- has to open the next one, or the parser skipping the
	// pragma line skips the declaration too.
	if r.i < len(r.toks) {
		r.toks[r.i].Flags |= token.FlagNLBefore
	}
	return true
}
