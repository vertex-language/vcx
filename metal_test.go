package vcx_test

// The Metal ladder: tests/metal/NNN-*.metal. Each file is compiled twice,
// by vcx and by Apple's own compiler (xcrun metal); both libraries run on
// this Mac's GPU with the same inputs, and every buffer must come out the
// same -- bit for bit, or within the ulps a file states for fast math.
//
// A .metal file has no host code, so it says how to run it on //! lines
// (see tests/README.md):
//
//	//! kernel vector_add
//	//! grid 1000 64            threads, and threads per threadgroup (x y px py for 2D)
//	//! buffer float 1000 iota  [[buffer(0)]]: float, uint or int; length; contents
//	//! ulp 4                   float results may differ by this many ulps

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/vertex-language/air"
	"github.com/vertex-language/air/check"
)

func TestMetal(t *testing.T) {
	files := ladder(t, filepath.Join("tests", "metal"), "*.metal")
	if err := check.Toolchain(); err != nil {
		t.Skip(err)
	}
	ctx := context.Background()
	for _, path := range files {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".metal"), func(t *testing.T) {
			spec, err := readMetalSpec(path)
			if err != nil {
				t.Fatal(err)
			}
			libPath := filepath.Join(t.TempDir(), "vcx.metallib")
			vcxBuild(t, "-o", libPath, path)
			lib, err := os.ReadFile(libPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := check.Validate(ctx, lib); err != nil {
				t.Fatalf("vcx's library: %v", err)
			}
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			ref, err := check.Compile(ctx, string(src), air.MSL30, air.Target{OS: air.MacOS, Version: air.OS(13, 0)})
			if err != nil {
				t.Fatalf("xcrun refused it: %v", err)
			}
			want, err := check.Run(ctx, ref, spec.kernel, spec.grid, spec.inputs()...)
			if err != nil {
				t.Fatalf("running xcrun's library: %v", err)
			}
			got, err := check.Run(ctx, lib, spec.kernel, spec.grid, spec.inputs()...)
			if err != nil {
				t.Fatalf("running vcx's library: %v", err)
			}
			for i, b := range spec.buffers {
				if msg := b.compare(got.Buffers[i], want.Buffers[i], spec.ulp); msg != "" {
					t.Errorf("[[buffer(%d)]]: %s", i, msg)
				}
			}
		})
	}
}

type metalSpec struct {
	kernel  string
	grid    check.Grid
	buffers []metalBuffer
	ulp     int
}

type metalBuffer struct {
	typ  string // float, uint, int
	n    int
	fill string
	arg  float64
}

func readMetalSpec(path string) (*metalSpec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := &metalSpec{}
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		text, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "//!")
		if !ok {
			continue
		}
		w := strings.Fields(text)
		bad := fmt.Errorf("%s:%d: %q is not a //! line this harness reads", path, line, text)
		if len(w) == 0 {
			return nil, bad
		}
		var perr error
		atoi := func(i int) int {
			if i >= len(w) {
				perr = bad
				return 0
			}
			n, e := strconv.Atoi(w[i])
			if e != nil {
				perr = bad
			}
			return n
		}
		switch w[0] {
		case "kernel":
			if len(w) != 2 {
				return nil, bad
			}
			s.kernel = w[1]
		case "grid":
			switch len(w) {
			case 3:
				s.grid = check.Grid1D(atoi(1), atoi(2))
			case 5:
				s.grid = check.Grid{Threads: [3]int{atoi(1), atoi(2), 1}, PerGroup: [3]int{atoi(3), atoi(4), 1}}
			default:
				return nil, bad
			}
		case "buffer":
			if len(w) < 4 || len(w) > 5 {
				return nil, bad
			}
			switch w[1] {
			case "float", "uint", "int":
			default:
				return nil, bad
			}
			b := metalBuffer{typ: w[1], n: atoi(2), fill: w[3]}
			switch b.fill {
			case "zero", "iota", "rand":
			case "mod", "const":
				if len(w) != 5 {
					return nil, bad
				}
				b.arg, perr = strconv.ParseFloat(w[4], 64)
			default:
				return nil, bad
			}
			s.buffers = append(s.buffers, b)
		case "ulp":
			s.ulp = atoi(1)
		default:
			return nil, bad
		}
		if perr != nil {
			return nil, perr
		}
	}
	if s.kernel == "" || s.grid.Threads[0] == 0 {
		return nil, fmt.Errorf("%s: no //! kernel and //! grid lines", path)
	}
	return s, sc.Err()
}

func (s *metalSpec) inputs() []check.Buffer {
	var out []check.Buffer
	for i, b := range s.buffers {
		out = append(out, b.contents(uint32(i+1)))
	}
	return out
}

// contents is the buffer before the dispatch. rand is a xorshift from a
// seed per buffer, so that two buffers of rand differ.
func (b metalBuffer) contents(seed uint32) check.Buffer {
	x := seed*2654435761 + 1
	next := func() uint32 {
		x ^= x << 13
		x ^= x >> 17
		x ^= x << 5
		return x
	}
	out := make([]byte, 4*b.n)
	for i := 0; i < b.n; i++ {
		var v uint32
		switch b.fill {
		case "iota":
			v = uint32(i)
			if b.typ == "float" {
				v = math.Float32bits(float32(i))
			}
		case "rand":
			v = next()
			if b.typ == "float" {
				v = math.Float32bits(float32(v>>8) / (1 << 24))
			}
		case "mod":
			v = next() % uint32(b.arg)
			if b.typ == "float" {
				v = math.Float32bits(float32(v))
			}
		case "const":
			v = uint32(int32(b.arg))
			if b.typ == "float" {
				v = math.Float32bits(float32(b.arg))
			}
		}
		binary.LittleEndian.PutUint32(out[4*i:], v)
	}
	return out
}

// compare is "" when vcx's buffer is xcrun's, and what differs otherwise.
func (b metalBuffer) compare(got, want []byte, ulp int) string {
	bad := 0
	var first string
	for i := 0; i+4 <= len(want); i += 4 {
		g, w := binary.LittleEndian.Uint32(got[i:]), binary.LittleEndian.Uint32(want[i:])
		if g == w {
			continue
		}
		if b.typ == "float" {
			gf, wf := math.Float32frombits(g), math.Float32frombits(w)
			if gf != gf && wf != wf {
				continue // both NaN
			}
			if d := int64(int32(g)) - int64(int32(w)); ulp > 0 && d >= -int64(ulp) && d <= int64(ulp) {
				continue
			}
			if first == "" {
				first = fmt.Sprintf("element %d is %v, and xcrun's %v", i/4, gf, wf)
			}
		} else if first == "" {
			first = fmt.Sprintf("element %d is %d, and xcrun's %d", i/4, g, w)
		}
		bad++
	}
	if bad == 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d elements differ; %s", bad, len(want)/4, first)
}
