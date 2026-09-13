package scanner

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/vertex-language/vcx/token"
)

// ---- identifiers ----

// identPart consumes one identifier character (ASCII, UCN, or UTF-8 sequence).
func (s *scanner) identPart() bool {
	if s.off >= len(s.text) {
		return false
	}
	switch c := s.text[s.off]; {
	case isIdentPart(c):
		s.off++
		return true
	case c == '\\' && (s.peek(1) == 'u' || s.peek(1) == 'U'):
		s.scanUCN()
		return true
	case c >= 0x80:
		r, n := utf8.DecodeRune(s.text[s.off:])
		if r == utf8.RuneError && n <= 1 {
			s.errTok(s.off, s.off+1, "invalid UTF-8 byte in source")
			s.off++ // always advance: the caller loops on this
			return true
		}
		s.off += n
		return true
	}
	return false
}

// scanIdent consumes an identifier, universal character names included
// (undecoded, per invariant — the span keeps the \uXXXX spelling).
// Keyword lookup is by exact spelling.
func (s *scanner) scanIdent(start int) {
	if s.text[s.off] == '\\' && s.peek(1) != 'u' && s.peek(1) != 'U' {
		s.off++
		s.other(start, `stray '\' in program`)
		return
	}
	for s.identPart() {
	}
	spelling := string(s.text[start:s.off])
	k := token.Lookup(spelling, s.std)
	var fl token.Flags
	if token.IsAltToken(spelling) {
		fl = token.FlagAltToken
	}
	s.emitFlags(k, start, fl)
}

// scanUCN consumes \uXXXX or \UXXXXXXXX starting at the backslash. A
// malformed one is one diagnostic; scanning continues. This is a
// formation error, not a value error: it reports in every mode.
func (s *scanner) scanUCN() {
	start := s.off
	need := 4
	if s.text[s.off+1] == 'U' {
		need = 8
	}
	s.off += 2
	got := 0
	for got < need && s.off < len(s.text) && isHexDigit(s.text[s.off]) {
		s.off++
		got++
	}
	if got < need {
		s.errTok(start, s.off, fmt.Sprintf(
			"malformed universal character name (need %d hexadecimal digits, have %d)", need, got))
	}
}

// ---- literal prefixes ----

// literalPrefixes lists encoding prefixes for string and character literals.
var literalPrefixes = []struct {
	lit string
	raw bool
}{
	{`u8R"`, true},
	{`u8"`, false}, {`u8'`, false},
	{`uR"`, true}, {`UR"`, true}, {`LR"`, true},
	{`R"`, true},
	{`u"`, false}, {`U"`, false}, {`L"`, false},
	{`u'`, false}, {`U'`, false}, {`L'`, false},
}

// scanIdentOrLiteralPrefix handles the four bytes that may open an
// encoding prefix — u, U, L, R — and falls through to an identifier
// when none of them does.
func (s *scanner) scanIdentOrLiteralPrefix() {
	start := s.off
	for _, p := range literalPrefixes {
		if !s.has(p.lit) {
			continue
		}
		s.off += len(p.lit) - 1 // leave s.off on the quote
		switch {
		case p.raw:
			s.scanRawString(start)
		case p.lit[len(p.lit)-1] == '\'':
			s.scanChar(start)
		default:
			s.scanString(start)
		}
		return
	}
	s.scanIdent(start)
}

// ---- raw strings ----

// isRawDelimChar reports whether c may appear in a raw string
// delimiter: a basic source character other than space, parentheses,
// backslash, and the control characters.
func isRawDelimChar(c byte) bool {
	return c > ' ' && c < 0x7f && c != '(' && c != ')' && c != '\\'
}

