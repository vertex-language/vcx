package preprocessor

import (
	"strings"

	"github.com/vertex-language/vcx/token"
)

// [cpp.cond] is a different language from [expr.const], and reusing the
// constant evaluator would be the wrong semantics rather than a shortcut:
// there is no sizeof, no enumerator, no cast, no literal operator; every
// identifier that survives expansion is 0 except true and false; `defined`,
// `__has_include` and `__has_cpp_attribute` are operators; and arithmetic is
// in intmax_t/uintmax_t whatever int is on the target.
type value struct {
	u        uint64
	unsigned bool
}

func (v value) i() int64      { return int64(v.u) }
func (v value) nonzero() bool { return v.u != 0 }

// usual applies the usual arithmetic conversions in the two types this
// evaluator has.
func usual(a, b value) (value, value, bool) {
	un := a.unsigned || b.unsigned
	a.unsigned, b.unsigned = un, un
	return a, b, un
}

type evaluator struct {
	p    *Preprocessor
	toks []Token
	i    int
	site Site
	bad  bool
}

// Eval evaluates a #if, #elif or #elifdef controlling expression.
//
// Order matters and is the standard's: the operators are resolved first, over
// the unexpanded line, because `#if defined FOO` must not expand FOO and
// `#if __has_include(<x.h>)` must not expand the header name. What is left is
// macro-expanded. Only then does every remaining identifier become 0 — except
// true and false, which [cpp.cond]/1 keeps as themselves, and which is the
// clearest single difference between this evaluator and C's.
//
// An operator *produced* by expansion is unspecified ([cpp.cond]/12). It is
// evaluated as the operator anyway, under a named warning: g++ and clang++
// both do, real standard-library headers lean on it, and the alternative is
// refusing to preprocess them.
func (p *Preprocessor) Eval(line []Token, at Site) bool {
	if len(line) == 0 {
		p.errorf(at, "#if with no expression")
		return false
	}
	line = p.resolveOperators(line, at)
	line = p.expandClosed(line)
	line = p.resolveExpandedOperators(line, at)
	line = p.zeroIdents(line)

	e := &evaluator{p: p, toks: line, site: at}
	v := e.conditional()
	if !e.bad && e.i < len(e.toks) {
		e.fail(e.toks[e.i].Site(), "unexpected %q in preprocessor expression", e.toks[e.i].Text())
	}
	return !e.bad && v.nonzero()
}

// resolveOperators replaces `defined X`, `defined (X)`, `__has_include(…)`
// and `__has_cpp_attribute(…)` with 1 or 0.
func (p *Preprocessor) resolveOperators(line []Token, at Site) []Token {
	out := make([]Token, 0, len(line))
	for i := 0; i < len(line); i++ {
		t := line[i]
		switch {
		case t.Is("defined"):
			n, next, ok := p.evalDefined(line, i)
			if !ok {
				out = append(out, p.number(t, 0))
				return out
			}
			out = append(out, p.number(t, n))
			i = next

		case t.Is("__has_include"):
			n, next, ok := p.evalHasInclude(line, i)
			if !ok {
				out = append(out, p.number(t, 0))
				return out
			}
			out = append(out, p.number(t, n))
			i = next

		case t.Is("__has_cpp_attribute"):
			n, next, ok := p.evalHasAttribute(line, i)
			if !ok {
				out = append(out, p.number(t, 0))
				return out
			}
			out = append(out, p.number(t, n))
			i = next

		case p.cfg.Vendor != nil && t.Kind == token.IDENT && vendorOperator(t.Text()):
			n, next, ok := p.evalVendor(line, i)
			if !ok {
				out = append(out, p.number(t, 0))
				return out
			}
			out = append(out, p.number(t, n))
			i = next

		default:
			out = append(out, t)
		}
	}
	return out
}

