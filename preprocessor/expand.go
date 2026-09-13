package preprocessor

import "github.com/vertex-language/vcx/token"

// HideSet is the blue paint of Prosser's algorithm: the set of macros that
// must not be expanded in a token, carried by the token itself rather than by
// a stack. Sets are immutable and shared; they are almost always empty or
// tiny.
type HideSet struct{ ids []int }

func (h *HideSet) Has(id int) bool {
	if h == nil {
		return false
	}
	for _, x := range h.ids {
		if x == id {
			return true
		}
		if x > id {
			break
		}
	}
	return false
}

func (h *HideSet) Add(id int) *HideSet {
	if h.Has(id) {
		return h
	}
	out := make([]int, 0, len(h.idsOf())+1)
	inserted := false
	for _, x := range h.idsOf() {
		if !inserted && x > id {
			out, inserted = append(out, id), true
		}
		out = append(out, x)
	}
	if !inserted {
		out = append(out, id)
	}
	return &HideSet{ids: out}
}

// idsOf is a nil-safe accessor so Add and the set operations need no nil
// checks.
func (h *HideSet) idsOf() []int {
	if h == nil {
		return nil
	}
	return h.ids
}

func hsUnion(a, b *HideSet) *HideSet {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	out := a
	for _, x := range b.ids {
		out = out.Add(x)
	}
	return out
}

// hsInter is the operation the standard's prose loses. For a function-like
// macro the new hide set is (HS ∩ HS') ∪ {T}, where HS' belongs to the
// *closing parenthesis* — not the macro name. Anything else gets the
// classic recursive cases wrong.
func hsInter(a, b *HideSet) *HideSet {
	if a == nil || b == nil {
		return nil
	}
	var out []int
	for _, x := range a.ids {
		if b.Has(x) {
			out = append(out, x)
		}
	}
	if out == nil {
		return nil
	}
	return &HideSet{ids: out}
}

// hsadd unions hs into every token's hide set.
func hsadd(hs *HideSet, ts []Token) []Token {
	if hs == nil {
		return ts
	}
	for i := range ts {
		ts[i].Hide = hsUnion(hs, ts[i].Hide)
	}
	return ts
}

// stream is a token sequence with pushback. more supplies tokens from a file
// and is nil for a closed sequence (a macro argument), which is what makes
// "pre-expansion of an argument cannot take tokens from after the invocation"
// fall out of the structure instead of needing a rule.
type stream struct {
	buf  []Token
	i    int
	more func() (Token, bool)
}

func (s *stream) fill(n int) bool {
	for len(s.buf)-s.i <= n {
		if s.more == nil {
			return false
		}
		t, ok := s.more()
		if !ok {
			s.more = nil
			return false
		}
		s.buf = append(s.buf, t)
	}
	return true
}

func (s *stream) peek(n int) (Token, bool) {
	if !s.fill(n) {
		return Token{}, false
	}
	return s.buf[s.i+n], true
}

func (s *stream) next() (Token, bool) {
	t, ok := s.peek(0)
	if ok {
		s.i++
	}
	return t, ok
}

func (s *stream) push(ts []Token) {
	if len(ts) == 0 {
		return
	}
	rest := s.buf[s.i:]
	s.buf = append(append(make([]Token, 0, len(ts)+len(rest)), ts...), rest...)
	s.i = 0
}

// Expand macro-replaces a finite token sequence and returns the result. It is
// phase 4's replacement step on its own: the directive walk calls it on the
// text lines it lets through, and the #if evaluator calls it on the
// controlling expression.
func (p *Preprocessor) Expand(ts []Token) []Token { return p.expandClosed(ts) }

// expandInto is the expansion driver, written as a loop rather than as
// recursion on a whole sequence, so the top-level stream can keep reading
// from a file.
func (p *Preprocessor) expandInto(s *stream, out []Token) []Token {
	for {
		t, ok := s.peek(0)
		if !ok {
			return out
		}
		if t.Kind == token.IDENT && p.expandOne(s) {
			continue
		}
		s.next()
		out = append(out, t)
	}
}

