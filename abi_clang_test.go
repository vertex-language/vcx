package vcx_test

// Test Itanium ABI layouts and vtables against clang++ dumps.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// clangLayouts asks clang for the layout of every named class.
func clangLayouts(t *testing.T, clang, path string, names []string) map[string]layout {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(clang, "-std=c++23", "-w", "-fsyntax-only",
		"-Xclang", "-fdump-record-layouts-complete", abs).CombinedOutput()
	if err != nil {
		t.Fatalf("clang++ refused a file the ABI corpus expects it to accept:\n%s", out)
	}
	records := parseClangRecords(string(out))

	var wrap strings.Builder
	wrap.WriteString("#include \"" + abs + "\"\n")
	for _, n := range names {
		r, ok := records[n]
		if !ok || !r.polymorphic || r.union {
			continue
		}
		v := "__vcx_" + n
		wrap.WriteString("struct " + v + " : " + n + " { " + v + "(); }; " + v + "::" + v + "() {}\n")
	}
	dir := t.TempDir()
	wrapPath := filepath.Join(dir, "wrap.cpp")
	if err := os.WriteFile(wrapPath, []byte(wrap.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = exec.Command(clang, "-std=c++23", "-w", "-c", "-o", filepath.Join(dir, "wrap.o"),
		"-Xclang", "-fdump-vtable-layouts", wrapPath).CombinedOutput()
	if err != nil {
		t.Fatalf("clang++ refused the file that makes it emit the tables:\n%s\n%s", out, wrap.String())
	}
	groups := parseClangVTables(string(out))

	result := map[string]layout{}
	for _, n := range names {
		r, ok := records[n]
		if !ok {
			continue
		}
		l := r.layout
		g, ok := groups[n]
		if !ok {
			// If a class lacks a key function, use the derived wrapper's vtable group.
			if dg, found := groups["__vcx_"+n]; found {
				g, ok = renameGroup(dg, "__vcx_"+n, n), true
			}
		}
		if ok {
			for _, tb := range g {
				l.vtables = append(l.vtables, tb.slots)
				l.vtableAt = append(l.vtableAt, tb.at)
				if len(tb.vbases) > 0 {
					l.vbtables = append(l.vbtables, tb.vbases)
				}
			}
		} else if r.polymorphic {
			t.Errorf("%s: clang++ printed no vtable group for a polymorphic class", n)
		}
		if l.vtableAt == nil {
			l.vtableAt = []int64{}
		}
		result[n] = l
	}
	return result
}

type clangRecord struct {
	layout
	polymorphic bool
	union       bool
}

// clangRecordLine matches offset, bitfield range, and content in a record dump.
var clangRecordLine = regexp.MustCompile(`^\s*(?:(\d+)(?::(\d+)-\d+)?)?\s*\|( *)(.*)$`)

var clangRecordHeader = regexp.MustCompile(`^(struct|class|union) (.+?)(?: \(empty\))?$`)

var clangSizeof = regexp.MustCompile(`\[sizeof=(\d+),`)

// parseClangRecords reads record layouts from a clang AST dump.
func parseClangRecords(text string) map[string]clangRecord {
	out := map[string]clangRecord{}
	blocks := strings.Split(text, "*** Dumping AST Record Layout")
	for _, blk := range blocks[1:] {
		var r clangRecord
		var name string
		skipBelow := -1
		for _, line := range strings.Split(blk, "\n") {
			if m := clangSizeof.FindStringSubmatch(line); m != nil {
				r.size, _ = strconv.ParseInt(m[1], 10, 64)
				continue
			}
			m := clangRecordLine.FindStringSubmatch(line)
			if m == nil || strings.TrimSpace(m[4]) == "" {
				continue
			}
			depth := len(m[3])
			content := strings.TrimSpace(m[4])
			if name == "" {
				h := clangRecordHeader.FindStringSubmatch(content)
				if h == nil {
					break
				}
				name, r.union = h[2], h[1] == "union"
				r.offsets, r.bits = map[string]int64{}, map[string]int64{}
				continue
			}
			if skipBelow >= 0 {
				if depth > skipBelow {
					continue
				}
				skipBelow = -1
			}
			if strings.HasSuffix(content, "vtable pointer)") {
				r.polymorphic = true
				continue
			}
			if strings.Contains(content, "base)") {
				continue // the base's members follow, nested
			}
			off, _ := strconv.ParseInt(m[1], 10, 64)
			member, ok := trailingIdent(content)
			if !ok {
				continue
			}
			r.offsets[member] = off
			if m[2] != "" {
				bit, _ := strconv.ParseInt(m[2], 10, 64)
				r.bits[member] = off*8 + bit
			}
			if strings.HasPrefix(content, "struct ") || strings.HasPrefix(content, "class ") ||
				strings.HasPrefix(content, "union ") {
				skipBelow = depth
			}
		}
		if name != "" {
			if _, dup := out[name]; !dup {
				out[name] = r
			}
		}
	}
	return out
}

type clangTable struct {
	at     int64
	vbases []int64
	slots  []string
}

var (
	clangVTableHeader = regexp.MustCompile(`^Vtable for '(.+)' \(\d+ entries\)\.$`)
	clangVTableEntry  = regexp.MustCompile(`^\s*\d+ \| (.*)$`)
	clangAddressPoint = regexp.MustCompile(`^\s*-- \((.+), (-?\d+)\) vtable address --$`)
	clangOffsetEntry  = regexp.MustCompile(`^(offset_to_top|vbase_offset|vcall_offset) \((-?\d+)\)$`)
	clangDtorEntry    = regexp.MustCompile(`^(.+)::~.*\(\) \[(complete|deleting)\]`)
)

// parseClangVTables reads vtable layout blocks into tables.
func parseClangVTables(text string) map[string][]clangTable {
	out := map[string][]clangTable{}
	var class string
	var tables []clangTable
	var vbases []int64
	flush := func() {
		if class != "" {
			if _, dup := out[class]; !dup {
				out[class] = tables
			}
		}
		class, tables, vbases = "", nil, nil
	}
	for _, line := range strings.Split(text, "\n") {
		if m := clangVTableHeader.FindStringSubmatch(line); m != nil {
			flush()
			class = m[1]
			continue
		}
		if class == "" {
			continue
		}
		if m := clangAddressPoint.FindStringSubmatch(line); m != nil {
			if n := len(tables); n > 0 && tables[n-1].slots == nil && tables[n-1].at < 0 {
				tables[n-1].at, _ = strconv.ParseInt(m[2], 10, 64)
			}
			continue
		}
		m := clangVTableEntry.FindStringSubmatch(line)
		if m == nil {
			if strings.TrimSpace(line) == "" || !strings.HasPrefix(line, " ") {
				flush()
			}
			continue
		}
		entry := strings.TrimSpace(m[1])
		if o := clangOffsetEntry.FindStringSubmatch(entry); o != nil {
			switch o[1] {
			case "vbase_offset":
				n, _ := strconv.ParseInt(o[2], 10, 64)
				vbases = append(vbases, n)
			case "offset_to_top":
				tables = append(tables, clangTable{at: -1, vbases: vbases})
				vbases = nil
			}
			continue
		}
		if strings.HasSuffix(entry, " RTTI") || len(tables) == 0 {
			continue
		}
		tb := &tables[len(tables)-1]
		if tb.slots == nil {
			tb.slots = []string{}
		}
		tb.slots = append(tb.slots, clangSlot(entry))
	}
	flush()
	// clang prints vbase offsets nearest-first; the comparison reads them
	// in declaration order of the bases, which is the reverse.
	for _, g := range out {
		for i := range g {
			v := g[i].vbases
			for a, b := 0, len(v)-1; a < b; a, b = a+1, b-1 {
				v[a], v[b] = v[b], v[a]
			}
		}
	}
	return out
}

// clangSlot is one function entry as `Definer::name`, the spelling vcx's
// slots are compared in.
func clangSlot(entry string) string {
	if d := clangDtorEntry.FindStringSubmatch(entry); d != nil {
		s := qualifiedTail(d[1]) + "::{dtor}"
		if d[2] == "deleting" {
			s += "[deleting]"
		}
		return s
	}
	entry = strings.TrimSuffix(entry, " [pure]")
	pre := entry
	if i := strings.Index(entry, "operator()"); i >= 0 {
		pre = entry[:i+len("operator()")]
	} else if i := strings.IndexByte(entry, '('); i >= 0 {
		pre = entry[:i]
	}
	k := strings.LastIndex(pre, "::")
	if k < 0 {
		return pre
	}
	return qualifiedTail(pre[:k]) + "::" + pre[k+2:]
}

// qualifiedTail is the class name that ends s, with the return type and
// any pointer or reference spelling in front of it dropped. A space inside
// template brackets is part of the name.
func qualifiedTail(s string) string {
	depth := 0
	for i := len(s) - 1; i >= 0; i-- {
		switch s[i] {
		case '>':
			depth++
		case '<':
			depth--
		case ' ', '*', '&':
			if depth == 0 {
				return s[i+1:]
			}
		}
	}
	return s
}

func renameGroup(g []clangTable, from, to string) []clangTable {
	out := make([]clangTable, len(g))
	for i, tb := range g {
		out[i] = tb
		out[i].slots = make([]string, len(tb.slots))
		for j, s := range tb.slots {
			if strings.HasPrefix(s, from+"::") {
				s = to + s[len(from):]
			}
			out[i].slots[j] = s
		}
	}
	return out
}
