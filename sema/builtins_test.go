package sema

import "testing"

func TestRetype(t *testing.T) {
	for _, tc := range [][3]string{
		{"d(d)", "f", "f(f)"},
		{"d(d,*i)", "L", "L(L,*i)"},
		{"d(d,*d)", "f", "f(f,*f)"},
		{"d(d,L)", "f", "f(f,L)"},
		{"i(d)", "L", "i(L)"},
	} {
		if got := retype(tc[0], tc[1]); got != tc[2] {
			t.Errorf("retype(%q, %q) = %q, want %q", tc[0], tc[1], got, tc[2])
		}
	}
}
