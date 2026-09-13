package vcx_test

// Corpus test file selection per target and dialect.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	vcx "github.com/vertex-language/vcx"
)

// dialectDirs are the dialect subfolders a corpus may have.
var dialectDirs = map[string]bool{"msvc": true, "gnu": true}

// corpusEntries returns test files or directories for the given target and dialect.
func corpusEntries(t *testing.T, corpus, target string, dirs bool) (paths, names []string) {
	t.Helper()
	tgt, err := vcx.TargetByName(target)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("tests", corpus)
	collect := func(dir, prefix string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, e := range entries {
			switch {
			case dirs && e.IsDir() && !dialectDirs[e.Name()]:
				paths = append(paths, filepath.Join(dir, e.Name()))
				names = append(names, prefix+e.Name())
			case !dirs && !e.IsDir() && strings.HasSuffix(e.Name(), ".cpp"):
				paths = append(paths, filepath.Join(dir, e.Name()))
				names = append(names, prefix+strings.TrimSuffix(e.Name(), ".cpp"))
			}
		}
	}
	collect(root, "")
	d := tgt.Dialect.String()
	collect(filepath.Join(root, d), d+"/")
	if len(paths) == 0 {
		t.Fatalf("no files found in %s for %s", root, target)
	}
	return paths, names
}
