package vcx_test

// Test mangled symbol names against platform compiler dumps (dumpbin / nm).

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	vcx "github.com/vertex-language/vcx"
)

func TestMangleCorpus(t *testing.T) {
	if runtime.GOOS == "darwin" {
		testMangleCorpusClang(t)
		return
	}
	cl, ok := findMSVC(t)
	if !ok {
		t.Skip("no cl.exe: the mangling corpus has no oracle")
	}

	files, names := corpusEntries(t, "mangle", "x86_64-windows", false)
	for i, path := range files {
		t.Run(names[i], func(t *testing.T) {
			dir := t.TempDir()
			out, err := cl.run(dir, path)
			if err != nil {
				t.Fatalf("cl refused a corpus file:\n%s", out)
			}
			obj := filepath.Join(dir, strings.TrimSuffix(filepath.Base(path), ".cpp")+".obj")
			want, err := definedExternals(cl, obj)
			if err != nil {
				t.Fatalf("dumpbin: %v", err)
			}

			c := &vcx.Compiler{Target: "x86_64-windows", Std: vcx.Cxx23}
			syms, diags, err := c.Symbols(vcx.File(path))
			if err != nil {
				t.Fatal(err)
			}
			if vcx.HasErrors(diags) {
				var b strings.Builder
				for _, d := range diags {
					b.WriteString("  " + d.Message + "\n")
				}
				t.Fatalf("vcx refused a corpus file:\n%s", b.String())
			}

			got := map[string]string{}
			for _, s := range syms {
				got[s.Name] = s.Source
			}

			var missing []string
			for name, src := range got {
				if _, defined := want[name]; !defined {
					missing = append(missing, name+"   ("+src+")")
				}
			}
			sort.Strings(missing)
			for _, m := range missing {
				t.Errorf("vcx would define a symbol cl does not:\n  %s", m)
			}

			var unnamed []string
			for name, demangled := range want {
				if _, named := got[name]; named || !userSymbol(name) {
					continue
				}
				unnamed = append(unnamed, name+"   ("+demangled+")")
			}
			sort.Strings(unnamed)
			for _, u := range unnamed {
				t.Errorf("cl defines a symbol vcx did not name:\n  %s", u)
			}

			if len(missing) > 0 || len(unnamed) > 0 {
				var b strings.Builder
				for _, s := range syms {
					b.WriteString("  " + s.Name + "\n")
				}
				t.Logf("vcx's names:\n%s", b.String())
			}
		})
	}
}

// symbolLine is one row of `dumpbin /symbols`:
//
//	008 00000000 SECT4  notype ()    External     | ?f@@YAXXZ (void __cdecl f(void))
//
// The section number says it is defined here (UNDEF would say it is not),
// External that the linker will see it, and the parenthesis holds what
// undname makes of the name.
var symbolLine = regexp.MustCompile(`^\S+ [0-9A-F]{8} SECT\S+\s+\S+\s+(?:\(\)\s+)?External\s+\|\s+(\S+)(?: \((.*)\))?`)

// definedExternals is the set of names an object defines for the linker,
// each with cl's own reading of it.
func definedExternals(m msvc, obj string) (map[string]string, error) {
	dumpbin := filepath.Join(filepath.Dir(m.exe), "dumpbin.exe")
	cmd := exec.Command(dumpbin, "/nologo", "/symbols", obj)
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(m.exe)+";"+os.Getenv("PATH"))
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		if m := symbolLine.FindStringSubmatch(sc.Text()); m != nil {
			names[m[1]] = m[2]
		}
	}
	return names, nil
}

// userSymbol reports whether a name from cl is an expected user definition.
func userSymbol(name string) bool {
	switch {
	case !strings.HasPrefix(name, "?"):
		return !strings.HasPrefix(name, "_") && !strings.HasPrefix(name, "$") && !strings.HasPrefix(name, ".")
	case strings.HasPrefix(name, "??_7"):
		return true // a vftable
	case strings.HasPrefix(name, "??_"):
		return false // ??_C strings, ??_R RTTI, ??_G/??_E destructor thunks
	case strings.HasPrefix(name, "?$"):
		return false // a template instance
	}
	return true
}

// testMangleCorpusClang verifies mangled names against clang++ and nm.
func testMangleCorpusClang(t *testing.T) {
	clang, err := exec.LookPath("clang++")
	if err != nil {
		t.Skip("no clang++: the mangling corpus has no oracle")
	}
	target := vcx.DefaultTarget().Name
	files, names := corpusEntries(t, "mangle", target, false)
	for i, path := range files {
		name := names[i]
		t.Run(name, func(t *testing.T) {
			obj := filepath.Join(t.TempDir(), name+".o")
			if out, err := exec.Command(clang, "-std=c++23", "-w", "-c", "-o", obj, path).CombinedOutput(); err != nil {
				t.Fatalf("clang++ refused a corpus file:\n%s", out)
			}
			out, err := exec.Command("nm", "-gU", obj).Output()
			if err != nil {
				t.Fatalf("nm: %v", err)
			}
			want := map[string]bool{}
			for _, line := range strings.Split(string(out), "\n") {
				if f := strings.Fields(line); len(f) == 3 {
					want[f[2]] = true
				}
			}

			c := &vcx.Compiler{Target: target, Std: vcx.Cxx23}
			syms, diags, err := c.Symbols(vcx.File(path))
			if err != nil {
				t.Fatal(err)
			}
			if vcx.HasErrors(diags) {
				var b strings.Builder
				for _, d := range diags {
					b.WriteString("  " + d.Message + "\n")
				}
				t.Fatalf("vcx refused a corpus file:\n%s", b.String())
			}
			got := map[string]string{}
			for _, s := range syms {
				got[s.Name] = s.Source
			}

			var missing, unnamed []string
			for n, src := range got {
				if want[n] {
					continue
				}
				if base := structorVariant.ReplaceAllString(n, "${1}C2$2"); base != n && want[base] {
					continue // abstract: clang has only the base-object constructor
				}
				missing = append(missing, n+"   ("+src+")")
			}
			for n := range want {
				if _, named := got[n]; named || !itaniumUserSymbol(n) {
					continue
				}
				unnamed = append(unnamed, n)
			}
			sort.Strings(missing)
			sort.Strings(unnamed)
			for _, m := range missing {
				t.Errorf("vcx would define a symbol clang++ does not:\n  %s", m)
			}
			for _, u := range unnamed {
				t.Errorf("clang++ defines a symbol vcx did not name:\n  %s", u)
			}
		})
	}
}

// structorVariant finds a complete-object constructor code in a mangled
// name: `C1` closing a nested name or opening its template arguments.
var structorVariant = regexp.MustCompile(`([^0-9])C1([EI])`)

// itaniumUserSymbol reports whether a name clang defined is one vcx is
// expected to have named: not a structor variant or type information that
// lowering adds, and not a thunk.
func itaniumUserSymbol(name string) bool {
	switch {
	case strings.HasPrefix(name, "__ZTI"), strings.HasPrefix(name, "__ZTS"),
		strings.HasPrefix(name, "__ZTh"), strings.HasPrefix(name, "__ZGV"):
		return false
	case regexp.MustCompile(`[^0-9](C2|D2|D0)[EI]`).MatchString(name):
		return false
	}
	return true
}
