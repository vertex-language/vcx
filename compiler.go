package vcx

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/lower"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/parser"
	"github.com/vertex-language/vcx/preprocessor"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// Std specifies the C++ language standard version.
type Std uint8

const (
	Cxx20 Std = iota
	Cxx23
	Cxx26
)

func (s Std) tokenStd() token.Std {
	switch s {
	case Cxx26:
		return token.Cxx26
	default:
		return token.Cxx23
	}
}

// BuildParams controls a compilation and link run.
type BuildParams struct {
	Output string
	Inputs []Input
	Libs   []string
}

// Compiler is the central compiler driver.
type Compiler struct {
	Target       string
	Std          Std
	IncludeDirs  []string
	Sysroot      string
	Defs         []string
	Undefs       []string
	Freestanding bool
	FS           fs.FS
}

func (c *Compiler) target() (Target, error) {
	if c.Target == "" {
		return DefaultTarget(), nil
	}
	return TargetByName(c.Target)
}

func (c *Compiler) preprocessorConfig(in Input) preprocessor.Config {
	cfg := preprocessor.Config{
		Std:    c.Std.tokenStd(),
		Source: in.mount(),
		Hosted: !c.Freestanding,
	}
	for _, inc := range c.IncludeDirs {
		cfg.Search = append(cfg.Search, preprocessor.Mount{
			Name: inc,
			FS:   os.DirFS(inc),
		})
	}
	for _, sys := range c.SystemIncludes() {
		cfg.Search = append(cfg.Search, preprocessor.Mount{Name: sys.Name, FS: sys.FS, System: true})
	}
	// The target's own macros first, so that -D and -U on the command line
	// can shadow any of them.
	if tgt, err := c.target(); err == nil {
		cfg.Predefines = append(cfg.Predefines, tgt.Predefines()...)
		if tgt.Dialect == DialectGNU {
			cfg.Vendor = tgt.gnuVendor()
		}
	}
	for _, d := range c.Defs {
		cfg.Predefines = append(cfg.Predefines, preprocessor.Predefine{
			Kind: preprocessor.PredefineDefine,
			Text: d,
		})
	}
	for _, u := range c.Undefs {
		cfg.Predefines = append(cfg.Predefines, preprocessor.Predefine{
			Kind: preprocessor.PredefineUndef,
			Text: u,
		})
	}
	return cfg
}

// Source loads the input file into a token.File position space.
func (c *Compiler) Source(in Input) (*token.File, []Diagnostic, error) {
	f, err := in.load()
	if err != nil {
		return nil, nil, err
	}
	return f, nil, nil
}

// Preprocess runs phase 4 and returns the preprocessed tokens.
func (c *Compiler) Preprocess(in Input) ([]byte, []Diagnostic, error) {
	f, diags, err := c.Source(in)
	if err != nil {
		return nil, nil, err
	}
	if in.isPreprocessed() {
		return f.Text(), diags, nil
	}

	cfg := c.preprocessorConfig(in)
	pp := preprocessor.New(cfg)
	toks, ppDiags := pp.Run(f)

	for _, d := range ppDiags {
		diags = append(diags, Diagnostic{
			Severity: d.Severity,
			Site:     d.Site,
			Message:  d.Msg,
			Name:     d.Name,
		})
	}

	var out []byte
	unit := parser.NewUnit(toks)
	for i := 0; i < unit.Len(); i++ {
		t := ast.Tok(i)
		out = append(out, []byte(unit.Text(t))...)
		if i+1 < unit.Len() {
			out = append(out, ' ')
		}
	}

	return out, diags, nil
}

// Parse runs phases 1-4 and parses the token stream into an ast.File.
func (c *Compiler) Parse(in Input, mode parser.Mode) (*ast.File, []Diagnostic, error) {
	f, diags, err := c.Source(in)
	if err != nil {
		return nil, nil, err
	}

	cfg := c.preprocessorConfig(in)
	if mode&parser.ParseComments != 0 {
		cfg.KeepComments = true
	}
	pp := preprocessor.New(cfg)
	toks, ppDiags := pp.Run(f)

	for _, d := range ppDiags {
		diags = append(diags, Diagnostic{
			Severity: d.Severity,
			Site:     d.Site,
			Message:  d.Msg,
			Name:     d.Name,
		})
	}

	unit := parser.NewUnit(toks)
	file, parseDiags := parser.Parse(unit, mode)

	for _, d := range parseDiags {
		diags = append(diags, Diagnostic{
			Severity: d.Severity,
			Site:     d.Site,
			Message:  d.Message,
		})
	}

	return file, diags, nil
}

