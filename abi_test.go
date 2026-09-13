package vcx_test

// Test ABI layouts and vtables against cl.exe (/d1reportSingleClassLayout) and clang++.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	vcx "github.com/vertex-language/vcx"
	"github.com/vertex-language/vcx/types"
)

// layout is one class's size and member offsets.
type layout struct {
	size    int64
	offsets map[string]int64

	// vtables holds virtual table function slots in slot order (Definer::name).
	vtables [][]string

	// vbtables holds virtual base offsets in table order.
	vbtables [][]int64

	// vtableAt is where in the object each table's pointer sits.
	vtableAt []int64

	// bits is where each named bit-field's first bit sits.
	bits map[string]int64
}

// abiOracle is the platform compiler's own account of a file's layouts,
// one per class it was asked about.
type abiOracle struct {
	name    string
	target  string
	layouts func(t *testing.T, path string, names []string) map[string]layout
}

func findABIOracle(t *testing.T) (abiOracle, bool) {
	t.Helper()
	switch runtime.GOOS {
	case "windows":
		cl, ok := findMSVC(t)
		if !ok {
			t.Skip("no cl.exe found; there is no oracle to diff the layouts against")
		}
		return abiOracle{name: "cl", target: "x86_64-windows", layouts: func(t *testing.T, path string, names []string) map[string]layout {
			return msvcLayouts(t, cl, path, names)
		}}, true
	case "darwin":
		exe, err := exec.LookPath("clang++")
		if err != nil {
			t.Skip("no clang++ on PATH; there is no oracle to diff the layouts against")
		}
		return abiOracle{name: "clang++", target: vcx.DefaultTarget().Name, layouts: func(t *testing.T, path string, names []string) map[string]layout {
			return clangLayouts(t, exe, path, names)
		}}, true
	}
	t.Skip("no layout oracle wired up for " + runtime.GOOS)
	return abiOracle{}, false
}

func TestABICorpus(t *testing.T) {
	oracle, _ := findABIOracle(t)
	cl := oracle.name

	files, names := corpusEntries(t, "abi", oracle.target, false)
	for i, file := range files {
		t.Run(names[i], func(t *testing.T) {
			ours := vcxLayouts(t, file, oracle.target)
			if len(ours) == 0 {
				t.Fatal("vcx computed no layouts for this file")
			}

			names := make([]string, 0, len(ours))
			for n := range ours {
				names = append(names, n)
			}
			sort.Strings(names)

			theirs := oracle.layouts(t, file, names)
			for _, n := range names {
				want, asked := theirs[n]
				if !asked {
					t.Errorf("%s: %s reported no layout, so there is nothing to compare against", n, cl)
					continue
				}
				got := ours[n]
				if got.size != want.size {
					t.Errorf("%s: vcx says size %d, %s says %d", n, got.size, cl, want.size)
				}
				for member, wantOff := range want.offsets {
					gotOff, present := got.offsets[member]
					if !present {
						t.Errorf("%s.%s: %s places it at %d; vcx has no such member",
							n, member, cl, wantOff)
						continue
					}
					if gotOff != wantOff {
						t.Errorf("%s.%s: vcx says offset %d, %s says %d",
							n, member, gotOff, cl, wantOff)
					}
				}
				for member, gotOff := range got.offsets {
					if _, present := want.offsets[member]; !present {
						t.Errorf("%s.%s: vcx places it at %d; %s reports no such member",
							n, member, gotOff, cl)
					}
				}
				for member, wantBit := range want.bits {
					if gotBit := got.bits[member]; gotBit != wantBit {
						t.Errorf("%s.%s: vcx puts the bit-field at bit %d, %s at bit %d",
							n, member, gotBit, cl, wantBit)
					}
				}
				compareVTables(t, cl, n, got, want)
				compareVBTables(t, cl, n, got, want)
			}
		})
	}
}

