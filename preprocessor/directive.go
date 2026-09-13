package preprocessor

import (
	"strconv"
	"strings"

	"github.com/vertex-language/vcx/token"
)

// gnuDirectives defines non-standard GNU directives and their diagnostics.
var gnuDirectives = map[string]string{
	"assert":   "removed from GNU C itself; no ISO equivalent",
	"unassert": "removed from GNU C itself; no ISO equivalent",
	"ident":    "no ISO equivalent",
	"sccs":     "no ISO equivalent",
	"import":   "an MSVC type-library directive; no ISO equivalent",
}

// directive processes one directive line. The '#' has been consumed; line
// holds the rest of the logical line, without the terminator.
func (p *Preprocessor) directive(r *reader, hash Token, line []Token) {
	at := hash.Site()

	// The null directive. It does not invalidate the multiple-include
	// optimization, matching g++, clang++ and cl.exe.
	if len(line) == 0 {
		return
	}

	name := line[0]
	rest := line[1:]
	word := ""
	if name.IsName() {
		word = name.Text()
	}

	// Inside a skipped group only the conditional directives are recognized;
	// everything else, including syntactically invalid directives, is
	// skipped. That is what lets a group hold syntax this revision does not
	// have, and what makes an #if around a missing header safe.
	switch word {
	case "if", "ifdef", "ifndef", "elif", "elifdef", "elifndef", "else", "endif":
	default:
		if r.skipping() {
			return
		}
	}

	// Any directive other than an opening conditional ends the window in
	// which an include guard can start.
	switch word {
	case "if", "ifdef", "ifndef":
	default:
		r.miValid = false
	}

	switch word {
	case "define":
		p.doDefine(rest, at)
	case "undef":
		p.doUndef(rest, at)
	case "include":
		p.doInclude(r, rest, at)
	case "include_next":
		p.doIncludeNext(r, rest, at)
	case "if":
		r.beginIf("#if", at, func() bool { return p.Eval(rest, at) })
		r.noteGuardIf(rest)
	case "ifdef", "ifndef":
		p.doIfdef(r, word == "ifndef", rest, at)
	case "elif":
		r.doElif(p, "#elif", at, func() bool { return p.Eval(rest, at) })
	case "elifdef", "elifndef":
		// C++23, P2334. The spelling exists because #elif defined(X) cannot
		// be written when X is not a valid expression operand, and because
		// the asymmetry with #ifdef was a standing wart.
		neg := word == "elifndef"
		r.doElif(p, "#"+word, at, func() bool { return p.definedOperand(word, rest, at, neg) })
	case "else":
		r.doElse(p, at)
		p.expectEnd(rest, "#else")
	case "endif":
		r.doEndif(p, at)
		p.expectEnd(rest, "#endif")
	case "line":
		p.doLine(rest, at)
	case "error":
		p.errorf(at, "#error %s", spell(rest))
	case "warning":
		p.warn("warning-directive", at, "#warning %s", spell(rest))
	case "pragma":
		p.doPragma(r, rest, at)
	default:
		if why, ok := gnuDirectives[word]; ok {
			p.errorf(at, "#%s is a GNU directive; vcx preprocesses ISO C++", word)
			p.note(at, why)
			return
		}
		p.errorf(name.Site(), "invalid preprocessing directive #%s", name.Text())
	}
}

// definedOperand reads the macro name #ifdef, #ifndef, #elifdef and
// #elifndef all take, and answers their question.
func (p *Preprocessor) definedOperand(word string, line []Token, at Site, negate bool) bool {
	if len(line) == 0 || !line[0].IsName() {
		p.errorf(at, "#%s with no macro name", word)
		return false
	}
	name := line[0].Text()
	p.expectEnd(line[1:], "#"+word)
	v := p.macros.Defined(name)
	if v {
		p.macros.Lookup(name).Used = true
	} else {
		v = p.isOperatorName(name)
	}
	if negate {
		return !v
	}
	return v
}

// doDefine enters a macro, applying [cpp.replace]/2's redefinition rule. The
// grammar itself is macro.go's, because -D runs the same one.
func (p *Preprocessor) doDefine(line []Token, at Site) {
	if len(line) == 0 {
		p.errorf(at, "#define with no macro name")
		return
	}
	m := p.parseDefine(line)
	if m == nil {
		return
	}
	// [macro.names]/2: a program that includes a standard header may not
	// define a name lexically identical to a keyword or an alternative
	// token. It is a rule about the program rather than a constraint on this
	// package, so it is a warning — and a loud one, because `#define new` is
	// how a translation unit stops compiling three headers later.
	if token.IsStandardKeyword(m.Name, p.cfg.Std) || token.IsAltToken(m.Name) {
		p.warn("keyword-macro", m.Def, "%q is a keyword; defining it is ill-formed "+
			"in any translation unit that includes a standard header", m.Name)
	}
	if prev := p.macros.Lookup(m.Name); prev != nil && !SameDefinition(prev, m) {
		if p.warn("macro-redefined", m.Def, "%q redefined", m.Name) && prev.Def.Valid() {
			p.note(prev.Def, "previous definition is here")
		}
	}
	p.macros.Define(m)
}

func (p *Preprocessor) doUndef(line []Token, at Site) {
	if len(line) == 0 {
		p.errorf(at, "#undef with no macro name")
		return
	}
	name := line[0]
	if !name.IsName() {
		p.errorf(name.Site(), "macro name must be an identifier")
		return
	}
	if Reserved(name.Text()) {
		p.errorf(name.Site(), "%q cannot be undefined", name.Text())
		return
	}
	p.macros.Undef(name.Text())
	p.expectEnd(line[1:], "#undef")
}