// scanRawString scans an R"delim(...)delim" raw string literal.
func (s *scanner) scanRawString(start int) {
	src := s.f.Source()
	i := s.f.RawOffset(s.off) + 1 // past the opening quote

	dbeg := i
	for i < len(src) && isRawDelimChar(src[i]) {
		i++
	}
	delim := src[dbeg:i]

	switch {
	case len(delim) > 16:
		s.off = s.f.TransOffset(i)
		s.errTok(start, s.off, fmt.Sprintf(
			"raw string delimiter is %d characters, at most 16 allowed", len(delim)))
		s.emitFlags(token.STRING_LIT, start, token.FlagRaw)
		return
	case i >= len(src) || src[i] != '(':
		s.off = s.f.TransOffset(i + 1)
		s.errTok(start, s.off, "invalid character in raw string delimiter")
		s.emitFlags(token.STRING_LIT, start, token.FlagRaw)
		return
	}
	i++ // past '('

	closer := append(append([]byte(")"), delim...), '"')
	if j := bytes.Index(src[i:], closer); j >= 0 {
		s.off = s.f.TransOffset(i + j + len(closer))
	} else {
		s.off = len(s.text)
		s.errTok(start, s.off, "unterminated raw string literal")
		s.emitFlags(token.STRING_LIT, start, token.FlagRaw)
		return
	}

	fl := token.FlagRaw
	if s.scanUDSuffix() {
		fl |= token.FlagUserDefined
	}
	s.emitFlags(token.STRING_LIT, start, fl)
}

// ---- numbers ----

// scanNumber consumes one pp-number ([lex.ppnumber]) and classifies it
// as an integer or floating-point literal, handling digit separators and ud-suffixes.
func (s *scanner) scanNumber() {
	start := s.off
loop:
	for s.off < len(s.text) {
		switch c := s.text[s.off]; {
		case isDigit(c) || c == '.':
			s.off++
		case c == 'e' || c == 'E' || c == 'p' || c == 'P':
			s.off++
			if s.off < len(s.text) && (s.text[s.off] == '+' || s.text[s.off] == '-') {
				s.off++
			}
		case c == '\'' && (isDigit(s.peek(1)) || isIdentStart(s.peek(1))):
			s.off++ // the separator; its digit is taken next round
		default:
			if !s.identPart() {
				break loop
			}
		}
	}
	k, fl := s.classify(start)
	s.emitFlags(k, start, fl)
}

// classify enforces the literal grammar of [lex.icon] and [lex.fcon]
// over the consumed run and picks INT_LIT or FLOAT_LIT.
func (s *scanner) classify(start int) (token.Kind, token.Flags) {
	t := s.text[start:s.off]
	fail := func(msg string) { s.valueErr(start, s.off, msg) }

	// A run with two or more dots is a pp-number (e.g. version numbers) but not a valid scalar literal.
	if dots(t) >= 2 {
		return token.FLOAT_LIT, 0
	}

	i := 0
	// digits consumes a run of base digits with ' separators between
	// them, and returns how many digits it saw. A separator that is not
	// between two digits ends the run and lands in the suffix, where it
	// is reported as itself.
	digits := func(pred func(byte) bool) int {
		n := 0
		for i < len(t) {
			switch {
			case pred(t[i]):
				i++
				n++
			case t[i] == '\'' && n > 0 && i+1 < len(t) && pred(t[i+1]):
				i += 2
				n++
			default:
				return n
			}
		}
		return n
	}

	hex := len(t) > 1 && t[0] == '0' && (t[1] == 'x' || t[1] == 'X')
	bin := len(t) > 1 && t[0] == '0' && (t[1] == 'b' || t[1] == 'B')
	isFloat := false

	switch {
	case bin:
		i = 2
		if digits(isBinDigit) == 0 {
			fail("binary literal requires at least one digit")
			return token.INT_LIT, 0
		}
	case hex:
		i = 2
		n := digits(isHexDigit)
		if i < len(t) && t[i] == '.' {
			i++
			isFloat = true
			n += digits(isHexDigit)
		}
		if n == 0 {
			fail("hexadecimal literal requires at least one digit")
			return token.INT_LIT, 0
		}
		if i < len(t) && (t[i] == 'p' || t[i] == 'P') {
			isFloat = true
			i++
			if i < len(t) && (t[i] == '+' || t[i] == '-') {
				i++
			}
			if digits(isDigit) == 0 { // binary exponents take decimal digits
				fail("binary exponent requires decimal digits")
				return token.FLOAT_LIT, 0
			}
		} else if isFloat {
			fail("hexadecimal floating-point literal requires a binary exponent")
			return token.FLOAT_LIT, 0
		}
	default:
		digits(isDigit)
		if i < len(t) && t[i] == '.' {
			i++
			isFloat = true
			digits(isDigit)
		}
		if i < len(t) && (t[i] == 'e' || t[i] == 'E') {
			i++
			isFloat = true
			if i < len(t) && (t[i] == '+' || t[i] == '-') {
				i++
			}
			if digits(isDigit) == 0 {
				fail("exponent requires digits")
				return token.FLOAT_LIT, 0
			}
		}
		if !isFloat && t[0] == '0' {
			for j := 1; j < i; j++ {
				if t[j] != '\'' && t[j] > '7' {
					fail("invalid digit in octal literal") // 0779: one run, one report
					return token.INT_LIT, 0
				}
			}
		}
	}

	kind := token.INT_LIT
	if isFloat {
		kind = token.FLOAT_LIT
	}

	suffix := t[i:]
	switch {
	case len(suffix) == 0:
		return kind, 0
	case suffix[0] == '\'':
		fail("digit separator must appear between two digits")
		return kind, 0
	case suffix[0] == '+' || suffix[0] == '-':
		fail(fmt.Sprintf("invalid suffix %q on numeric literal "+
			"(e, E, p and P take a following sign into the number — put a space before it)",
			string(suffix)))
		return kind, 0
	case isFloat && isFloatSuffix(suffix):
		return kind, 0
	case !isFloat && isIntSuffix(suffix):
		return kind, 0
	case isUDSuffix(suffix):
		// Any identifier is a ud-suffix ([lex.ext]). Whether a literal
		// operator with that name exists is not a lexical question.
		return kind, token.FlagUserDefined
	}
	fail(fmt.Sprintf("invalid suffix %q on numeric literal", string(suffix)))
	return kind, 0
}