// evalDefined reads the operand of `defined` starting at line[i], and returns
// its value and the index of its last token.
func (p *Preprocessor) evalDefined(line []Token, i int) (n, end int, ok bool) {
	t := line[i]
	j := i + 1
	paren := j < len(line) && line[j].Kind == token.LPAREN
	if paren {
		j++
	}
	if j >= len(line) || !line[j].IsName() {
		p.errorf(t.Site(), `operator "defined" requires an identifier`)
		return 0, 0, false
	}
	name := line[j].Text()
	if paren {
		if j+1 >= len(line) || line[j+1].Kind != token.RPAREN {
			p.errorf(t.Site(), `missing ')' after "defined"`)
		} else {
			j++
		}
	}
	if p.macros.Defined(name) {
		p.macros.Lookup(name).Used = true
		return 1, j, true
	}
	if p.isOperatorName(name) {
		return 1, j, true
	}
	return 0, j, true
}

// evalHasAttribute implements __has_cpp_attribute ([cpp.cond]/5). The value
// is the revision in which the attribute was added, which is what a header
// compares against.
func (p *Preprocessor) evalHasAttribute(line []Token, i int) (n, end int, ok bool) {
	t := line[i]
	j := i + 1
	if j >= len(line) || line[j].Kind != token.LPAREN {
		p.errorf(t.Site(), "__has_cpp_attribute requires a parenthesized attribute name")
		return 0, 0, false
	}
	j++
	var b strings.Builder
	depth := 1
	for ; j < len(line); j++ {
		switch line[j].Kind {
		case token.LPAREN:
			depth++
		case token.RPAREN:
			if depth--; depth == 0 {
				return cppAttributes[b.String()], j, true
			}
		}
		b.WriteString(line[j].Text())
	}
	p.errorf(t.Site(), "missing ')' after __has_cpp_attribute")
	return 0, 0, false
}

// cppAttributes is the standard attribute set, each mapped to the value
// [cpp.cond] assigns it — the revision that introduced it, as a date. An
// attribute vcx does not implement is absent, and absent is 0, which is the
// answer a header wants: it will write the code that does not use it.
//
// Vendor attributes are deliberately not here. __has_cpp_attribute is asked
// about `gnu::always_inline` by real headers, and answering 0 is answering
// truthfully.
var cppAttributes = map[string]int{
	"noreturn":           200809,
	"carries_dependency": 200809,
	"deprecated":         201309,
	"fallthrough":        201603,
	"nodiscard":          201907,
	"maybe_unused":       201603,
	"likely":             201803,
	"unlikely":           201803,
	"no_unique_address":  201803,
	"assume":             202207,
	"indeterminate":      202403,
}

// resolveExpandedOperators handles an operator that macro expansion produced.
// One warning per controlling expression, however many the expansion yielded;
// the warning is named, so a system header that does this on every inclusion
// reports once per translation unit. The resolution itself is
// resolveOperators, unchanged — an operator means the same thing wherever it
// came from.
func (p *Preprocessor) resolveExpandedOperators(line []Token, at Site) []Token {
	for _, t := range line {
		if t.Is("defined") || t.Is("__has_include") || t.Is("__has_cpp_attribute") ||
			(p.cfg.Vendor != nil && t.Kind == token.IDENT && vendorOperator(t.Text())) {
			p.warn("expansion-operator", t.Site(),
				"%q produced by macro expansion is unspecified; vcx evaluates it as the operator, matching g++ and clang++",
				t.Text())
			return p.resolveOperators(line, at)
		}
	}
	return line
}

