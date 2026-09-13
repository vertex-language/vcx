package preprocessor

import (
	"io/fs"
	"path"
	"strings"

	"github.com/vertex-language/vcx/token"
)

// Deps accumulates the dependency set: the target, and every file #include
// reached, in first-seen order.
//
// It is returned as a value rather than written as a .d file. vmake imports
// this package and reads the slice; nothing has to parse Makefile syntax to
// learn what a translation unit depends on.
type Deps struct {
	Target string
	Files  []string
	seen   map[string]bool
}

func (d *Deps) add(name string) {
	if d == nil {
		return
	}
	if d.seen == nil {
		d.seen = map[string]bool{}
	}
	if !d.seen[name] {
		d.seen[name] = true
		d.Files = append(d.Files, name)
	}
}

// cached is one entry of the open-once cache. Content is read at most once
// per translation unit; guard is the controlling macro discovered when the
// file was fully read, and is what lets the second #include skip the file
// entirely.
//
// diags holds the phases 1-3 diagnostics scanning produced, deferred: they
// are reported on the first read, through the real Origin, so they carry the
// inclusion chain and the System treatment — open() has neither.
type cached struct {
	file  *token.File
	toks  []Token
	diags []token.Diagnostic
	guard string
	done  bool

	// once is set when the file said #pragma once while it was read. Unlike
	// guard it needs no macro to still be defined to hold: the file asked not
	// to be read again, and nothing the program does afterwards withdraws the
	// request.
	once bool
}

// doInclude implements [cpp.include].
func (p *Preprocessor) doInclude(r *reader, line []Token, at Site) {
	name, angled, n, ok := p.headerName(line, at)
	if !ok {
		return
	}
	p.expectEnd(line[n:], "#include")
	if len(p.stack) >= p.cfg.MaxIncludeDepth {
		p.errorf(at, "#include nested too deeply (limit %d)", p.cfg.MaxIncludeDepth)
		p.note(at, "a header that includes itself needs an include guard")
		return
	}
	p.include(r, name, angled, at, false)
}

// doIncludeNext is GNU's #include_next: the same search, resumed after the
// directory this file was found in.
//
// It exists because a header may want to wrap the one it shadows — libstdc++'s
// <limits.h> includes the compiler's, and a build that puts its own <stdio.h>
// ahead of the platform's still wants the platform's underneath. There is no
// ISO spelling for that, and a header that uses it cannot be rewritten by the
// person compiling it.
func (p *Preprocessor) doIncludeNext(r *reader, line []Token, at Site) {
	name, angled, n, ok := p.headerName(line, at)
	if !ok {
		return
	}
	p.expectEnd(line[n:], "#include_next")
	if len(p.stack) >= p.cfg.MaxIncludeDepth {
		p.errorf(at, "#include_next nested too deeply (limit %d)", p.cfg.MaxIncludeDepth)
		return
	}
	p.include(r, name, angled, at, true)
}

// headerName recovers the header-name pp-token, which phase 3 cannot produce:
// <vector> scans as LSS IDENT GTR everywhere except here, and only this call
// site knows the context. It returns how many tokens it consumed.
//
// The quoted form is one STRING_LIT and needs no reconstruction — but it must
// be a plain one: L"x" and u8"x" are string literals with encoding prefixes
// and are not header names, and a raw string is not one either.
//
// The angled form is rebuilt from the raw bytes between '<' and '>', not from
// the token spellings, because the characters between them are not tokens:
// <sys/socket.h> must survive with its slash and dots intact, and
// <experimental/__config> must keep an identifier that starts with two
// underscores from being anything but text.
func (p *Preprocessor) headerName(line []Token, at Site) (name string, angled bool, n int, ok bool) {
	if len(line) == 0 {
		p.errorf(at, `expected "FILENAME" or <FILENAME>`)
		return "", false, 0, false
	}
	switch {
	case line[0].Kind == token.STRING_LIT && plainString(line[0]):
		return strings.Trim(line[0].Text(), `"`), false, 1, true

	case line[0].Kind == token.LSS:
		for i := 1; i < len(line); i++ {
			if line[i].Kind != token.GTR {
				continue
			}
			org := line[0].Origin
			if org == nil || org.File == nil {
				break
			}
			return string(org.File.Slice(line[0].End, line[i].Pos)), true, i + 1, true
		}
		p.errorf(at, "missing '>' in header name")
		return "", false, 0, false
	}

	// Neither form: the line is a macro that expands to one ([cpp.include]/4).
	expanded := p.expandClosed(line)
	if len(expanded) > 0 && !sameTokens(expanded, line) {
		nm, ang, _, ok := p.headerName(expanded, at)
		return nm, ang, len(line), ok
	}
	p.errorf(at, `expected "FILENAME" or <FILENAME>`)
	return "", false, 0, false
}

// plainString reports whether a string literal has no encoding prefix and is
// not raw — the only spelling that is also a header name.
func plainString(t Token) bool {
	s := t.Text()
	return len(s) > 0 && s[0] == '"' && !t.Flags.Has(token.FlagRaw)
}

func sameTokens(a, b []Token) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Kind != b[i].Kind || a[i].Text() != b[i].Text() {
			return false
		}
	}
	return true
}