// expandClosed expands a finite sequence: macro arguments, and #if lines.
func (p *Preprocessor) expandClosed(ts []Token) []Token {
	if len(ts) == 0 {
		return nil
	}
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > p.cfg.MaxExpansionDepth {
		p.errorf(ts[0].Site(), "macro expansion nested too deeply")
		return ts
	}
	s := &stream{buf: append([]Token(nil), ts...)}
	return p.expandInto(s, nil)
}

// expandOne handles the head of s if it invokes a macro, and reports whether
// it did. The order of tests matters: the hide-set check comes before the
// search for '(', because finding the paren can pop a context and re-enable
// the very macro we are inside.
func (p *Preprocessor) expandOne(s *stream) bool {
	t, _ := s.peek(0)
	m := p.macros.Lookup(t.Text())
	if m == nil || t.Hide.Has(m.ID) {
		return false
	}
	m.Used = true

	if m.Builtin != NotBuiltin {
		s.next()
		p.pushRep(s, p.builtin(m, t), t)
		return true
	}

	if m.ObjLike {
		s.next()
		use := t.Site()
		rep := p.subst(m, m.Body, nil, t.Hide.Add(m.ID), use, t.Exp)
		p.pushRep(s, rep, t)
		return true
	}

	// A function-like macro expands only when the next token is '('.
	lp, ok := s.peek(1)
	if !ok || lp.Kind != token.LPAREN {
		return false
	}
	s.next() // name
	s.next() // '('
	args, rp, ok := p.arguments(s, m, t)
	if !ok {
		return false
	}
	use := t.Site()
	hs := hsInter(t.Hide, rp.Hide).Add(m.ID)
	rep := p.subst(m, m.Body, args, hs, use, t.Exp)
	p.pushRep(s, rep, t)
	return true
}

// pushRep pushes a replacement list back for rescanning. The first token of
// an expansion inherits the invocation's spacing, which is what keeps `x=FOO`
// from printing as `x= bar` and `x = FOO` from printing as `x =bar`.
func (p *Preprocessor) pushRep(s *stream, rep []Token, at Token) {
	if len(rep) > 0 {
		rep[0].Flags &^= token.FlagAdjacent
		rep[0].Flags |= at.Flags & token.FlagAdjacent
		rep[0].Flags |= at.Flags & token.FlagNLBefore
	} else if at.Flags.Has(token.FlagNLBefore) {
		if _, ok := s.peek(0); ok {
			s.buf[s.i].Flags |= token.FlagNLBefore
		}
	}
	s.push(rep)
}

// arguments collects a function-like macro's actual arguments, splitting on
// commas that are not inside parentheses.
//
// Only parentheses protect a comma. A template argument list does not:
// M(std::pair<int, int>) passes two arguments, because phase 4 has no idea
// what a template is and the angle brackets are ordinary punctuators. That is
// the standard's answer and the reason every C++ codebase eventually writes
// an extra pair of parentheses.
//
// A variadic macro's trailing slot swallows the remaining commas. An
// invocation that reaches the end of the file, or a line-opening '#', is
// unterminated: the reader refuses to cross a directive, so a macro
// invocation can never straddle one.
func (p *Preprocessor) arguments(s *stream, m *Macro, name Token) ([][]Token, Token, bool) {
	args := make([][]Token, 0, m.Arity())
	cur := []Token{}
	depth := 0
	slot := 0
	for {
		t, ok := s.next()
		if !ok {
			p.errorf(name.Site(), "unterminated argument list for macro %q", m.Name)
			return nil, Token{}, false
		}
		switch {
		case t.Kind == token.LPAREN:
			depth++
		case t.Kind == token.RPAREN:
			if depth == 0 {
				args = append(args, cur)
				return p.checkArity(args, m, name, t)
			}
			depth--
		case t.Kind == token.COMMA && depth == 0:
			// Once we are in the variadic slot, commas are ordinary tokens.
			if !(m.Variadic && slot >= len(m.Params)) {
				args = append(args, cur)
				cur = []Token{}
				slot++
				continue
			}
		}
		cur = append(cur, t)
	}
}