// isIntSuffix accepts at most one unsigned part and one size part, in
// either order: u, l, L, ll, LL, and C++23's z and Z ([lex.icon]).
// ull, LLU and uz pass; lul and lL do not.
//
// isIntSuffix accepts standard integer suffixes and MSVC sized extensions (e.g. ui64).
func isIntSuffix(t []byte) bool {
	if msvcSizedSuffix(t) {
		return true
	}
	var u, size bool
	for i := 0; i < len(t); {
		switch {
		case (t[i] == 'u' || t[i] == 'U') && !u:
			u = true
			i++
		case (t[i] == 'l' || t[i] == 'L') && !size:
			size = true
			if i+1 < len(t) && t[i+1] == t[i] {
				i++
			}
			i++
		case (t[i] == 'z' || t[i] == 'Z') && !size:
			size = true
			i++
		default:
			return false
		}
	}
	return true
}

// msvcSizedSuffix reports whether t is one of MSVC's sized integer
// suffixes: i8, i16, i32, i64, with an optional leading u or U.
func msvcSizedSuffix(t []byte) bool {
	i := 0
	if i < len(t) && (t[i] == 'u' || t[i] == 'U') {
		i++
	}
	if i >= len(t) || (t[i] != 'i' && t[i] != 'I') {
		return false
	}
	switch string(t[i+1:]) {
	case "8", "16", "32", "64":
		return true
	}
	return false
}

// isFloatSuffix accepts f, F, l, L and C++23's extended
// floating-point suffixes ([lex.fcon], P1467).
func isFloatSuffix(t []byte) bool {
	switch string(t) {
	case "f", "F", "l", "L",
		"f16", "F16", "f32", "F32", "f64", "F64", "f128", "F128", "bf16", "BF16":
		return true
	}
	return false
}

// isUDSuffix reports whether t is an identifier, which is all a
// ud-suffix has to be.
func isUDSuffix(t []byte) bool {
	if len(t) == 0 || !(isIdentStart(t[0]) || t[0] >= 0x80 || t[0] == '\\') {
		return false
	}
	for _, c := range t {
		if !(isIdentPart(c) || c >= 0x80 || c == '\\') {
			return false
		}
	}
	return true
}

// dots counts the '.' characters in a numeric run.
func dots(t []byte) int {
	n := 0
	for _, c := range t {
		if c == '.' {
			n++
		}
	}
	return n
}

// ---- character and string literals ----

// scanUDSuffix consumes a ud-suffix if one is adjacent, and reports
// whether it did. "abc"_s is one token; "abc" _s is two.
func (s *scanner) scanUDSuffix() bool {
	if s.off >= len(s.text) {
		return false
	}
	c := s.text[s.off]
	if !(isIdentStart(c) || c >= 0x80 || (c == '\\' && (s.peek(1) == 'u' || s.peek(1) == 'U'))) {
		return false
	}
	for s.identPart() {
	}
	return true
}

