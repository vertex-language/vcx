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
	"github.com/vertex-language/vcx/offload"

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
	// Output is the executable, or with CompileOnly the object of the
	// one input (or a directory for several).
	Output string
	Inputs []Input

	// Libs and LibDirs are -l and -L for the link.
	Libs    []string
	LibDirs []string

	// CompileOnly stops at objects: -c.
	CompileOnly bool

	// Frameworks are Apple frameworks to link: -framework.
	Frameworks []string
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

	// IncludeFS are include directories that are filesystems rather than
	// paths, searched after IncludeDirs and before the system's: headers
	// a caller carries in an embed.FS, which no -I can name.
	IncludeFS []SystemInclude

	// Language overrides what an input's extension says it is written
	// in: -x cuda makes a .cpp a CUDA unit.
	Language Language

	// OffloadArch is the device an offload unit's kernels are compiled
	// for -- sm_75, gfx942 -- as --offload-arch names it. Empty is sm_52
	// for CUDA; HIP has no default. Several, separated by commas or
	// given in OffloadArchs, put an image for each in the fat binary,
	// and the runtime picks the newest the device runs.
	OffloadArch  string
	OffloadArchs []string

	// CUDAPath names the CUDA toolkit to use: empty finds one, "none"
	// uses none, a directory is one. CUDARuntime is how a CUDA program's
	// runtime is linked: "vcx" (vcx's own, over the driver), "static" or
	// "shared" (the toolkit's cudart), "none"; empty is the toolkit's
	// static cudart when a toolkit is there and vcx's otherwise.
	CUDAPath    string
	CUDARuntime string

	// DeviceOnly compiles only the device pass of an offload unit, and
	// HostOnly only the host pass: --cuda-device-only and
	// --cuda-host-only. Neither set is both, once the host side exists;
	// until then an offload unit compiles as its device pass alone.
	DeviceOnly bool
	HostOnly   bool

	// The Metal flags, as xcrun metal spells them: MetalStd is -std=
	// metal3.1 (empty is metal3.0), MinOS -mmacosx-version-min (empty is
	// 13.0, which picks AIR 2.5), and NoFastMath -fno-fast-math. Fast math
	// is Metal's default, as it is xcrun's.
	MetalStd   string
	MinOS      string
	NoFastMath bool

	// Modules are the interface units of the named modules a unit may
	// import, by module name: path to file. `import net.tcp;` and an
	// implementation unit's `module net.tcp;` read the file named here.
	// Nil leaves a module import unresolved.
	Modules map[string]string
}

func (c *Compiler) target() (Target, error) {
	t := DefaultTarget()
	t.MinOS = c.MinOS
	if c.Target == "" {
		// With no target named, clang builds for this Mac's release, as
		// the SDK states it; a named target without a version is built
		// for the oldest release the architecture has.
		if t.MinOS == "" {
			t.MinOS = sdkRelease()
		}
		return t, nil
	}
	named, err := TargetByName(c.Target)
	if err != nil {
		return named, err
	}
	named.MinOS = c.MinOS
	return named, nil
}

// A pass is one compilation of an input: the target it is compiled for
// and the model the analysis sees. A C++ unit has one; an offload unit
// has a host pass and a device pass, and the device pass is the one
// whose module holds the kernels.
type pass struct {
	lang   Language
	arch   OffloadArch // the device, for an offload unit
	device bool        // this is the device pass
	host   Target      // the host's target, whose headers both passes read
	tgt    Target      // the target compiled for: host, or the device
	model  types.Model
}

// passFor is the pass an input's object comes from: the host's for a
// C++ unit and for an offload unit, whose host object carries the device
// pass's image, unless DeviceOnly asks for the device pass alone.
func (c *Compiler) passFor(in Input) (pass, error) {
	if err := in.notCXX(); err != nil {
		return pass{}, err
	}
	host, err := c.target()
	if err != nil {
		return pass{}, err
	}
	p := pass{lang: c.language(in), host: host, tgt: host, model: types.ModelForTarget(host.Arch, host.OS)}
	if p.lang == LangCXX || p.lang == LangObjCXX {
		p.model.ObjC = p.lang == LangObjCXX
		return p, nil
	}
	if p.lang == LangMetal {
		return c.metalPass(p)
	}
	p.arch, err = c.offloadArch(p.lang)
	if err != nil {
		return pass{}, err
	}
	p.model.Offload = p.lang.offload()
	p.model.DeviceISA = p.arch.ISA()
	if c.DeviceOnly {
		return p.toDevice(), nil
	}
	return p, nil
}

// metalPass is a .metal file's one pass. All of it is device code, and
// there is no host whose headers it shares: the "host" is the GPU itself,
// so no CPU's macros or system headers reach it, and its data model is
// MSL's, which is LP64 whatever machine compiles it.
func (c *Compiler) metalPass(p pass) (pass, error) {
	arch, err := c.offloadArch(p.lang)
	if err != nil {
		return pass{}, err
	}
	t := arch.Target()
	p.arch, p.host, p.tgt, p.device = arch, t, t, true
	p.model = types.ModelForTarget(t.Arch, t.OS)
	p.model.Offload = types.Metal
	p.model.DeviceISA = types.AIR
	p.model.DevicePass = true
	return p, nil
}