func (p *Preprocessor) checkArity(args [][]Token, m *Macro, name, rp Token) ([][]Token, Token, bool) {
	// `M()` with M taking one parameter passes one empty argument, not zero.
	if len(args) == 1 && len(args[0]) == 0 && m.Arity() == 0 {
		args = nil
	}
	switch {
	case len(args) < m.Arity() && m.Variadic && len(args) == m.Arity()-1:
		// A variadic macro may be called with nothing in the variadic slot.
		args = append(args, nil)
	case len(args) < m.Arity():
		p.errorf(name.Site(), "macro %q requires %d arguments, but %d given",
			m.Name, m.Arity(), len(args))
		return nil, rp, false
	case len(args) > m.Arity():
		p.errorf(name.Site(), "macro %q passed %d arguments, but takes %d",
			m.Name, len(args), m.Arity())
		return nil, rp, false
	}
	return args, rp, true
}

// subst walks a replacement list case for case: stringize, paste-with-
// parameter on either side, plain parameter (fully expanded first), and
// everything else copied through. hsadd is applied once at the end.
func (p *Preprocessor) subst(m *Macro, is []Token, ap [][]Token, hs *HideSet, use Site, outer *Expansion) []Token {
	is = p.vaOptRewrite(m, is, ap)

	var os []Token
	arg := func(t Token) (int, bool) {
		if t.Kind != token.IDENT {
			return 0, false
		}
		i := m.Param(t.Text())
		return i, i >= 0
	}
	body := func(t Token) Token {
		t.Exp = &Expansion{
			Macro: m.Name,
			Use:   use,
			Def:   Site{Origin: t.Origin, Pos: t.Pos, End: t.End},
			Outer: outer,
		}
		return t
	}

	for i := 0; i < len(is); i++ {
		t := is[i]

		// # parameter
		if t.Kind == token.HASH && i+1 < len(is) {
			if n, ok := arg(is[i+1]); ok {
				st := p.gen.Stringize(ap[n])
				st.Flags = t.Flags & token.FlagAdjacent
				os = append(os, body(st))
				i++
				continue
			}
		}

		// , ## __VA_ARGS__ — GNU's comma swallow.
		//
		// The ## here does not paste. It marks the comma as belonging to the
		// variadic argument: if that argument is empty the comma goes with
		// it, and otherwise both stay and the argument is substituted the
		// ordinary way. __VA_OPT__ is the standard answer and vcx implements
		// it, but this spelling is in a great deal of C reached from C++ —
		// glibc's headers and the Windows SDK both — and refusing it means
		// refusing those headers.
		if t.Kind == token.HASHHASH && i+1 < len(is) && len(os) > 0 &&
			os[len(os)-1].Kind == token.COMMA && m.Variadic {
			if n, ok := arg(is[i+1]); ok && n == len(m.Params) {
				if len(ap[n]) == 0 {
					os = os[:len(os)-1]
				} else {
					os = append(os, spaceLike(is[i+1], p.expandClosed(ap[n]))...)
				}
				i++
				continue
			}
		}

		// ## parameter, and ## anything
		if t.Kind == token.HASHHASH && i+1 < len(is) {
			nxt := is[i+1]
			if n, ok := arg(nxt); ok {
				if len(ap[n]) > 0 {
					os = p.glue(os, ap[n], body(t))
				}
				i++
				continue
			}
			os = p.glue(os, []Token{body(nxt)}, body(t))
			i++
			continue
		}

		// parameter ## — the argument goes in unexpanded, and the ## is left
		// for the next iteration to consume.
		if n, ok := arg(t); ok && i+1 < len(is) && is[i+1].Kind == token.HASHHASH {
			if len(ap[n]) == 0 {
				i++ // drop the parameter and the ##: the placemarker rule
				continue
			}
			os = append(os, spaceLike(t, ap[n])...)
			continue
		}

		// plain parameter: fully expanded before substitution
		if n, ok := arg(t); ok {
			os = append(os, spaceLike(t, p.expandClosed(ap[n]))...)
			continue
		}

		os = append(os, body(t))
	}
	return hsadd(hs, os)
}

