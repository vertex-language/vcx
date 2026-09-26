package preprocessor

import (
	"io/fs"
	"reflect"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/vertex-language/vcx/token"
)

// countingFS counts the opens of each name, found or not.
type countingFS struct {
	fsys  fs.FS
	mu    sync.Mutex
	opens map[string]int
}

func (c *countingFS) Open(name string) (fs.File, error) {
	c.mu.Lock()
	c.opens[name]++
	c.mu.Unlock()
	return c.fsys.Open(name)
}

func TestCacheSharesReadsNotState(t *testing.T) {
	upper := &countingFS{fsys: fstest.MapFS{}, opens: map[string]int{}}
	lower := &countingFS{fsys: fstest.MapFS{
		"guard.h": &fstest.MapFile{Data: []byte("#ifndef GUARD_H\n#define GUARD_H\nint g;\n#endif\n")},
		"once.h":  &fstest.MapFile{Data: []byte("#pragma once\nint o;\n")},
		// A backslash-newline at the end of the file is a scanning
		// diagnostic: each unit that reads the header reports it.
		"splice.h": &fstest.MapFile{Data: []byte("int s;\\\n")},
	}, opens: map[string]int{}}
	units := []string{
		"#include <guard.h>\n#include <guard.h>\n#include <once.h>\n#include <splice.h>\n",
		// Another unit's guard was defined: this one's is not, so it reads
		// the header even though the first unit skipped it the second time.
		"#include <once.h>\n#include <guard.h>\n",
		"#define GUARD_H\n#include <guard.h>\n#include <splice.h>\n",
	}
	cfg := func(c *Cache) Config {
		return Config{Cache: c, Search: []Mount{{Name: "up", FS: upper}, {Name: "low", FS: lower}}}
	}

	type result struct {
		out   string
		diags []string
	}
	runAll := func(c *Cache) []result {
		var rs []result
		for _, src := range units {
			_, out, diags := runCfg(cfg(c), src)
			rs = append(rs, result{out, diagStrings(diags)})
		}
		return rs
	}
	want := runAll(nil)
	for _, fsys := range []*countingFS{upper, lower} {
		clear(fsys.opens)
	}

	got := runAll(NewCache())
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("with a cache:\n got %q\nwant %q", got, want)
	}
	if len(want[0].diags) == 0 || len(want[2].diags) == 0 {
		t.Errorf("splice.h's diagnostic is missing from a unit: %q", want)
	}
	for _, fsys := range []*countingFS{upper, lower} {
		for name, n := range fsys.opens {
			if n != 1 {
				t.Errorf("%s opened %d times; want 1", name, n)
			}
		}
	}
	if upper.opens["guard.h"] != 1 {
		t.Errorf("a failed lookup is not recorded: %v", upper.opens)
	}
}

func TestCacheConcurrentUnits(t *testing.T) {
	fsys := fstest.MapFS{}
	src := ""
	for _, n := range []string{"a.h", "b.h", "c.h", "d.h"} {
		fsys[n] = &fstest.MapFile{Data: []byte("#pragma once\nint " + n[:1] + ";\n")}
		src += "#include <" + n + ">\n"
	}
	shared := &countingFS{fsys: fsys, opens: map[string]int{}}
	cache := NewCache()
	_, want, _ := runCfg(Config{Search: []Mount{{Name: "inc", FS: fsys}}}, src)

	var wg sync.WaitGroup
	outs := make([]string, 16)
	for i := range outs {
		wg.Go(func() {
			p := New(Config{Cache: cache, Search: []Mount{{Name: "inc", FS: shared}}})
			toks, _ := p.Run(token.NewFile("main.cpp", []byte(src)))
			outs[i] = norm(toks)
		})
	}
	wg.Wait()
	for i, out := range outs {
		if out != want {
			t.Errorf("unit %d: got %q, want %q", i, out, want)
		}
	}
	for name, n := range shared.opens {
		if n != 1 {
			t.Errorf("%s opened %d times; want 1", name, n)
		}
	}
}

// A filesystem that cannot key a map is read directly, not shared.
func TestCacheUncomparableFS(t *testing.T) {
	type wrapped struct{ fs.FS }
	fsys := wrapped{fstest.MapFS{"h.h": &fstest.MapFile{Data: []byte("int h;\n")}}}
	_, out, _ := runCfg(Config{Cache: NewCache(), Search: []Mount{{Name: "inc", FS: fsys}}}, "#include <h.h>\n")
	if out != "int h ;" {
		t.Errorf("got %q", out)
	}
}