func (p *Preprocessor) doIfdef(r *reader, negate bool, line []Token, at Site) {
	word := "#ifdef"
	if negate {
		word = "#ifndef"
	}
	if r.skipping() {
		r.pushCond(cond{site: at, directive: word})
		return
	}
	if len(line) == 0 || !line[0].IsName() {
		p.errorf(at, "%s with no macro name", word)
		r.pushCond(cond{site: at, directive: word})
		return
	}
	v := p.definedOperand(strings.TrimPrefix(word, "#"), line, at, negate)
	c := cond{site: at, directive: word, taken: v, active: v}
	// #ifndef GUARD at the top of a file, with nothing before it, is the
	// include-guard idiom.
	if negate && r.miValid && len(r.conds) == 0 && !r.sawToken {
		c.guard = line[0].Text()
	}
	r.pushCond(c)
}

// noteGuardIf recognizes the other guard spelling: #if !defined FOO.
func (r *reader) noteGuardIf(line []Token) {
	c := r.topCond()
	if c == nil || !r.miValid || len(r.conds) != 1 || r.sawToken {
		return
	}
	if len(line) >= 2 && line[0].Kind == token.NOT && line[1].Is("defined") {
		for _, t := range line[2:] {
			if t.Kind == token.IDENT {
				c.guard = t.Text()
				return
			}
		}
	}
}

// doLine implements [cpp.line]. It changes what __LINE__ and __FILE__ report;
// it does not move any span, because a diagnostic must underline what the
// user actually typed.
func (p *Preprocessor) doLine(line []Token, at Site) {
	line = p.expandClosed(line)
	if len(line) == 0 || line[0].Kind != token.INT_LIT {
		p.errorf(at, "#line requires a digit sequence")
		return
	}
	n := 0
	for _, c := range line[0].Text() {
		if c < '0' || c > '9' {
			p.errorf(line[0].Site(), "#line requires a digit sequence")
			return
		}
		n = n*10 + int(c-'0')
	}
	// [cpp.line]/2 numbers the line *following* the directive, so the offset
	// is measured from the line after this one.
	p.lineDelta = n - p.physicalLine(at) - 1
	if len(line) > 1 {
		if line[1].Kind != token.STRING_LIT {
			p.errorf(line[1].Site(), "#line filename must be a string literal")
			return
		}
		p.fileName = strings.Trim(line[1].Text(), `"`)
	}
}

// doPragma processes #pragma directives, honoring `#pragma once` and passing unrecognized pragmas through ([cpp.pragma]).
func (p *Preprocessor) doPragma(r *reader, line []Token, at Site) {
	if len(line) > 0 && line[0].Is("once") {
		r.once = true
		if at.Origin == nil || !at.Origin.System {
			if p.warn("pragma-once", at, "#pragma once is not ISO C++") {
				p.note(at, "vcx honours it; the portable spelling is an #ifndef "+
					"include guard, which vcx detects on its own")
			}
		}
		return
	}
	if len(line) > 0 && (line[0].Is("push_macro") || line[0].Is("pop_macro")) {
		p.pushPopMacro(line, at)
		return
	}
	if len(line) > 0 && (line[0].Is("GCC") || line[0].Is("clang")) {
		if p.cfg.Vendor != nil {
			// The GNU dialect's own pragmas. system_header is the one
			// that means something here: the rest of the file is a
			// system header's, and warns as one. The rest -- diagnostic
			// push, pop and ignored -- adjust warnings a build sets.
			if len(line) > 1 && line[1].Is("system_header") && r.org != nil {
				r.org.System = true
			}
			return
		}
		if p.warn("vendor-pragma", at, "#pragma %s is a vendor extension and has no effect",
			line[0].Text()) {
			p.note(at, "warning state is a build decision, not a source one")
		}
		return
	}

	// Re-emit pragma line into output for phase 7.
	hash := p.gen.Mint(token.HASH, "#")
	hash.Flags = token.FlagNLBefore
	word := p.gen.Mint(token.IDENT, "pragma")
	word.Flags = token.FlagAdjacent
	p.out = append(p.out, hash, word)
	p.out = append(p.out, line...)
}

func (p *Preprocessor) expectEnd(rest []Token, what string) {
	if len(rest) > 0 {
		p.warn("extra-tokens", rest[0].Site(), "extra tokens at end of %s directive", what)
	}
}

func spell(ts []Token) string {
	var b strings.Builder
	for i, t := range ts {
		if i > 0 && t.Spaced() {
			b.WriteByte(' ')
		}
		b.WriteString(t.Text())
	}
	return b.String()
}

// pushPopMacro is `#pragma push_macro("name")` and `pop_macro("name")`,
// which cl, gcc and clang all implement: the macro's definition -- or its
// absence -- is saved, and restored by the matching pop, whatever was done
// to the name in between. A header that #undefs min and max for its own
// use restores the program's with it.
func (p *Preprocessor) pushPopMacro(line []Token, at Site) {
	if len(line) < 4 || line[1].Kind != token.LPAREN || line[2].Kind != token.STRING_LIT || line[3].Kind != token.RPAREN {
		p.warn("pragma-syntax", at, "#pragma %s expects a parenthesized string", line[0].Text())
		return
	}
	name, err := strconv.Unquote(line[2].Text())
	if err != nil {
		return
	}
	if p.pushedMacros == nil {
		p.pushedMacros = map[string][]*Macro{}
	}
	if line[0].Is("push_macro") {
		p.pushedMacros[name] = append(p.pushedMacros[name], p.macros.Lookup(name))
		return
	}
	stack := p.pushedMacros[name]
	if len(stack) == 0 {
		return
	}
	saved := stack[len(stack)-1]
	p.pushedMacros[name] = stack[:len(stack)-1]
	if saved == nil {
		p.macros.Undef(name)
		return
	}
	p.macros.Restore(saved)
}
