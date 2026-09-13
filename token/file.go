package token

import (
	"bytes"
	"fmt"
	"sort"
)

// Position is a resolved source location in raw bytes (Line and Column are 1-based).
type Position struct {
	Filename string
	Offset   int
	Line     int
	Column   int
}

func (p Position) String() string {
	return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
}

// bom is the UTF-8 byte order mark (EF BB BF).
var bom = []byte{0xEF, 0xBB, 0xBF}

// File represents a source file and maps translated text positions back to raw source bytes.
type File struct {
	name string
	src  []byte // raw, as typed
	text []byte // translated, what the scanner reads

	// rawLo and rawHi map translated byte offsets back to raw byte offsets.
	rawLo, rawHi []int32

	lineStarts []int32 // raw offsets of line starts
	diags      []Diagnostic
}

// NewFile translates src through translation phases 1 and 2 and returns its File.
// Handles UTF-8 BOM removal and backslash-newline line splicing ([lex.phases]).
func NewFile(name string, src []byte) *File {
	f := &File{name: name, src: src}
	f.scanLines()

	if !bytes.HasPrefix(src, bom) && bytes.IndexByte(src, '\\') < 0 {
		f.text = src // fast path: identity mapping
		return f
	}

	spliceAtEOF := f.translate()
	f.checkSplicedUCN()
	if spliceAtEOF {
		f.report(Warn, len(f.text)-1, "backslash-newline at end of file")
	}
	return f
}

// report appends one diagnostic with a non-empty span clamped to the
// translated text. A file whose translation is empty has no span to
// carry one; nothing is reported.
func (f *File) report(sev Severity, off int, msg string) {
	n := len(f.text)
	if n == 0 {
		return
	}
	if off < 0 {
		off = 0
	}
	if off >= n {
		off = n - 1
	}
	p := f.Pos(off)
	f.diags = append(f.diags, Diagnostic{Pos: p, End: p + 1, Severity: sev, Message: msg})
}

func isHorizontalSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\v' || c == '\f'
}

// translate runs phases 1 and 2 in one pass, building text and the
// translated→raw mapping. Returns whether the file's final bytes were
// consumed by a splice.
func (f *File) translate() (spliceAtEOF bool) {
	src, n := f.src, len(f.src)
	text := make([]byte, 0, n)
	lo := make([]int32, 0, n)
	hi := make([]int32, 0, n)

	// Deferred so that they are sited in the translated space, which
	// only exists once the byte before them has been appended.
	type pending struct {
		off int
		msg string
	}
	var pad []pending

	i := 0
	if bytes.HasPrefix(src, bom) {
		i = len(bom)
	}

	for i < n {
		// Phase 2: line splicing.
		if src[i] == '\\' {
			j := i + 1
			for j < n && isHorizontalSpace(src[j]) {
				j++
			}
			if j < n && (src[j] == '\n' || src[j] == '\r') {
				if j > i+1 {
					// Whitespace between backslash and newline.
					pad = append(pad, pending{len(text) - 1,
						"backslash and newline separated by whitespace"})
				}
				if src[j] == '\r' {
					j++
					if j < n && src[j] == '\n' {
						j++
					}
				} else {
					j++
				}
				i = j
				spliceAtEOF = i == n
				continue
			}
		}

		text = append(text, src[i])
		lo = append(lo, int32(i))
		hi = append(hi, int32(i+1))
		i++
		spliceAtEOF = false
	}

	f.text, f.rawLo, f.rawHi = text, lo, hi
	for _, p := range pad {
		f.report(Warn, p.off, p.msg)
	}
	return spliceAtEOF
}

// checkSplicedUCN warns on universal-character-names created across line splices.
func (f *File) checkSplicedUCN() {
	if f.rawLo == nil {
		return
	}
	for i := 0; i+1 < len(f.text); i++ {
		if f.text[i] != '\\' {
			continue
		}
		if c := f.text[i+1]; c != 'u' && c != 'U' {
			continue
		}
		if f.rawHi[i] != f.rawLo[i+1] {
			f.report(Error, i, "universal-character-name produced by line splicing")
		}
	}
}

