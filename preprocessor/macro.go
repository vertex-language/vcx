package preprocessor

import (
	"github.com/vertex-language/vcx/token"
)

// Macro is one entry in the macro table.
//
// Params holds the formal parameter names in order. For a variadic macro the
// trailing __VA_ARGS__ is not in Params; Variadic records it, and argument
// collection maps the last slot to everything past the named ones. Body is
// the replacement list, with the parameters left as ordinary IDENT tokens,
// looked up by name during substitution.
//
// ObjLike distinguishes #define M x from #define M() x. The difference is not
// len(Params): a function-like macro with an empty parameter list has zero
// params and is still only invoked when followed by '('.
type Macro struct {
	ID       int // dense, assigned on definition; hide sets store these
	Name     string
	ObjLike  bool
	Variadic bool
	Params   []string
	Body     []Token

	Def     Site
	Builtin Builtin

	// Used records whether the macro was ever expanded or tested. Nothing in
	// the language depends on it; the unused-macro warning does.
	Used bool
}

// Param reports the index of name in the macro's formal parameters, or -1.
// __VA_ARGS__ resolves to the variadic slot, which sits one past the named
// parameters.
func (m *Macro) Param(name string) int {
	if m.ObjLike {
		return -1
	}
	for i, p := range m.Params {
		if p == name {
			return i
		}
	}
	if m.Variadic && name == "__VA_ARGS__" {
		return len(m.Params)
	}
	return -1
}

// Arity is the number of argument slots collection must fill: the named
// parameters plus the variadic slot when there is one.
func (m *Macro) Arity() int {
	if m.Variadic {
		return len(m.Params) + 1
	}
	return len(m.Params)
}

// Table is the macro table: names to definitions, plus the ID allocator hide
// sets index into.
//
// The table is per-translation-unit state, not per-file: a #define in a
// header outlives the header, which is the whole point of a header. What does
// not outlive anything is a module interface — macros are not exported by
// modules ([module.import]/6), so importing one adds nothing here. That is
// the single largest thing modules change about this package, and it is
// expressed by an absence.
type Table struct {
	byName map[string]*Macro
	next   int
}

// NewTable returns an empty table.
func NewTable() *Table {
	return &Table{byName: make(map[string]*Macro, 512)}
}

// Lookup returns the macro named name, or nil.
func (t *Table) Lookup(name string) *Macro { return t.byName[name] }

// Defined reports whether name is currently defined — the `defined`
// operator's question, and #ifdef's.
func (t *Table) Defined(name string) bool { _, ok := t.byName[name]; return ok }

// Define enters m, assigning its ID. It returns the macro previously bound to
// the name, if any, so the caller can apply [cpp.replace]/2: a redefinition
// is permitted only when the two definitions are identical, and must be
// reported otherwise. Define itself does not report.
func (t *Table) Define(m *Macro) (prev *Macro) {
	prev = t.byName[m.Name]
	if prev != nil {
		// Reuse the ID so hide sets captured under the old definition still
		// name this macro. A redefinition that reaches here is either
		// identical (harmless) or already reported.
		m.ID = prev.ID
	} else {
		m.ID = t.next
		t.next++
	}
	t.byName[m.Name] = m
	return prev
}

// Restore binds a macro that was defined before, keeping the ID it was
// given then, so that hide sets naming it still do.
func (t *Table) Restore(m *Macro) { t.byName[m.Name] = m }

// Undef removes name and reports whether it was bound. Undefining a name that
// is not defined is not an error.
func (t *Table) Undef(name string) bool {
	if _, ok := t.byName[name]; !ok {
		return false
	}
	delete(t.byName, name)
	return true
}

// Len reports how many macros are defined.
func (t *Table) Len() int { return len(t.byName) }

// Each calls f for every defined macro, in unspecified order.
func (t *Table) Each(f func(*Macro)) {
	for _, m := range t.byName {
		f(m)
	}
}

// SameDefinition implements [cpp.replace]/2's identity test: two definitions
// are the same if they have the same kind, the same spelling of parameters in
// the same order, and replacement lists that agree token by token in spelling
// and in whitespace separation. "Whitespace separation" is presence, not
// amount, so this compares FlagAdjacent and not the trivia itself.
func SameDefinition(a, b *Macro) bool {
	if a.ObjLike != b.ObjLike || a.Variadic != b.Variadic || len(a.Params) != len(b.Params) {
		return false
	}
	for i := range a.Params {
		if a.Params[i] != b.Params[i] {
			return false
		}
	}
	if len(a.Body) != len(b.Body) {
		return false
	}
	for i := range a.Body {
		x, y := a.Body[i], b.Body[i]
		if x.Kind != y.Kind || x.Text() != y.Text() {
			return false
		}
		// The first token of a replacement list is separated from the
		// '#define' line by definition; only interior separation counts.
		if i > 0 && x.Spaced() != y.Spaced() {
			return false
		}
	}
	return true
}