// toDevice is the device pass of the same unit.
func (p pass) toDevice() pass {
	p.device = true
	p.tgt = p.arch.Target()
	p.model = types.ForDevice(p.model)
	p.model.DevicePass = true
	return p
}

// embedsImages reports whether the pass is a host pass that carries the
// device pass's image: every host pass of an offload unit, unless
// HostOnly asked for the host alone.
func (c *Compiler) embedsImages(p pass) bool {
	return p.lang != LangCXX && p.lang != LangObjCXX && !p.device && !c.HostOnly
}

func (c *Compiler) preprocessorConfig(in Input) preprocessor.Config {
	p, passErr := c.passFor(in)
	return c.preprocessorConfigFor(in, p, passErr)
}

func (c *Compiler) preprocessorConfigFor(in Input, p pass, passErr error) preprocessor.Config {
	cfg := preprocessor.Config{
		Std:    c.Std.tokenStd(),
		Source: in.mount(),
		Hosted: !c.Freestanding,
	}
	if c.Modules != nil {
		cfg.Modules = map[string]preprocessor.ModuleUnit{}
		for name, file := range c.Modules {
			dir := filepath.Dir(file)
			cfg.Modules[name] = preprocessor.ModuleUnit{
				Mount: preprocessor.Mount{Name: dir, FS: os.DirFS(dir)},
				Path:  filepath.Base(file),
			}
		}
	}
	for _, inc := range c.IncludeDirs {
		cfg.Search = append(cfg.Search, preprocessor.Mount{
			Name: inc,
			FS:   os.DirFS(inc),
		})
	}
	for _, inc := range c.IncludeFS {
		cfg.Search = append(cfg.Search, preprocessor.Mount{Name: inc.Name, FS: inc.FS})
	}
	// An offload unit's own headers come before the system's, and its
	// wrapper is read before the unit's first line, as clang reads its
	// __clang_cuda_runtime_wrapper.h: the execution-space macros, the
	// builtin variables and the device library are in scope everywhere.
	if own, ok := offloadHeaders(p.lang); ok && passErr == nil {
		cfg.Search = append(cfg.Search, preprocessor.Mount{Name: own.Name, FS: own.FS, System: true})
		cfg.PreIncludes = append(cfg.PreIncludes, offloadWrapper(p.lang))
	}
	// A .metal file reads no system's headers: MSL has no C library, and
	// its standard library is vcx's own, above.
	for _, sys := range c.SystemIncludes() {
		if p.lang == LangMetal {
			break
		}
		cfg.Search = append(cfg.Search, preprocessor.Mount{Name: sys.Name, FS: sys.FS, System: true, Framework: sys.Framework})
	}
	// The target's own macros first, so that -D and -U on the command line
	// can shadow any of them. Both passes of an offload unit carry the
	// host's macros, since both read the host's headers; the device pass
	// adds the device's on top, as clang's does.
	if passErr == nil {
		cfg.Predefines = append(cfg.Predefines, p.host.Predefines()...)
		cfg.Predefines = append(cfg.Predefines, offloadPredefines(p.lang, p.arch, p.device)...)
		if p.host.Dialect == DialectGNU || p.device {
			cfg.Vendor = p.tgt.gnuVendor()
		}
		if p.lang == LangObjCXX {
			cfg.ObjC = true
			cfg.Predefines = append(cfg.Predefines, objcPredefines()...)
			if cfg.Vendor != nil {
				for _, f := range objcFeatures {
					cfg.Vendor.Features[f] = true
				}
			}
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

// A Scan is what a unit declares about how it is built, which a build
// reads before compiling anything: its module directives -- what module it
// is (`export module M;`, `module M;`) and what it imports -- and the
// libraries and frameworks its pragmas ask the link for.
type Scan struct {
	Modules []preprocessor.ModuleDirective
	Links   []preprocessor.LinkDirective
}

// Scan runs phases 1-4 and returns what the unit declares about its build.
// It is a dependency scan, which is why it stops short of parsing.
func (c *Compiler) Scan(in Input) (Scan, []Diagnostic, error) {
	f, diags, err := c.Source(in)
	if err != nil {
		return Scan{}, nil, err
	}
	p, err := c.passFor(in)
	if err != nil {
		return Scan{}, nil, err
	}
	cfg := c.preprocessorConfigFor(in, p, nil)
	// A scan names what a unit imports; it does not read it.
	cfg.Modules = nil
	pp := preprocessor.New(cfg)
	_, ppDiags := pp.Run(f)
	for _, d := range ppDiags {
		diags = append(diags, Diagnostic{Severity: d.Severity, Site: d.Site, Message: d.Msg, Name: d.Name})
	}
	return Scan{Modules: pp.Modules(), Links: pp.Links()}, diags, nil
}

// ModuleDirectives is Scan's module directives alone.
func (c *Compiler) ModuleDirectives(in Input) ([]preprocessor.ModuleDirective, []Diagnostic, error) {
	sc, diags, err := c.Scan(in)
	return sc.Modules, diags, err
}

// Parse runs phases 1-4 and parses the token stream into an ast.File.
func (c *Compiler) Parse(in Input, mode parser.Mode) (*ast.File, []Diagnostic, error) {
	p, err := c.passFor(in)
	return c.parseFor(in, mode, p, err)
}

func (c *Compiler) parseFor(in Input, mode parser.Mode, p pass, passErr error) (*ast.File, []Diagnostic, error) {
	f, diags, err := c.Source(in)
	if err != nil {
		return nil, nil, err
	}

	cfg := c.preprocessorConfigFor(in, p, passErr)
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
	if p.lang == LangObjCXX {
		mode |= parser.ObjC
	}
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

	p, err := c.passFor(in)
	if err != nil {
		return diags, err
	}

	_, semaDiags := sema.Analyze(file, p.model)
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

	p, err := c.passFor(in)
	if err != nil {
		return nil, diags, err
	}
	model := p.model

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
	visited := map[*sema.Scope]bool{}
	// Every namespace, not only the global one: a record declared inside
	// `namespace vertex` has a layout like any other.
	var walk func(scope *sema.Scope)
	walk = func(scope *sema.Scope) {
		if scope == nil || visited[scope] {
			return
		}
		visited[scope] = true
		for _, syms := range scope.Symbols {
			for _, sym := range syms {
				switch rs := sym.(type) {
				case *sema.NamespaceSymbol:
					walk(rs.InnerScope)
				case *sema.RecordSymbol:
					if rs.Record == nil || seen[rs.Record] || !rs.Record.Complete {
						continue
					}
					seen[rs.Record] = true
					out = append(out, recordLayout(model, rs.Record))
				}
			}
		}
	}
	walk(res.GlobalScope)
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

// IR runs the analysis and lowers the AST into a VIR module: the host's
// module for a C++ unit, and for an offload unit the host pass's, which
// embeds the device pass's image, or the device pass's own under
// DeviceOnly.
func (c *Compiler) IR(in Input) (*ir.Module, []Diagnostic, error) {
	p, err := c.passFor(in)
	if err != nil {
		return nil, nil, err
	}
	var images []offload.Image
	var diags []Diagnostic
	if c.embedsImages(p) {
		images, diags, err = c.deviceImages(in, p)
		if err != nil || HasErrors(diags) {
			return nil, diags, err
		}
	}
	mod, more, err := c.irFor(in, p, images)
	return mod, append(diags, more...), err
}

// deviceImages runs the device pass of an offload unit for each
// architecture asked for and is the images the host pass embeds. The
// passes are separate compilations: __CUDA_ARCH__ differs, and a body
// may read it.
func (c *Compiler) deviceImages(in Input, p pass) ([]offload.Image, []Diagnostic, error) {
	archs, err := c.offloadArchs(p.lang)
	if err != nil {
		return nil, nil, err
	}
	var images []offload.Image
	var diags []Diagnostic
	for _, arch := range archs {
		dp := p
		dp.arch = arch
		dp.model.DeviceISA = arch.ISA()
		dp = dp.toDevice()
		mod, more, err := c.irFor(in, dp, nil)
		diags = append(diags, more...)
		if err != nil || mod == nil || HasErrors(diags) {
			return nil, diags, err
		}
		data, err := emitObject(mod, dp.tgt, dp.arch)
		if err != nil {
			return nil, diags, err
		}
		images = append(images, imageOf(dp.arch, p.host, data))
	}
	return images, diags, nil
}

// imageOf describes a device image for the container it travels in.
func imageOf(arch OffloadArch, host Target, data []byte) offload.Image {
	im := offload.Image{Arch: arch.Name, Data: data, Host: host.OS}
	if arch.ISA() == types.AMDGCN {
		im.Kind = offload.ELF
		return im
	}
	im.Kind = offload.PTX
	im.SM = arch.SM.SM
	im.ISAMajor, im.ISAMinor = 8, 0
	if i := strings.Index(string(data), ".version "); i >= 0 {
		fmt.Sscanf(string(data[i+len(".version "):]), "%d.%d", &im.ISAMajor, &im.ISAMinor)
	}
	return im
}

// irFor is one pass of an input: its module.
func (c *Compiler) irFor(in Input, p pass, images []offload.Image) (*ir.Module, []Diagnostic, error) {
	file, diags, err := c.parseFor(in, parser.DefaultMode, p, nil)
	if err != nil {
		return nil, nil, err
	}
	defer file.Release()
	tgt, model := p.tgt, p.model

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
		Images:       images,
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
	p, err := c.passFor(in)
	if err != nil {
		return nil, diags, err
	}
	obj, err := emitObject(mod, p.tgt, p.arch)
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