// scanLines records raw line starts. \n, \r\n, and lone \r all
// terminate a line.
func (f *File) scanLines() {
	starts := []int32{0}
	src := f.src
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '\n':
			starts = append(starts, int32(i+1))
		case '\r':
			if i+1 < len(src) && src[i+1] == '\n' {
				i++
			}
			starts = append(starts, int32(i+1))
		}
	}
	f.lineStarts = starts
}

// Name returns the file name given to NewFile.
func (f *File) Name() string { return f.name }

// Text returns the translated text — what the scanner reads.
func (f *File) Text() []byte { return f.text }

// Source returns the raw bytes — what the author typed.
func (f *File) Source() []byte { return f.src }

// Size is the length of the translated text. Pos(Size()) is the valid
// one-past-the-end position (the scanner's EOF token).
func (f *File) Size() int { return len(f.text) }

// Pos converts a translated-text offset in [0, Size()] to a Pos.
func (f *File) Pos(offset int) Pos {
	if offset < 0 || offset > len(f.text) {
		panic(fmt.Sprintf("token: Pos offset %d out of range [0, %d]", offset, len(f.text)))
	}
	return Pos(offset + 1)
}

// Offset converts a Pos back to a translated-text offset.
func (f *File) Offset(p Pos) int {
	if !p.IsValid() || int(p) > len(f.text)+1 {
		panic(fmt.Sprintf("token: invalid Pos %d for file %q", p, f.name))
	}
	return int(p) - 1
}

// Slice returns the translated bytes of a span — what the scanner read.
// Feed this to decoders.
func (f *File) Slice(pos, end Pos) []byte {
	return f.text[f.Offset(pos):f.Offset(end)]
}

// Raw returns the raw source bytes corresponding to a span.
func (f *File) Raw(pos, end Pos) []byte {
	lo, hi := f.Offset(pos), f.Offset(end)
	if f.rawLo == nil {
		return f.src[lo:hi]
	}
	if lo >= hi {
		r := f.rawOff(lo)
		return f.src[r:r]
	}
	return f.src[f.rawLo[lo]:f.rawHi[hi-1]]
}

// RawOffset maps a translated offset to its raw source byte offset.
func (f *File) RawOffset(off int) int { return f.rawOff(off) }

// TransOffset maps a raw byte offset to a translated offset.
func (f *File) TransOffset(raw int) int {
	if f.rawLo == nil {
		switch {
		case raw < 0:
			return 0
		case raw > len(f.text):
			return len(f.text)
		}
		return raw
	}
	return sort.Search(len(f.text), func(i int) bool {
		return int(f.rawLo[i]) >= raw
	})
}

// rawOff maps a translated offset to the raw offset of its first byte.
func (f *File) rawOff(off int) int {
	if f.rawLo == nil {
		return off
	}
	if off >= len(f.text) {
		return len(f.src)
	}
	return int(f.rawLo[off])
}

// rawAfter maps a translated offset to the raw offset following the previous translated byte.
func (f *File) rawAfter(off int) int {
	if f.rawLo == nil {
		return off
	}
	if off <= 0 {
		return 0
	}
	if off > len(f.text) {
		return len(f.src)
	}
	return int(f.rawHi[off-1])
}

// Position resolves a Pos to raw-byte offset, line, and column.
func (f *File) Position(p Pos) Position {
	raw := f.rawOff(f.Offset(p))
	i := sort.Search(len(f.lineStarts), func(i int) bool {
		return int(f.lineStarts[i]) > raw
	}) - 1
	return Position{
		Filename: f.name,
		Offset:   raw,
		Line:     i + 1,
		Column:   raw - int(f.lineStarts[i]) + 1,
	}
}

// Between returns the raw source bytes between two tokens.
func (f *File) Between(prev, next Token) []byte {
	lo := f.rawAfter(f.Offset(prev.End))
	hi := f.rawOff(f.Offset(next.Pos))
	if lo > hi {
		lo = hi
	}
	return f.src[lo:hi]
}

// Diagnostics returns the phase 1-2 diagnostics, sorted. The scanner
// merges these into its own slice.
func (f *File) Diagnostics() []Diagnostic {
	SortDiagnostics(f.diags)
	return f.diags
}