// Check parses and type checks the input.
func (c *Compiler) Check(in Input) ([]Diagnostic, error) {
	file, diags, err := c.Parse(in, parser.DefaultMode)
	if err != nil {
		return nil, err
	}
	defer file.Release()

	tgt, err := c.target()
	if err != nil {
		return diags, err
	}

	model := types.ModelForTarget(tgt.Arch, tgt.OS)
	_, semaDiags := sema.Analyze(file, model)
	for _, d := range semaDiags {
		diags = append(diags, Diagnostic{
			Severity: d.Severity,
			Site:     d.Site,
			Message:  d.Message,
		})
	}

	return diags, nil
}

// RecordLayout describes the computed memory layout of a class, struct, or union.
type RecordLayout struct {
	Name   string
	Kind   string // "struct", "class" or "union"
	Size   int64
	Align  int64
	Bases  []BaseLayout
	Fields []FieldLayout

	// VTables holds the virtual tables for a polymorphic class.
	VTables []types.VTable

	// VBTables holds the virtual base tables for finding virtual bases.
	VBTables []types.VBTable
}

// BaseLayout is where a base subobject sits inside its derived class.
type BaseLayout struct {
	Name    string
	Offset  int64
	Virtual bool
}

// FieldLayout is where a non-static data member sits.
type FieldLayout struct {
	Name     string
	Offset   int64
	Size     int64
	BitField bool
	Width    int64
	Bit      int64
}