// vcxLayouts asks this compiler what it computed.
func vcxLayouts(t *testing.T, path, target string) map[string]layout {
	t.Helper()
	c := &vcx.Compiler{Std: vcx.Cxx23, Target: target}
	records, diags, err := c.Layout(vcx.File(path))
	if err != nil {
		t.Fatalf("vcx: %v", err)
	}
	if vcx.HasErrors(diags) {
		var b strings.Builder
		for _, d := range diags {
			b.WriteString("\n  ")
			b.WriteString(d.Message)
		}
		t.Fatalf("vcx refused a file the ABI corpus expects it to accept:%s", b.String())
	}

	out := make(map[string]layout, len(records))
	for _, r := range records {
		l := layout{size: r.Size, offsets: map[string]int64{}, bits: map[string]int64{}}
		for _, f := range r.Fields {
			if f.Name == "" {
				continue // an unnamed bit-field is padding and has no name to compare
			}
			l.offsets[f.Name] = f.Offset
			if f.BitField {
				l.bits[f.Name] = f.Bit
				if !strings.HasSuffix(target, "-windows") {
					// clang's byte for a bit-field is the one its first
					// bit is in; cl's is where its storage unit starts.
					l.offsets[f.Name] = f.Bit / 8
				}
			}
		}
		for _, vt := range r.VTables {
			slots := make([]string, 0, len(vt.Slots))
			for _, sl := range vt.Slots {
				slots = append(slots, slotName(sl))
			}
			l.vtables = append(l.vtables, slots)
			l.vtableAt = append(l.vtableAt, vt.Offset)
			if len(vt.VBaseOffsets) > 0 {
				l.vbtables = append(l.vbtables, vt.VBaseOffsets)
			}
		}
		for _, vb := range r.VBTables {
			l.vbtables = append(l.vbtables, vb.Entries)
		}
		out[r.Name] = l
	}
	return out
}

// compareVTables checks that vtable slots match between compilers.
func compareVTables(t *testing.T, cl, class string, got, want layout) {
	t.Helper()
	if len(got.vtables) != len(want.vtables) {
		t.Errorf("%s: vcx computed %d virtual table(s), %s has %d (vcx %v, %s %v)",
			class, len(got.vtables), cl, len(want.vtables), got.vtables, cl, want.vtables)
		return
	}
	for i := range want.vtables {
		gotSlots, wantSlots := got.vtables[i], want.vtables[i]
		if want.vtableAt != nil && got.vtableAt[i] != want.vtableAt[i] {
			t.Errorf("%s %s: vcx puts its pointer at %d, %s at %d",
				class, tableName(i), got.vtableAt[i], cl, want.vtableAt[i])
		}
		if len(gotSlots) != len(wantSlots) {
			t.Errorf("%s %s: vcx has %d slots, %s has %d (vcx %v, %s %v)",
				class, tableName(i), len(gotSlots), cl, len(wantSlots), gotSlots, cl, wantSlots)
			continue
		}
		for j := range wantSlots {
			if gotSlots[j] != wantSlots[j] {
				t.Errorf("%s %s slot %d: vcx says %s, %s says %s",
					class, tableName(i), j, gotSlots[j], cl, wantSlots[j])
			}
		}
	}
}

// slotName formats a slot as Definer::name.
func slotName(sl types.VSlot) string {
	s := sl.Definer + "::" + sl.Name
	if sl.Deleting {
		s += "[deleting]"
	}
	return s
}

// compareVBTables checks that virtual base offsets match between compilers.
func compareVBTables(t *testing.T, cl, class string, got, want layout) {
	t.Helper()
	if len(got.vbtables) != len(want.vbtables) {
		t.Errorf("%s: vcx computed %d virtual base table(s), %s has %d (vcx %v, %s %v)",
			class, len(got.vbtables), cl, len(want.vbtables), got.vbtables, cl, want.vbtables)
		return
	}
	for i := range want.vbtables {
		gotE, wantE := got.vbtables[i], want.vbtables[i]
		if len(gotE) != len(wantE) {
			t.Errorf("%s vbtable %d: vcx has %d entries, %s has %d (vcx %v, %s %v)",
				class, i, len(gotE), cl, len(wantE), gotE, cl, wantE)
			continue
		}
		for j := range wantE {
			if gotE[j] != wantE[j] {
				t.Errorf("%s vbtable %d entry %d: vcx says %d, %s says %d",
					class, i, j, gotE[j], cl, wantE[j])
			}
		}
	}
}