func (p *Preprocessor) number(at Token, n int) Token {
	t := p.gen.Mint(token.INT_LIT, itoa(n))
	t.Flags = at.Flags & token.FlagAdjacent
	t.Exp = at.Exp
	return t
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// zeroIdents implements [cpp.cond]/1: identifiers remaining after expansion
// are replaced by 0 — except true and false, which keep their values. That
// exception is the reason this cannot be shared with a C preprocessor:
// #if true is 1 here and 0 there.
//
// Keywords are identifiers for this purpose — `#if sizeof` is 0, not an error
// — which is why this runs on kinds the scanner already classified. No
// operator can reach this point: both resolve passes consume every spelling,
// and the malformed-operand path truncates the line.
func (p *Preprocessor) zeroIdents(line []Token) []Token {
	for i, t := range line {
		switch {
		case t.Kind == token.TRUE || t.Kind == token.FALSE:
			// left alone; unary() reads them
		case t.Kind == token.IDENT || t.Kind.IsKeyword():
			line[i] = p.number(t, 0)
		}
	}
	return line
}

func (e *evaluator) fail(s Site, f string, a ...any) {
	if !e.bad {
		e.p.errorf(s, f, a...)
		e.bad = true
	}
}

func (e *evaluator) peek() (Token, bool) {
	if e.i >= len(e.toks) {
		return Token{}, false
	}
	return e.toks[e.i], true
}

func (e *evaluator) accept(k token.Kind) bool {
	if t, ok := e.peek(); ok && t.Kind == k {
		e.i++
		return true
	}
	return false
}

func (e *evaluator) conditional() value {
	c := e.binary(1)
	if !e.accept(token.QUESTION) {
		return c
	}
	// Both arms are parsed; only the taken one may report. Division by zero
	// in the untaken arm of `#if 0 ? 1/0 : 1` is not a mistake.
	save, nd := e.bad, len(e.p.diags)
	t := e.conditional()
	if !e.accept(token.COLON) {
		e.fail(e.site, "expected ':' in preprocessor conditional")
		return value{}
	}
	f := e.conditional()
	if c.nonzero() {
		return t
	}
	e.bad = save
	e.p.diags = e.p.diags[:nd]
	return f
}

// prec gives the binary levels [cpp.cond] admits. It is not
// token.Kind.Precedence: <=> is not in a controlling expression (it yields a
// class type, which this evaluator has no notion of), and neither are .* and
// ->*. Sharing the table would import three levels that cannot occur.
func prec(k token.Kind) int {
	switch k {
	case token.LOR:
		return 1
	case token.LAND:
		return 2
	case token.OR:
		return 3
	case token.XOR:
		return 4
	case token.AND:
		return 5
	case token.EQL, token.NEQ:
		return 6
	case token.LSS, token.GTR, token.LEQ, token.GEQ:
		return 7
	case token.SHL, token.SHR:
		return 8
	case token.ADD, token.SUB:
		return 9
	case token.MUL, token.QUO, token.REM:
		return 10
	}
	return 0
}

func (e *evaluator) binary(min int) value {
	lhs := e.unary()
	for {
		t, ok := e.peek()
		if !ok {
			return lhs
		}
		pr := prec(t.Kind)
		if pr < min {
			return lhs
		}
		e.i++

		// && and || short-circuit even here, so `defined(X) && X > 2` is safe
		// and `0 && 1/0` folds without reporting.
		if t.Kind == token.LAND || t.Kind == token.LOR {
			save, nd := e.bad, len(e.p.diags)
			rhs := e.binary(pr + 1)
			skip := (t.Kind == token.LAND && !lhs.nonzero()) ||
				(t.Kind == token.LOR && lhs.nonzero())
			if skip {
				// Retract what the unevaluated arm reported. Clearing the
				// flag is not enough: fail() has already appended, and
				// `#if 0 && 1/0` must be silent, not merely non-fatal.
				e.bad = save
				e.p.diags = e.p.diags[:nd]
			}
			n := uint64(0)
			if (t.Kind == token.LAND && lhs.nonzero() && rhs.nonzero()) ||
				(t.Kind == token.LOR && (lhs.nonzero() || rhs.nonzero())) {
				n = 1
			}
			lhs = value{u: n}
			continue
		}

		rhs := e.binary(pr + 1)
		lhs = e.apply(t, lhs, rhs)
	}
}

func (e *evaluator) apply(op Token, a, b value) value {
	boolean := func(t bool) value {
		if t {
			return value{u: 1}
		}
		return value{}
	}
	switch op.Kind {
	case token.SHL, token.SHR:
		// Shifts do not convert: the left operand's type wins.
		n := b.u & 63
		if op.Kind == token.SHL {
			return value{u: a.u << n, unsigned: a.unsigned}
		}
		if a.unsigned {
			return value{u: a.u >> n, unsigned: true}
		}
		return value{u: uint64(a.i() >> n)}
	}

	a, b, un := usual(a, b)
	switch op.Kind {
	case token.ADD:
		return value{u: a.u + b.u, unsigned: un}
	case token.SUB:
		return value{u: a.u - b.u, unsigned: un}
	case token.MUL:
		return value{u: a.u * b.u, unsigned: un}
	case token.QUO, token.REM:
		if b.u == 0 {
			e.fail(op.Site(), "division by zero in preprocessor expression")
			return value{}
		}
		if un {
			if op.Kind == token.QUO {
				return value{u: a.u / b.u, unsigned: true}
			}
			return value{u: a.u % b.u, unsigned: true}
		}
		if op.Kind == token.QUO {
			return value{u: uint64(a.i() / b.i())}
		}
		return value{u: uint64(a.i() % b.i())}
	case token.AND:
		return value{u: a.u & b.u, unsigned: un}
	case token.OR:
		return value{u: a.u | b.u, unsigned: un}
	case token.XOR:
		return value{u: a.u ^ b.u, unsigned: un}
	case token.EQL:
		return boolean(a.u == b.u)
	case token.NEQ:
		return boolean(a.u != b.u)
	case token.LSS:
		if un {
			return boolean(a.u < b.u)
		}
		return boolean(a.i() < b.i())
	case token.GTR:
		if un {
			return boolean(a.u > b.u)
		}
		return boolean(a.i() > b.i())
	case token.LEQ:
		if un {
			return boolean(a.u <= b.u)
		}
		return boolean(a.i() <= b.i())
	case token.GEQ:
		if un {
			return boolean(a.u >= b.u)
		}
		return boolean(a.i() >= b.i())
	}
	e.fail(op.Site(), "%q is not valid in a preprocessor expression", op.Text())
	return value{}
}

func (e *evaluator) unary() value {
	t, ok := e.peek()
	if !ok {
		e.fail(e.site, "expected an expression")
		return value{}
	}
	switch t.Kind {
	case token.ADD:
		e.i++
		return e.unary()
	case token.SUB:
		e.i++
		v := e.unary()
		return value{u: -v.u, unsigned: v.unsigned}
	case token.TILDE:
		e.i++
		v := e.unary()
		return value{u: ^v.u, unsigned: v.unsigned}
	case token.NOT:
		e.i++
		v := e.unary()
		if v.nonzero() {
			return value{}
		}
		return value{u: 1}
	case token.LPAREN:
		e.i++
		v := e.conditional()
		if !e.accept(token.RPAREN) {
			e.fail(t.Site(), "missing ')' in preprocessor expression")
		}
		return v
	case token.TRUE:
		e.i++
		return value{u: 1}
	case token.FALSE:
		e.i++
		return value{}
	case token.INT_LIT:
		e.i++
		return e.intConst(t)
	case token.CHAR_LIT:
		e.i++
		return e.charConst(t)
	case token.FLOAT_LIT:
		e.i++
		e.fail(t.Site(), "floating literal in preprocessor expression")
		return value{}
	case token.STRING_LIT:
		e.i++
		e.fail(t.Site(), "string literal in preprocessor expression")
		return value{}
	}
	e.fail(t.Site(), "unexpected %q in preprocessor expression", t.Text())
	return value{}
}

// intConst decodes a pp-number as an integer literal. This is the second
// decoder in the tree — sema has the [expr.const] one — and it must stay in
// step with it, because `#if 'x' == 120` and `char c = 'x';` are asking about
// the same source text.
//
// Digit separators are removed first: 1'000'000 is one token here as it is
// everywhere else, and the separators carry no value.
func (e *evaluator) intConst(t Token) value {
	s := strings.ReplaceAll(t.Text(), "'", "")
	base := 10
	switch {
	case len(s) > 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X'):
		base, s = 16, s[2:]
	case len(s) > 2 && s[0] == '0' && (s[1] == 'b' || s[1] == 'B'):
		base, s = 2, s[2:]
	case len(s) > 1 && s[0] == '0':
		base, s = 8, s[1:]
	}

	// MSVC's sized suffix comes off before the u/l/z loop so the digit
	// scanner sees only digits.
	unsigned := false
	s, unsigned = stripMSVCSuffix(s, unsigned)

suffix:
	for len(s) > 0 {
		switch c := s[len(s)-1]; c {
		case 'u', 'U':
			unsigned = true
		case 'l', 'L', 'z', 'Z': // z and Z are C++23's size_t suffix
		default:
			break suffix
		}
		s = s[:len(s)-1]
	}

	var v uint64
	over := false
	for i := 0; i < len(s); i++ {
		d := digit(s[i])
		if d < 0 || d >= base {
			e.fail(t.Site(), "invalid digit %q in literal", string(s[i]))
			return value{}
		}
		n := v*uint64(base) + uint64(d)
		if n < v {
			over = true
		}
		v = n
	}
	if over {
		e.fail(t.Site(), "integer literal is too large for intmax_t")
	}
	// A decimal literal too large for intmax_t is unsigned; hex, octal and
	// binary reach unsigned candidates earlier. Both land here.
	if v > 1<<63-1 {
		unsigned = true
	}
	return value{u: v, unsigned: unsigned}
}

// stripMSVCSuffix strips an MSVC sized integer suffix and updates the
// unsigned flag.
func stripMSVCSuffix(s string, unsigned bool) (string, bool) {
	for _, suf := range []string{"i64", "I64", "i32", "I32", "i16", "I16", "i8", "I8"} {
		if len(s) > len(suf) && s[len(s)-len(suf):] == suf {
			pos := len(s) - len(suf)
			if pos > 0 && (s[pos-1] == 'u' || s[pos-1] == 'U') {
				return s[:pos-1], true
			}
			return s[:pos], unsigned
		}
	}
	return s, unsigned
}

func digit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// charConst decodes a character literal. The encoding prefix is stripped and
// remembered: a prefixed literal holds one character with its own value,
// where a plain multi-character literal accumulates left to right with an
// implementation-defined value — vcx's is the same accumulation g++ and
// cl.exe both use.
func (e *evaluator) charConst(t Token) value {
	s := t.Text()
	prefixed := false
	for len(s) > 0 && s[0] != '\'' {
		prefixed = true
		s = s[1:]
	}
	s = strings.TrimPrefix(s, "'")
	s = strings.TrimSuffix(s, "'")

	var acc int64
	n := 0
	for len(s) > 0 {
		var c int64
		c, s = decodeChar(s)
		acc = acc<<8 | (c & 0xff)
		if n == 0 && (prefixed || len(s) == 0) {
			acc = c
		}
		n++
	}
	// A plain single-character literal has type char, which is signed on the
	// targets vcx supports, so the high half is negative.
	if n == 1 && !prefixed && acc > 127 {
		acc = int64(int8(acc))
	}
	return value{u: uint64(acc)}
}

func decodeChar(s string) (int64, string) {
	if s[0] != '\\' {
		return int64(s[0]), s[1:]
	}
	s = s[1:]
	if s == "" {
		return '\\', ""
	}
	switch s[0] {
	case 'n':
		return '\n', s[1:]
	case 't':
		return '\t', s[1:]
	case 'r':
		return '\r', s[1:]
	case '0', '1', '2', '3', '4', '5', '6', '7':
		v, i := int64(0), 0
		for i < 3 && i < len(s) && s[i] >= '0' && s[i] <= '7' {
			v = v*8 + int64(s[i]-'0')
			i++
		}
		return v, s[i:]
	case 'x':
		v, i := int64(0), 1
		if i < len(s) && s[i] == '{' { // C++23's delimited form
			i++
			for i < len(s) && s[i] != '}' {
				v = v*16 + int64(digit(s[i]))
				i++
			}
			if i < len(s) {
				i++
			}
			return v, s[i:]
		}
		for i < len(s) && digit(s[i]) >= 0 && digit(s[i]) < 16 {
			v = v*16 + int64(digit(s[i]))
			i++
		}
		return v, s[i:]
	case 'a':
		return 7, s[1:]
	case 'b':
		return 8, s[1:]
	case 'f':
		return 12, s[1:]
	case 'v':
		return 11, s[1:]
	}
	return int64(s[0]), s[1:]
}
