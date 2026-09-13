// Package literal implements decoding and concatenation of string literals.
package literal

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/types"
)

// Encoding is a string literal's encoding prefix.
type Encoding int

const (
	Narrow Encoding = iota // "…", an array of char
	UTF8                   // u8"…", char8_t
	UTF16                  // u"…", char16_t
	UTF32                  // U"…", char32_t
	Wide                   // L"…", wchar_t
)

func (e Encoding) String() string {
	return [...]string{"", "u8", "u", "U", "L"}[e]
}

// String is a decoded string literal with code units in its declared encoding.
type String struct {
	Enc   Encoding
	Units []uint32
}

// Bytes returns the narrow byte representation.
func (s String) Bytes() []byte {
	out := make([]byte, len(s.Units))
	for i, u := range s.Units {
		out[i] = byte(u)
	}
	return out
}

// ElemKind is the element type of the decoded string.
func (s String) ElemKind() types.Kind {
	switch s.Enc {
	case UTF8:
		return types.Char8
	case UTF16:
		return types.Char16
	case UTF32:
		return types.Char32
	case Wide:
		return types.WChar
	}
	return types.Char
}

// Decode decodes and concatenates adjacent string literal segments.
func Decode(u ast.Unit, e *ast.StringLit) (String, error) {
	var s String
	seenEnc := false
	for _, seg := range e.Segs {
		raw := u.Text(seg.Lo)
		enc, rawStr, body, err := split(raw)
		if err != nil {
			return s, err
		}
		if enc != Narrow {
			if seenEnc && enc != s.Enc {
				return s, fmt.Errorf("string literal pieces with different encoding prefixes %s and %s", s.Enc, enc)
			}
			s.Enc, seenEnc = enc, true
		}
		if rawStr {
			for _, r := range body {
				s.Units = append(s.Units, encode(uint32(r), enc)...)
			}
			continue
		}
		units, err := unescape(body, enc)
		if err != nil {
			return s, err
		}
		s.Units = append(s.Units, units...)
	}
	return s, nil
}

// encode is one source character as the encoding's code units: UTF-8
// bytes for a narrow or u8 literal, a surrogate pair where UTF-16 needs
// one, and the code point itself otherwise.
func encode(r uint32, enc Encoding) []uint32 {
	switch enc {
	case Narrow, UTF8:
		if r < 0x80 {
			return []uint32{r}
		}
		var buf [4]byte
		n := utf8.EncodeRune(buf[:], rune(r))
		out := make([]uint32, n)
		for i, b := range buf[:n] {
			out[i] = uint32(b)
		}
		return out
	case UTF16:
		if r >= 0x10000 {
			r -= 0x10000
			return []uint32{0xD800 + (r >> 10), 0xDC00 + (r & 0x3FF)}
		}
	}
	return []uint32{r}
}

// split takes a literal's spelling apart: the encoding prefix, whether it
// is raw, and the body between the quotes (or the raw delimiters).
func split(raw string) (enc Encoding, isRaw bool, body string, err error) {
	i := 0
	for i < len(raw) && raw[i] != '"' {
		i++
	}
	if i >= len(raw) || raw[len(raw)-1] != '"' {
		return 0, false, "", fmt.Errorf("malformed string literal %s", raw)
	}
	prefix := raw[:i]
	if len(prefix) > 0 && prefix[len(prefix)-1] == 'R' {
		isRaw = true
		prefix = prefix[:len(prefix)-1]
	}
	switch prefix {
	case "":
		enc = Narrow
	case "u8":
		enc = UTF8
	case "u":
		enc = UTF16
	case "U":
		enc = UTF32
	case "L":
		enc = Wide
	default:
		return 0, false, "", fmt.Errorf("unknown string literal prefix %q", prefix)
	}
	body = raw[i+1 : len(raw)-1]
	if isRaw {
		// In R"delim( ... )delim", the delimiter is whatever
		// stands before the first parenthesis.
		open := 0
		for open < len(body) && body[open] != '(' {
			open++
		}
		delim := body[:open]
		tail := ")" + delim
		if open >= len(body) || len(body) < open+1+len(tail) || body[len(body)-len(tail):] != tail {
			return 0, false, "", fmt.Errorf("malformed raw string literal %s", raw)
		}
		body = body[open+1 : len(body)-len(tail)]
	}
	return enc, isRaw, body, nil
}

// unescape resolves string escape sequences. Characters
// are returned as code points; a numeric escape is the unit it names.
func unescape(body string, enc Encoding) ([]uint32, error) {
	var out []uint32
	for j := 0; j < len(body); {
		c := body[j]
		if c != '\\' {
			r, size := utf8.DecodeRuneInString(body[j:])
			if r == utf8.RuneError && size == 1 {
				// Not UTF-8: a byte, kept as one.
				out = append(out, uint32(c))
			} else {
				out = append(out, encode(uint32(r), enc)...)
			}
			j += size
			continue
		}
		j++
		if j >= len(body) {
			return nil, fmt.Errorf("string literal ends in a backslash")
		}
		esc := body[j]
		j++
		switch esc {
		case 'n':
			out = append(out, '\n')
		case 't':
			out = append(out, '\t')
		case 'r':
			out = append(out, '\r')
		case 'a':
			out = append(out, 7)
		case 'b':
			out = append(out, 8)
		case 'f':
			out = append(out, 12)
		case 'v':
			out = append(out, 11)
		case 'e':
			out = append(out, 27)
		case '\\', '\'', '"', '?':
			out = append(out, uint32(esc))
		case 'x':
			k := j
			for k < len(body) && isHexDigit(body[k]) {
				k++
			}
			if k == j {
				return nil, fmt.Errorf("\\x with no hexadecimal digits")
			}
			n, _ := strconv.ParseUint(body[j:k], 16, 32)
			out = append(out, uint32(n))
			j = k
		case 'u', 'U':
			// Universal character name: four or eight
			// hexadecimal digits naming a code point.
			width := 4
			if esc == 'U' {
				width = 8
			}
			if j+width > len(body) {
				return nil, fmt.Errorf("\\%c with fewer than %d hexadecimal digits", esc, width)
			}
			n, err := strconv.ParseUint(body[j:j+width], 16, 32)
			if err != nil {
				return nil, fmt.Errorf("\\%c with fewer than %d hexadecimal digits", esc, width)
			}
			out = append(out, encode(uint32(n), enc)...)
			j += width
		default:
			if esc >= '0' && esc <= '7' {
				k := j - 1
				for k < len(body) && k < j+2 && body[k] >= '0' && body[k] <= '7' {
					k++
				}
				n, _ := strconv.ParseUint(body[j-1:k], 8, 32)
				out = append(out, uint32(n))
				j = k
				break
			}
			return nil, fmt.Errorf("unknown escape sequence \\%c", esc)
		}
	}
	return out, nil
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}