func tableName(i int) string {
	if i == 0 {
		return "primary vftable"
	}
	return "vftable #" + strconv.Itoa(i)
}

// clHeader matches `class Padded	size(8):` and its struct and union forms.
var clHeader = regexp.MustCompile(`^(?:class|struct|union)\s+(\S+)\s+size\((\d+)\)`)

// clMember matches a member line: an offset, a run of `|` for however many
// subobjects deep it is, and the member's name. A bit-field's offset carries
// a trailing `.` and its name is followed by `(bitstart=...)`, which is why
// the name is taken as the first word.
var clMember = regexp.MustCompile(`^\s*(\d+)\.?\s*\|[\s|]*(.+?)\s*$`)

// msvcLayouts queries cl.exe for record layouts.
func msvcLayouts(t *testing.T, cl msvc, path string, names []string) map[string]layout {
	t.Helper()
	dir := t.TempDir()
	out := map[string]layout{}
	for _, name := range names {
		text, err := cl.run(dir, path, "/d1reportSingleClassLayout"+name)
		if err != nil {
			t.Fatalf("cl refused a file the ABI corpus expects it to accept:\n%s", text)
		}
		if l, ok := parseCLLayout(text, name); ok {
			out[name] = l
		}
	}
	return out
}

// parseCLLayout extracts size and member offsets from cl.exe layout output.
func parseCLLayout(text, name string) (layout, bool) {
	l := layout{offsets: map[string]int64{}}
	inClass := false
	l.vtables = parseCLVTables(text, name)
	l.vbtables = parseCLVBTables(text, name)

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")

		if m := clHeader.FindStringSubmatch(line); m != nil {
			if inClass {
				break // the next class's header ends this one
			}
			if m[1] != name {
				continue
			}
			inClass = true
			l.size, _ = strconv.ParseInt(m[2], 10, 64)
			continue
		}
		if !inClass {
			continue
		}
		if strings.Contains(line, "alignment member") ||
			strings.Contains(line, "{vfptr}") ||
			strings.Contains(line, "{vbptr}") {
			continue
		}
		if m := clMember.FindStringSubmatch(line); m != nil {
			// Extract member name from line.
			name, ok := trailingIdent(m[2])
			if !ok {
				continue
			}
			off, _ := strconv.ParseInt(m[1], 10, 64)
			l.offsets[name] = off
		}
	}
	return l, inClass
}

// trailingIdent is the member's own name on one of cl's member lines.
func trailingIdent(text string) (string, bool) {
	if i := strings.IndexByte(text, '('); i >= 0 {
		text = text[:i] // drop a bit-field's (bitstart=..,nbits=..)
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", false
	}
	name := fields[len(fields)-1]
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (i > 0 && c >= '0' && c <= '9') {
			continue
		}
		return "", false
	}
	return name, true
}

// clVTHeader matches `Multi::$vftable@P2@:` and the primary `A::$vftable@:`.
var clVTHeader = regexp.MustCompile(`^(\w+)::\$vftable@(\w*)@?:`)

// clVTSlot matches ` 0	| &B::f ` -- the slot number and the function the
// slot holds, which is the claim worth comparing.
var clVTSlot = regexp.MustCompile(`^\s*(\d+)\s*\|\s*&([\w:{}]+)\s*$`)

// parseCLVTables reads the `$vftable@` sections cl prints after a layout.
//
// The header lines it skips are the ones that are not slots: `&X_meta`
// points at the RTTI descriptor, and a bare number is the offset the table
// belongs at. Only the numbered entries are function pointers.
func parseCLVTables(text, class string) [][]string {
	var tables [][]string
	inTable := false

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")

		if m := clVTHeader.FindStringSubmatch(line); m != nil {
			inTable = m[1] == class
			if inTable {
				tables = append(tables, nil)
			}
			continue
		}
		if !inTable {
			continue
		}
		if strings.TrimSpace(line) == "" || strings.Contains(line, "this adjustor:") {
			inTable = false
			continue
		}
		if m := clVTSlot.FindStringSubmatch(line); m != nil {
			tables[len(tables)-1] = append(tables[len(tables)-1], m[2])
		}
	}
	return tables
}