// evalHasInclude implements __has_include ([cpp.cond]/6). It answers whether
// the same search #include would run finds a file, and reads nothing: a
// header that exists but does not compile still answers 1, which is what the
// operator asks.
func (p *Preprocessor) evalHasInclude(line []Token, i int) (n, end int, ok bool) {
	t := line[i]
	j := i + 1
	if j >= len(line) || line[j].Kind != token.LPAREN {
		p.errorf(t.Site(), "__has_include requires a parenthesized header name")
		return 0, 0, false
	}
	name, angled, consumed, ok := p.headerName(line[j+1:], t.Site())
	if !ok {
		return 0, 0, false
	}
	j += 1 + consumed
	if j >= len(line) || line[j].Kind != token.RPAREN {
		p.errorf(t.Site(), "missing ')' after __has_include")
		return 0, 0, false
	}
	if p.probe(name, angled) {
		return 1, j, true
	}
	return 0, j, true
}

// searchList returns the mounts and relative paths a header name resolves
// against, in order. It is the one place the search order is written down:
// a quoted include looks in the including file's own directory first and then
// walks the list; an angled include skips that first step; #include_next
// resumes after the directory the current file came from. There is no
// -iquote/-isystem/-idirafter tower on top of it.
func (p *Preprocessor) searchList(org *Origin, name string, angled, next bool) (mounts []*Mount, rels []string) {
	start := 0
	if next {
		for i := range p.cfg.Search {
			if org != nil && org.Mount == &p.cfg.Search[i] {
				start = i + 1
				break
			}
		}
	}
	if !angled && !next && org != nil && org.Mount != nil {
		mounts = append(mounts, org.Mount)
		rels = append(rels, path.Join(path.Dir(org.Path), name))
	}
	for i := start; i < len(p.cfg.Search); i++ {
		mounts = append(mounts, &p.cfg.Search[i])
		rels = append(rels, name)
	}
	return mounts, rels
}

// probe reports whether a header name resolves, without reading it.
func (p *Preprocessor) probe(name string, angled bool) bool {
	return p.probeFrom(name, angled, false)
}

// probeFrom is probe with #include_next's search when next is set.
func (p *Preprocessor) probeFrom(name string, angled, next bool) bool {
	if path.IsAbs(name) {
		return false
	}
	var org *Origin
	if len(p.stack) > 0 {
		org = p.stack[len(p.stack)-1].org
	}
	mounts, rels := p.searchList(org, name, angled, next)
	for i, m := range mounts {
		rel := path.Clean(rels[i])
		if strings.HasPrefix(rel, "..") {
			continue
		}
		if _, err := fs.Stat(m.FS, rel); err == nil {
			return true
		}
	}
	return false
}

// include resolves and reads a header.
func (p *Preprocessor) include(r *reader, name string, angled bool, at Site, next bool) {
	if path.IsAbs(name) {
		p.errorf(at, "absolute path in #include: %q", name)
		p.note(at, "use -I and a relative path so the build is reproducible")
		return
	}

	mounts, rels := p.searchList(r.org, name, angled, next)
	for i, m := range mounts {
		rel := path.Clean(rels[i])
		if strings.HasPrefix(rel, "..") {
			continue
		}
		display := path.Join(m.Name, rel)
		c, err := p.open(m, rel, display)
		if err != nil {
			continue
		}
		p.deps.add(display)

		// The multiple-include optimization: a file whose entire contents sit
		// inside #ifndef GUARD … #endif, with nothing outside, need not be
		// opened again while GUARD is defined. A file that said #pragma once
		// need not be opened again at all — it asked, and there is no macro
		// standing behind the request that the program could undefine.
		if c.done && (c.once || (c.guard != "" && p.macros.Defined(c.guard))) {
			return
		}
		p.readFile(c, m, rel, display, at, r.org)
		return
	}

	if next {
		// Nothing further down the list has it, which is what a wrapper
		// header at the bottom of the list will find. Saying nothing is
		// wrong; naming it as not found is right.
		p.errorf(at, "%q file not found after this directory", name)
		return
	}
	p.errorf(at, "%q file not found", name)
	if len(p.cfg.Search) == 0 {
		p.note(at, "no include directories are configured; use -I")
	} else {
		p.note(at, "searched %d directories; run `v++ env` to see the resolved list",
			len(p.cfg.Search))
	}
}

// open reads a file at most once per translation unit. Scanning diagnostics
// are stashed, not reported: they wait for readFile, where an Origin with the
// inclusion chain exists.
func (p *Preprocessor) open(m *Mount, rel, display string) (*cached, error) {
	if c, ok := p.files[display]; ok {
		return c, nil
	}
	src, err := fs.ReadFile(m.FS, rel)
	if err != nil {
		return nil, err
	}
	f := token.NewFile(display, src)
	toks, diags := p.scanRaw(f)
	c := &cached{file: f, diags: diags, toks: toks}
	p.files[display] = c
	return c, nil
}

func (p *Preprocessor) readFile(c *cached, m *Mount, rel, display string, at Site, parent *Origin) {
	org := &Origin{
		File:       c.file,
		Mount:      m,
		Path:       rel,
		Parent:     parent,
		IncludePos: at.Pos,
		System:     m.System,
		Guard:      c.guard,
	}
	toks := make([]Token, len(c.toks))
	copy(toks, c.toks)
	for i := range toks {
		toks[i].Origin = org
	}
	r := &reader{org: org, toks: toks, miValid: true}

	// Phases 1-3 diagnostics, deferred from open(): reported here, on the
	// first read only, through the real Origin — so they carry the include
	// chain and the System treatment. A scanner mistake in a header is the
	// header's, once, like any other diagnostic.
	if !c.done {
		for _, d := range c.diags {
			p.fromScan(org, d)
		}
	}

	p.stack = append(p.stack, r)
	p.run(r)
	p.stack = p.stack[:len(p.stack)-1]

	// Record what we learned, so the next #include of this file can skip it.
	if !c.done {
		c.done = true
		c.guard = r.guardFound
		c.once = r.once
	}
}