// Reserved reports whether name is one the program may not #define or #undef.
//
// [cpp.predefined]/2 forbids both for the predefined macros; [cpp.cond]/12
// and [cpp.replace]/5 forbid them for the operators and for the two variadic
// identifiers, which are not macros at all and cannot be made into ones.
//
// The wider restriction of [macro.names] — no keyword, no alternative token —
// is a rule about programs that include standard headers rather than a
// constraint on the preprocessor, so it is a warning from the directive
// grammar and not a member of this set.
func Reserved(name string) bool {
	switch name {
	case "defined", "__has_include", "__has_cpp_attribute",
		"__VA_ARGS__", "__VA_OPT__",
		"__FILE__", "__LINE__", "__DATE__", "__TIME__",
		"__cplusplus", "__STDC_HOSTED__":
		return true
	}
	return false
}

// ---- the replacement-list grammar ----

// parseDefine parses the tokens after `#define` and returns the macro, or nil
// if the line is too broken to be one. Diagnostics are reported here; the
// caller decides what to do with the result.
//
// This grammar lives with the macro rather than with the directives because
// -D runs it too, and a -D that parsed differently from a #define would be a
// bug nobody could see.
func (p *Preprocessor) parseDefine(toks []Token) *Macro {
	if len(toks) == 0 {
		return nil
	}
	name := toks[0]
	if !name.IsName() {
		// An alternative token is an operator, not an identifier
		// ([lex.operators]), so `#define and &&` fails here rather than at
		// [macro.names]. Naming which of the two it is saves the reader a
		// trip to the grammar.
		if token.IsAltToken(name.Text()) {
			p.errorf(name.Site(),
				"%q is an alternative spelling of an operator and cannot be a macro name",
				name.Text())
			return nil
		}
		p.errorf(name.Site(), "macro name must be an identifier")
		return nil
	}
	m := &Macro{Name: name.Text(), ObjLike: true, Def: name.Site()}
	if Reserved(m.Name) {
		p.errorf(name.Site(), "%q cannot be defined", m.Name)
		return nil
	}
	rest := toks[1:]

	// A '(' makes it function-like only when it is adjacent to the name.
	// `#define M (x)` is an object-like macro whose body begins with a
	// parenthesis, and the space is the entire difference.
	if len(rest) > 0 && rest[0].Kind == token.LPAREN && !rest[0].Spaced() {
		m.ObjLike = false
		var ok bool
		rest, ok = p.parseParams(m, rest[1:])
		if !ok {
			return nil
		}
	}

	m.Body = rest
	if !p.checkBody(m) {
		return nil
	}
	return m
}

// parseParams consumes the parameter list, the tokens after the '(', and
// returns what follows the ')'.
func (p *Preprocessor) parseParams(m *Macro, toks []Token) ([]Token, bool) {
	for i := 0; ; {
		if i >= len(toks) {
			p.errorf(m.Def, "unterminated parameter list for macro %q", m.Name)
			return nil, false
		}
		t := toks[i]

		switch {
		case t.Kind == token.RPAREN:
			return toks[i+1:], true

		case t.Kind == token.ELLIPSIS:
			m.Variadic = true
			i++
			if i >= len(toks) || toks[i].Kind != token.RPAREN {
				p.errorf(t.Site(), "'...' must be the last macro parameter")
				return nil, false
			}
			return toks[i+1:], true

		case t.IsName():
			n := t.Text()
			if n == "__VA_ARGS__" || n == "__VA_OPT__" {
				p.errorf(t.Site(), "%q cannot be used as a macro parameter", n)
				return nil, false
			}
			for _, prev := range m.Params {
				if prev == n {
					p.errorf(t.Site(), "duplicate macro parameter %q", n)
					return nil, false
				}
			}
			m.Params = append(m.Params, n)
			i++

		default:
			p.errorf(t.Site(), "expected a parameter name in macro %q", m.Name)
			return nil, false
		}

		// A separator must follow a parameter name.
		if i < len(toks) && toks[i].Kind == token.COMMA {
			i++
			continue
		}
		if i < len(toks) && (toks[i].Kind == token.RPAREN || toks[i].Kind == token.ELLIPSIS) {
			continue
		}
		if i >= len(toks) {
			p.errorf(m.Def, "unterminated parameter list for macro %q", m.Name)
			return nil, false
		}
		p.errorf(toks[i].Site(), "expected ',' or ')' in macro parameter list")
		return nil, false
	}
}