// vaOptRewrite resolves every __VA_OPT__(content) group in a replacement list
// before substitution runs, so that everything downstream sees an ordinary
// list ([cpp.replace]/8-12).
//
// When the variadic argument has at least one token the group is replaced by
// its contents, which the main loop then substitutes like any other stretch
// of replacement list — that is what makes __VA_ARGS__ and # and ## inside a
// group work without a second implementation of each. When it is empty the
// group is a placemarker: it produces nothing, and a ## on either side of it
// has nothing left to paste and goes too.
//
// #__VA_OPT__(…) is handled here rather than in the main loop, because after
// the rewrite there is no group left for '#' to name.
func (p *Preprocessor) vaOptRewrite(m *Macro, is []Token, ap [][]Token) []Token {
	if !m.Variadic {
		return is
	}
	found := false
	for _, t := range is {
		if t.Is("__VA_OPT__") {
			found = true
			break
		}
	}
	if !found {
		return is
	}

	slot := len(m.Params)
	present := slot < len(ap) && len(ap[slot]) > 0

	out := make([]Token, 0, len(is))
	for i := 0; i < len(is); i++ {
		t := is[i]
		if !t.Is("__VA_OPT__") {
			out = append(out, t)
			continue
		}
		end, ok := vaOptEnd(is, i)
		if !ok { // checkBody rejected this already; be inert
			out = append(out, t)
			continue
		}
		content := is[i+2 : end]

		if n := len(out); n > 0 && out[n-1].Kind == token.HASH {
			// # applies to the group's contents with parameters replaced by
			// their unexpanded arguments, which is what # does everywhere.
			hash := out[n-1]
			out = out[:n-1]
			var text []Token
			if present {
				text = substRaw(m, content, ap)
			}
			st := p.gen.Stringize(text)
			st.Flags = hash.Flags & token.FlagAdjacent
			out = append(out, st)
			i = end
			continue
		}

		switch {
		case present:
			out = append(out, spaceLike(t, content)...)
		default:
			if n := len(out); n > 0 && out[n-1].Kind == token.HASHHASH {
				out = out[:n-1]
			} else if end+1 < len(is) && is[end+1].Kind == token.HASHHASH {
				end++
			}
		}
		i = end
	}
	return out
}

// substRaw replaces parameter tokens with their arguments, unexpanded. It is
// what '#' stringizes, and it is deliberately not the full substitution: no
// pre-expansion, no pasting, no hide sets.
func substRaw(m *Macro, is []Token, ap [][]Token) []Token {
	var out []Token
	for _, t := range is {
		if t.Kind == token.IDENT {
			if n := m.Param(t.Text()); n >= 0 && n < len(ap) {
				out = append(out, spaceLike(t, ap[n])...)
				continue
			}
		}
		out = append(out, t)
	}
	return out
}

// spaceLike copies ts, giving its first token the spacing the parameter had
// in the replacement list. `x + y +z` must keep the space before y and not
// before z, whatever the arguments looked like at the call.
func spaceLike(param Token, ts []Token) []Token {
	out := append([]Token(nil), ts...)
	if len(out) > 0 {
		out[0].Flags &^= token.FlagAdjacent
		out[0].Flags |= param.Flags & token.FlagAdjacent
	}
	return out
}

// glue pastes the last token of ls with the first of rs. [cpp.concat]/3
// requires the result be a single preprocessing token; anything else is
// ill-formed, reported once, with both operands left in place so the rest of
// the line still parses.
//
// The pasted token takes its expansion chain from the '##' rather than from
// its left operand, because the macro holding that '##' is what created it.
// Taking it from the operand loses the chain entirely whenever both operands
// are plain arguments — CAT(foo,bar) — and a diagnostic about the result then
// has nowhere to point but the generated arena, which is not a place anyone
// can look at.
func (p *Preprocessor) glue(ls, rs []Token, at Token) []Token {
	if len(ls) == 0 {
		return append(ls, rs...)
	}
	l := ls[len(ls)-1]
	r := rs[0]
	pos, end := p.gen.Paste(l, r)
	spelling := p.gen.Origin().Gen.slice(pos, end)

	kind, ok := p.rescan(spelling)
	if !ok {
		p.errorf(at.Site(), "pasting %q and %q does not give a valid preprocessing token",
			l.Text(), r.Text())
		return append(ls, rs...)
	}
	pasted := Token{
		Kind:   kind,
		Flags:  l.Flags & token.FlagAdjacent,
		Pos:    pos,
		End:    end,
		Origin: p.gen.Origin(),
		Hide:   hsInter(l.Hide, r.Hide),
		Exp:    at.Exp,
	}
	out := append(ls[:len(ls)-1:len(ls)-1], pasted)
	return append(out, rs[1:]...)
}
