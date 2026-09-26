package vcx

import (
	"bytes"
	"path/filepath"
	"sync"
	"testing"
)

// TestConcurrentObjects compiles units on several goroutines at once, with
// one Compiler, and holds each object to what a compile on its own makes.
// A build compiles its native modules this way, so nothing in a compile
// may lean on package-level state. Run it under -race.
func TestConcurrentObjects(t *testing.T) {
	var files []string
	for _, n := range []string{"003", "050", "120", "150", "170", "185", "199", "209"} {
		m, _ := filepath.Glob("tests/" + n + "-*.cpp")
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Skip("no corpus")
	}
	c := &Compiler{Std: Cxx23}
	want := make([][]byte, len(files))
	for i, f := range files {
		obj, diags, err := c.Object(File(f))
		if err != nil || HasErrors(diags) {
			t.Fatalf("%s: %v %v", f, err, diags)
		}
		want[i] = obj
	}
	got := make([][]byte, len(files))
	var wg sync.WaitGroup
	for i, f := range files {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], _, _ = c.Object(File(f))
		}()
	}
	wg.Wait()
	for i, f := range files {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("%s: compiled alongside others, its object differs", f)
		}
	}
}