// checkBody applies the constraints on a replacement list: ## may not be at
// either end, # must precede a parameter in a function-like macro, and the
// two variadic identifiers may appear only where a variadic macro puts them.
func (p *Preprocessor) checkBody(m *Macro) bool {
	body := m.Body
	if len(body) > 0 {
		if body[0].Kind == token.HASHHASH {
			p.errorf(body[0].Site(), "'##' cannot appear at the start of a replacement list")
			return false
		}
		if last := body[len(body)-1]; last.Kind == token.HASHHASH {
			p.errorf(last.Site(), "'##' cannot appear at the end of a replacement list")
			return false
		}
	}
	for i := 0; i < len(body); i++ {
		t := body[i]
		switch {
		case t.Kind == token.HASH && !m.ObjLike:
			// [cpp.stringize]/1. The GNU spelling `#__VA_OPT__(x)` is
			// permitted for the same reason __VA_ARGS__ is.
			if i+1 >= len(body) || (m.Param(body[i+1].Text()) < 0 && !body[i+1].Is("__VA_OPT__")) {
				p.errorf(t.Site(), "'#' must be followed by a macro parameter")
				return false
			}

		case t.Is("__VA_ARGS__"):
			if !m.Variadic {
				p.errorf(t.Site(),
					"__VA_ARGS__ can only appear in a variadic macro's replacement list")
				return false
			}

		case t.Is("__VA_OPT__"):
			if !m.Variadic {
				p.errorf(t.Site(),
					"__VA_OPT__ can only appear in a variadic macro's replacement list")
				return false
			}
			end, ok := vaOptEnd(body, i)
			if !ok {
				p.errorf(t.Site(), "__VA_OPT__ must be followed by a parenthesized group")
				return false
			}
			for j := i + 2; j < end; j++ {
				if body[j].Is("__VA_OPT__") {
					p.errorf(body[j].Site(), "__VA_OPT__ cannot be nested")
					return false
				}
			}
			if end == i+2 {
				break // __VA_OPT__() is empty and legal
			}
			if body[i+2].Kind == token.HASHHASH || body[end-1].Kind == token.HASHHASH {
				p.errorf(t.Site(), "'##' cannot appear at either end of a __VA_OPT__ group")
				return false
			}
		}
	}
	return true
}

// vaOptEnd returns the index of the ')' closing the group that __VA_OPT__ at
// index i introduces, and whether the group is well formed.
func vaOptEnd(body []Token, i int) (int, bool) {
	if i+1 >= len(body) || body[i+1].Kind != token.LPAREN {
		return 0, false
	}
	depth := 0
	for j := i + 1; j < len(body); j++ {
		switch body[j].Kind {
		case token.LPAREN:
			depth++
		case token.RPAREN:
			if depth--; depth == 0 {
				return j, true
			}
		}
	}
	return 0, false
}

// DefineText applies one -D spelling: "NDEBUG", "VERSION=3", or
// "MAX(a,b)=((a)>(b)?(a):(b))". A define with no '=' takes the value 1, which
// is what every C++ toolchain does with a bare -D.
//
// The '=' is turned into a space and the result is handed to the same grammar
// #define runs, so the two cannot drift apart. Only the '=' that ends the
// name-and-parameters prefix is rewritten; any later one is part of the
// replacement list.
func (p *Preprocessor) DefineText(text string) *Macro {
	src := text
	if i := splitDefine(text); i < 0 {
		src = text + " 1"
	} else {
		src = text[:i] + " " + text[i+1:]
	}
	toks := p.Scan(token.NewFile("<command line>", []byte(src+"\n")))
	m := p.parseDefine(trimEOF(toks))
	if m == nil {
		return nil
	}
	p.macros.Define(m)
	return m
}

// UndefText applies one -U spelling.
func (p *Preprocessor) UndefText(name string) bool { return p.macros.Undef(name) }

// splitDefine returns the index of the '=' that separates a -D's name and
// parameter list from its replacement list, or -1 when there is none.
func splitDefine(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case '=':
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// trimEOF drops the scanner's terminating EOF token.
func trimEOF(ts []Token) []Token {
	if n := len(ts); n > 0 && ts[n-1].Kind == token.EOF {
		return ts[:n-1]
	}
	return ts
}