// scanChar scans a character literal.
func (s *scanner) scanChar(start int) {
	terminated, n := s.scanQuoted('\'')
	switch {
	case !terminated:
		s.errTok(start, s.off, "unterminated character literal")
	case n == 0:
		s.valueErr(start, s.off, "empty character literal")
	}
	var fl token.Flags
	if terminated && s.scanUDSuffix() {
		fl = token.FlagUserDefined
	}
	s.emitFlags(token.CHAR_LIT, start, fl)
}

// scanString scans one string literal. Adjacent literals are not
// concatenated — that is phase 6, above this package.
func (s *scanner) scanString(start int) {
	terminated, _ := s.scanQuoted('"')
	if !terminated {
		s.errTok(start, s.off, "unterminated string literal")
	}
	var fl token.Flags
	if terminated && s.scanUDSuffix() {
		fl = token.FlagUserDefined
	}
	s.emitFlags(token.STRING_LIT, start, fl)
}

// scanQuoted consumes characters from the opening quote through the closing quote.
func (s *scanner) scanQuoted(quote byte) (terminated bool, n int) {
	s.off++ // opening quote
	for s.off < len(s.text) {
		switch c := s.text[s.off]; c {
		case quote:
			s.off++
			return true, n
		case '\n', '\r':
			return false, n
		case '\\':
			s.scanEscape()
			n++
		default:
			s.off++
			n++
		}
	}
	return false, n
}

// scanEscape consumes one escape sequence.
func (s *scanner) scanEscape() {
	start := s.off
	s.off++ // backslash
	if s.off >= len(s.text) {
		return
	}
	c := s.text[s.off]
	switch {
	case c == '\n' || c == '\r':
		return // unterminated literal; the caller reports

	case strings.IndexByte(`'"?\abfnrtv`, c) >= 0:
		s.off++

	case isOctDigit(c):
		s.off++
		for n := 1; n < 3 && s.off < len(s.text) && isOctDigit(s.text[s.off]); n++ {
			s.off++
		}

	case c == 'o': // no undelimited form exists
		s.off++
		s.delimited(start, `\o`, isOctDigit, "octal")

	case c == 'x':
		s.off++
		if s.off < len(s.text) && s.text[s.off] == '{' {
			s.delimited(start, `\x`, isHexDigit, "hexadecimal")
			return
		}
		n := 0
		for s.off < len(s.text) && isHexDigit(s.text[s.off]) {
			s.off++
			n++
		}
		if n == 0 {
			s.valueErr(start, s.off, `\x escape requires hexadecimal digits`)
		}

	case c == 'u' && s.peek(1) == '{':
		s.off++
		s.delimited(start, `\u`, isHexDigit, "hexadecimal")

	case c == 'u' || c == 'U':
		s.off = start
		s.scanUCN()

	case c == 'N':
		s.off++
		s.named(start)

	default:
		s.off++
		s.valueErr(start, s.off, fmt.Sprintf("unknown escape sequence '\\%c'", c))
	}
}

// delimited consumes {digits} for one of the C++23 braced escapes.
func (s *scanner) delimited(start int, what string, pred func(byte) bool, digits string) {
	if s.off >= len(s.text) || s.text[s.off] != '{' {
		s.valueErr(start, s.off, what+" escape requires a braced sequence")
		return
	}
	s.off++
	n := 0
	for s.off < len(s.text) && pred(s.text[s.off]) {
		s.off++
		n++
	}
	if s.off >= len(s.text) || s.text[s.off] != '}' {
		s.valueErr(start, s.off, "unterminated "+what+" escape")
		return
	}
	s.off++
	if n == 0 {
		s.valueErr(start, s.off, what+" escape requires "+digits+" digits")
	}
}

// named consumes {CHARACTER NAME} for \N.
func (s *scanner) named(start int) {
	if s.off >= len(s.text) || s.text[s.off] != '{' {
		s.valueErr(start, s.off, `\N escape requires a braced character name`)
		return
	}
	s.off++
	n := 0
	for s.off < len(s.text) {
		switch c := s.text[s.off]; c {
		case '}', '\n', '\r', '"', '\'':
			if c != '}' {
				s.valueErr(start, s.off, `unterminated \N escape`)
				return
			}
			s.off++
			if n == 0 {
				s.valueErr(start, s.off, `\N escape requires a character name`)
			}
			return
		default:
			s.off++
			n++
		}
	}
	s.valueErr(start, s.off, `unterminated \N escape`)
}