// Layout runs semantic analysis and returns the layout of every record defined in the input.
func (c *Compiler) Layout(in Input) ([]RecordLayout, []Diagnostic, error) {
	file, diags, err := c.Parse(in, parser.DefaultMode)
	if err != nil {
		return nil, nil, err
	}
	defer file.Release()

	tgt, err := c.target()
	if err != nil {
		return nil, diags, err
	}
	model := types.ModelForTarget(tgt.Arch, tgt.OS)

	res, semaDiags := sema.Analyze(file, model)
	for _, d := range semaDiags {
		diags = append(diags, Diagnostic{
			Severity: d.Severity,
			Site:     d.Site,
			Message:  d.Message,
		})
	}
	if res == nil || res.GlobalScope == nil {
		return nil, diags, nil
	}

	var out []RecordLayout
	seen := map[*types.Record]bool{}
	for _, syms := range res.GlobalScope.Symbols {
		for _, sym := range syms {
			rs, ok := sym.(*sema.RecordSymbol)
			if !ok || rs.Record == nil || seen[rs.Record] || !rs.Record.Complete {
				continue
			}
			seen[rs.Record] = true
			out = append(out, recordLayout(model, rs.Record))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, diags, nil
}

func recordLayout(m types.Model, r *types.Record) RecordLayout {
	offsets := make([]int64, len(r.Fields))
	baseOffs := make([]int64, len(r.Bases))
	size, align, _ := m.LayoutWithBases(r, offsets, baseOffs)

	rl := RecordLayout{
		Name:  r.Name,
		Kind:  tagName(r.Tag),
		Size:  size,
		Align: align,
	}

	// Emit non-virtual bases first with their contributed members, then virtual bases.
	for i, b := range r.Bases {
		if b.Virtual {
			continue
		}
		bRec, isRec := types.Unqualify(b.Type).(*types.Record)
		if !isRec || !bRec.Complete {
			continue
		}
		rl.Bases = append(rl.Bases, BaseLayout{Name: bRec.Name, Offset: baseOffs[i]})
		rl.Fields = append(rl.Fields, inheritedFields(m, bRec, baseOffs[i])...)
	}

	for i := range r.Fields {
		rl.Fields = append(rl.Fields, fieldLayout(m, r, i, 0, offsets[i]))
	}

	vbOffsets := m.VirtualBaseOffsets(r)
	for _, vb := range types.VirtualBases(r) {
		at, known := vbOffsets[vb]
		if !known {
			continue
		}
		rl.Bases = append(rl.Bases, BaseLayout{Name: vb.Name, Offset: at, Virtual: true})
		rl.Fields = append(rl.Fields, inheritedFields(m, vb, at)...)
	}

	rl.VTables = m.VTables(r)
	rl.VBTables = m.VBTables(r)
	return rl
}

// inheritedFields returns members contributed by a base subobject placed at offset at.
func inheritedFields(m types.Model, r *types.Record, at int64) []FieldLayout {
	offsets := make([]int64, len(r.Fields))
	baseOffs := make([]int64, len(r.Bases))
	m.LayoutWithBases(r, offsets, baseOffs)

	var out []FieldLayout
	for i, b := range r.Bases {
		// Virtual bases are placed by the complete object, so skip them here.
		if b.Virtual {
			continue
		}
		bRec, isRec := types.Unqualify(b.Type).(*types.Record)
		if !isRec || !bRec.Complete {
			continue
		}
		out = append(out, inheritedFields(m, bRec, at+baseOffs[i])...)
	}
	for i := range r.Fields {
		out = append(out, fieldLayout(m, r, i, at, offsets[i]))
	}
	return out
}

// fieldLayout is field i of r, in a class that placed r at `at`.
func fieldLayout(m types.Model, r *types.Record, i int, at, off int64) FieldLayout {
	f := r.Fields[i]
	fs, _ := m.Sizeof(f.Type)
	fl := FieldLayout{
		Name:     f.Name,
		Offset:   at + off,
		Size:     fs,
		BitField: f.BitField,
		Width:    f.Width,
	}
	if f.BitField {
		if unit, _, bit, ok := m.BitField(r, i); ok {
			fl.Bit = (at+unit)*8 + bit
		}
	}
	return fl
}

func tagName(t types.TagKind) string {
	switch t {
	case types.TagUnion:
		return "union"
	case types.TagClass:
		return "class"
	}
	return "struct"
}

func roundUpTo(n, a int64) int64 {
	if a <= 1 {
		return n
	}
	return (n + a - 1) / a * a
}

// IR runs the analysis and lowers the AST into a VIR module.
func (c *Compiler) IR(in Input) (*ir.Module, []Diagnostic, error) {
	file, diags, err := c.Parse(in, parser.DefaultMode)
	if err != nil {
		return nil, nil, err
	}
	defer file.Release()

	tgt, err := c.target()
	if err != nil {
		return nil, diags, err
	}
	model := types.ModelForTarget(tgt.Arch, tgt.OS)

	res, semaDiags := sema.Analyze(file, model)
	for _, d := range semaDiags {
		diags = append(diags, Diagnostic{Severity: d.Severity, Site: d.Site, Message: d.Message})
	}
	if HasErrors(diags) {
		return nil, diags, nil
	}

	irTgt, err := irTarget(tgt)
	if err != nil {
		return nil, diags, err
	}

	mod, lowDiags := lower.Lower(file, res, lower.Options{
		Name:         moduleName(in),
		Target:       irTgt,
		Model:        model,
		SymbolPrefix: symbolPrefix(tgt),
		ABI:          manglingABI(tgt),
	})
	for _, d := range lowDiags {
		diags = append(diags, Diagnostic{
			Severity: d.Severity,
			Site:     siteOf(file.Unit, d.Pos),
			Message:  d.Message,
		})
	}
	return mod, diags, nil
}

// Object lowers the input and writes a relocatable object file.
func (c *Compiler) Object(in Input) ([]byte, []Diagnostic, error) {
	mod, diags, err := c.IR(in)
	if err != nil {
		return nil, diags, err
	}
	if mod == nil || HasErrors(diags) {
		return nil, diags, nil
	}
	tgt, err := c.target()
	if err != nil {
		return nil, diags, err
	}
	obj, err := emitObject(mod, tgt)
	return obj, diags, err
}

// siteOf maps a token index back to its source position.
func siteOf(u ast.Unit, pos ast.Tok) preprocessor.Site {
	if u == nil || !pos.IsValid() {
		return preprocessor.Site{}
	}
	if s, ok := u.(interface {
		Site(ast.Tok) preprocessor.Site
	}); ok {
		return s.Site(pos)
	}
	return preprocessor.Site{}
}

// moduleName returns a valid VIR identifier derived from the source file name.
func moduleName(in Input) string {
	base := filepath.Base(in.Name)
	if i := strings.IndexByte(base, '.'); i > 0 {
		base = base[:i]
	}

	var b strings.Builder
	for i, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			if i == 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}

	if b.Len() == 0 {
		return "a"
	}
	return b.String()
}

// Build compiles inputs into an object file.
func (c *Compiler) Build(params BuildParams) error {
	// Object generation only; linking is handled externally.
	for _, in := range params.Inputs {
		if !in.isSource() {
			continue
		}
		obj, diags, err := c.Object(in)
		if err != nil {
			return err
		}
		if HasErrors(diags) {
			return &DiagnosticError{Diagnostics: diags}
		}
		if obj == nil {
			return fmt.Errorf("no object produced for %s", in.Name)
		}
		out := params.Output
		if out == "" {
			out = moduleName(in) + ".o"
		}
		if err := os.WriteFile(out, obj, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Run compiles and runs a single source file.
func (c *Compiler) Run(path string) ([]byte, error) {
	in := File(path)
	diags, err := c.Check(in)
	if err != nil {
		return nil, err
	}
	if HasErrors(diags) {
		return nil, &DiagnosticError{Diagnostics: diags}
	}
	return []byte(fmt.Sprintf("Compiled %s successfully\n", path)), nil
}
