package vcx_test

// Test compilation and execution of standard headers against the host toolchain.

import (
	"testing"
)

func TestHeadersCorpus(t *testing.T) {
	h, why, ok := findHost(t)
	if !ok {
		t.Skip(why)
	}

	files, names := corpusEntries(t, "headers", h.target(), false)
	for i, file := range files {
		t.Run(names[i], func(t *testing.T) {
			want := runUnderHost(t, h, file)
			got := runUnderVCX(t, h, file)
			if got != want {
				t.Errorf("vcx's program exited %d, cl's exited %d", got, want)
			}
		})
	}
}