// clVBHeader matches `Diamond::$vbtable@Left@:` and the plain
// `One::$vbtable@:`.
var clVBHeader = regexp.MustCompile(`^(\w+)::\$vbtable@(\w*)@?:`)

// clVBEntry matches ` 1	| 16 (Oned(One+0)B)` -- the entry number and the
// offset, which is all the table actually holds; what follows in parentheses
// is cl naming the path for a reader.
var clVBEntry = regexp.MustCompile(`^\s*(\d+)\s*\|\s*(-?\d+)`)

// parseCLVBTables reads the `$vbtable@` sections, in the order cl prints
// them, which is the order vcx computes them in: the class's own first.
func parseCLVBTables(text, class string) [][]int64 {
	var tables [][]int64
	inTable := false

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")

		if m := clVBHeader.FindStringSubmatch(line); m != nil {
			inTable = m[1] == class
			if inTable {
				tables = append(tables, nil)
			}
			continue
		}
		if !inTable {
			continue
		}
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "vbi:") {
			inTable = false
			continue
		}
		if m := clVBEntry.FindStringSubmatch(line); m != nil {
			off, _ := strconv.ParseInt(m[2], 10, 64)
			tables[len(tables)-1] = append(tables[len(tables)-1], off)
		}
	}
	return tables
}

// msvc holds paths and environment for cl.exe and link.exe.
type msvc struct {
	exe     string
	link    string
	include string
	lib     string
}

// run compiles a file with cl.exe and returns compiler output.
func (m msvc) run(dir, path string, flags ...string) (string, error) {
	args := append([]string{"/nologo", "/std:c++latest", "/utf-8", "/c", "/Fo:" + dir + `\`}, flags...)
	args = append(args, filepath.ToSlash(path))
	cmd := exec.Command(m.exe, args...)
	cmd.Env = append(os.Environ(), "INCLUDE="+m.include)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func findMSVC(t *testing.T) (msvc, bool) {
	t.Helper()

	toolsRoot := `C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Tools\MSVC`
	toolset, ok := newestDir(toolsRoot)
	if !ok {
		return msvc{}, false
	}
	bin := filepath.Join(toolsRoot, toolset, `bin\Hostx64\x64`)
	exe := filepath.Join(bin, "cl.exe")
	if _, err := os.Stat(exe); err != nil {
		return msvc{}, false
	}

	kitsRoot := `C:\Program Files (x86)\Windows Kits\10`
	kitsInclude := filepath.Join(kitsRoot, "Include")
	sdk, ok := newestDir(kitsInclude)
	if !ok {
		return msvc{}, false
	}

	include := strings.Join([]string{
		filepath.Join(toolsRoot, toolset, "include"),
		filepath.Join(kitsInclude, sdk, "ucrt"),
		filepath.Join(kitsInclude, sdk, "shared"),
		filepath.Join(kitsInclude, sdk, "um"),
	}, ";")
	// The linker needs %LIB% for the same reason cl needs %INCLUDE%, and
	// for the same reason it is set here rather than by vcvars: on this
	// machine vcvars resolves no Windows SDK at all.
	lib := strings.Join([]string{
		filepath.Join(toolsRoot, toolset, `lib\x64`),
		filepath.Join(kitsRoot, "Lib", sdk, `ucrt\x64`),
		filepath.Join(kitsRoot, "Lib", sdk, `um\x64`),
	}, ";")

	return msvc{
		exe:     exe,
		link:    filepath.Join(bin, "link.exe"),
		include: include,
		lib:     lib,
	}, true
}

// newestDir is the last entry of a versioned directory, which is how both
// the MSVC toolsets and the Windows Kits are laid out.
func newestDir(root string) (string, bool) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", false
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 0 {
		return "", false
	}
	sort.Strings(dirs)
	return dirs[len(dirs)-1], true
}
