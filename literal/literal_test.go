package literal

import (
	"testing"
)

// The spelling is what the scanner hands over: prefix, quotes and all.
func TestSplit(t *testing.T) {
	cases := []struct {
		raw   string
		enc   Encoding
		isRaw bool
		body  string
	}{
		{`"abc"`, Narrow, false, "abc"},
		{`L"abc"`, Wide, false, "abc"},
		{`u8"x"`, UTF8, false, "x"},
		{`u"x"`, UTF16, false, "x"},
		{`U"x"`, UTF32, false, "x"},
		{`R"(a\nb)"`, Narrow, true, `a\nb`},
		{`R"xy(a)"b)xy"`, Narrow, true, `a)"b`},
		{`LR"(w)"`, Wide, true, "w"},
		{`u8R"()"`, UTF8, true, ""},
	}
	for _, c := range cases {
		enc, isRaw, body, err := split(c.raw)
		if err != nil {
			t.Errorf("%s: %v", c.raw, err)
			continue
		}
		if enc != c.enc || isRaw != c.isRaw || body != c.body {
			t.Errorf("%s: got (%v, %v, %q), want (%v, %v, %q)", c.raw, enc, isRaw, body, c.enc, c.isRaw, c.body)
		}
	}
	for _, bad := range []string{`"abc`, `R"(abc)x"`, `Q"abc"`} {
		if _, _, _, err := split(bad); err == nil {
			t.Errorf("%s: accepted", bad)
		}
	}
}

// TestUnescape checks that each escape is one unit, a numeric escape is the unit it
// names, and non-ASCII chars map properly per encoding.
func TestUnescape(t *testing.T) {
	cases := []struct {
		body string
		enc  Encoding
		want []uint32
	}{
		{`a\nb`, Narrow, []uint32{'a', '\n', 'b'}},
		{`\t\r\a\b\f\v\e\\\'\"\?`, Narrow, []uint32{9, 13, 7, 8, 12, 11, 27, '\\', '\'', '"', '?'}},
		{`\x41\x4a`, Narrow, []uint32{0x41, 0x4a}},
		{`\101\0\12x`, Narrow, []uint32{0101, 0, 012, 'x'}},
		{`\xff`, Narrow, []uint32{0xff}},
		{"é", Narrow, []uint32{0xC3, 0xA9}},
		{"é", Wide, []uint32{0xE9}},
		{"é", UTF16, []uint32{0xE9}},
		{`é`, Narrow, []uint32{0xC3, 0xA9}},
		{`é`, UTF32, []uint32{0xE9}},
		{`\U0001F600`, UTF16, []uint32{0xD83D, 0xDE00}},
		{`\U0001F600`, UTF32, []uint32{0x1F600}},
		{`\U0001F600`, Narrow, []uint32{0xF0, 0x9F, 0x98, 0x80}},
	}
	for _, c := range cases {
		got, err := unescape(c.body, c.enc)
		if err != nil {
			t.Errorf("%q: %v", c.body, err)
			continue
		}
		if !equal(got, c.want) {
			t.Errorf("%q (%v): got %v, want %v", c.body, c.enc, got, c.want)
		}
	}
	for _, bad := range []string{`\q`, `\x`, `a\`, `\u12`} {
		if _, err := unescape(bad, Narrow); err == nil {
			t.Errorf("%q: accepted", bad)
		}
	}
}

func equal(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
