package lower

// A .metal unit's kernels, and the builtins vcx's <metal_stdlib> is
// written over.
//
// A Metal kernel's parameters are bound by attributes: a buffer is
// [[buffer(n)]], and a built-in argument -- [[thread_position_in_grid]]
// and the rest -- is filled by Metal rather than bound. VIR has neither,
// so the kernel carries them as attachments the AIR backend reads:
//
//	!binding 0, 1, -                         each buffer's index
//	!param_space device, constant, -         where each buffer points
//	!param_builtin -, -, "thread_position_in_grid uint"
//
// A built-in stays a parameter, and the backend makes it AIR's own
// built-in argument, which is what gives it Metal's meaning exactly: in
// the last threadgroup of a grid the groups do not divide, a thread's
// threads_per_threadgroup is its own group's smaller size, which no sum
// of VIR's work-item verbs could say.

import (
	"strconv"
	"strings"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// metal reports whether the unit is a .metal file.
func (u *unit) metal() bool { return u.model.Offload == types.Metal }

// metalBuiltinArgs are MSL's built-in kernel arguments, and whether each
// may be a vector: the grid positions may, the counts and indices not.
var metalBuiltinArgs = map[string]bool{
	"thread_position_in_grid":          true,
	"thread_position_in_threadgroup":   true,
	"threadgroup_position_in_grid":     true,
	"threads_per_threadgroup":          true,
	"threadgroups_per_grid":            true,
	"threads_per_grid":                 true,
	"dispatch_threads_per_threadgroup": true,
	"thread_index_in_threadgroup":      false,
	"thread_index_in_simdgroup":        false,
	"simdgroup_index_in_threadgroup":   false,
	"simdgroups_per_threadgroup":       false,
	"threads_per_simdgroup":            false,
	"thread_execution_width":           false,
}

// A metalArg is what one kernel parameter is: a buffer at an index in a
// space, or a built-in of an MSL type.
type metalArg struct {
	builtin string // the attribute, for a built-in
	typ     string // its MSL type: uint, ushort2, ...
	index   int    // the buffer's [[buffer(n)]]
	space   string // device or constant
}

// declareMetalKernel declares a kernel's parameters and attaches their
// bindings. It reports errors at the parameter, in xcrun's terms where
// xcrun has them.
func (u *unit) declareMetalKernel(fn *sema.FuncSymbol, f *ir.Func) {
	decls := metalParamDecls(fn)
	args := make([]metalArg, len(fn.Params))
	used := map[int]bool{}
	var unindexed []int
	for i, p := range fn.Params {
		u.params[fn] = append(u.params[fn], u.declareParam(f, p))
		if i >= len(decls) {
			continue
		}
		a, explicit, ok := u.metalArgOf(p, decls[i])
		if !ok {
			continue
		}
		args[i] = a
		switch {
		case a.builtin != "":
		case explicit && used[a.index]:
			u.errorf(decls[i].Pos(), "cannot reserve 'buffer' resource location at index %d: another parameter of %s has it", a.index, fn.SymName)
		case explicit:
			used[a.index] = true
		default:
			unindexed = append(unindexed, i)
		}
	}
	// A buffer with no [[buffer(n)]] takes the lowest index nothing
	// else has, in order, as xcrun gives it one.
	next := 0
	for _, i := range unindexed {
		for used[next] {
			next++
		}
		args[i].index = next
		used[next] = true
	}

	var binding, space, builtin []ir.MetaArg
	hasBuiltin := false
	for _, a := range args {
		if a.builtin != "" {
			hasBuiltin = true
			binding = append(binding, ir.MIdent("-"))
			space = append(space, ir.MIdent("-"))
			builtin = append(builtin, ir.MStr(a.builtin+" "+a.typ))
			continue
		}
		binding = append(binding, ir.MInt(int64(a.index)))
		space = append(space, ir.MIdent(a.space))
		builtin = append(builtin, ir.MIdent("-"))
	}
	if len(args) == 0 {
		return
	}
	f.Meta(ir.Attached("binding", binding...), ir.Attached("param_space", space...))
	if hasBuiltin {
		f.Meta(ir.Attached("param_builtin", builtin...))
	}
}

// metalArgOf reads one parameter's attributes: a built-in, or a buffer
// and whether its index was written.
func (u *unit) metalArgOf(p *sema.VarSymbol, d *ast.ParamDecl) (a metalArg, explicit, ok bool) {
	var attrs []*ast.Attr
	attrs = append(attrs, d.Attrs...)
	if d.Specs != nil {
		for _, g := range d.Specs.AllAttrs(nil) {
			if g != nil {
				attrs = append(attrs, g.Attrs...)
			}
		}
	}
	if leaf := nameLeaf(d.Decl); leaf != nil {
		attrs = append(attrs, leaf.Attrs...)
	}
	at := d.Pos()
	a.index = -1
	var spaceNames []string
	for _, attr := range attrs {
		if attr == nil || attr.Name == nil {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(attr.Name.Text(u.unit), "__"), "__")
		switch {
		case name == "metal_device" || name == "metal_constant" || name == "metal_threadgroup" || name == "metal_thread":
			spaceNames = append(spaceNames, strings.TrimPrefix(name, "metal_"))
		case name == "buffer":
			n, good := u.attrInt(attr)
			if !good || n < 0 || n > 30 {
				u.errorf(at, "[[buffer(n)]] takes an index from 0 to 30")
				return a, false, false
			}
			a.index, explicit = n, true
		case name == "texture" || name == "sampler":
			u.errorf(at, "[[%s(n)]] arguments are not built yet: vcx's kernels take buffers", name)
			return a, false, false
		case name == "threadgroup":
			u.errorf(at, "[[threadgroup(n)]] arguments are not built yet; a threadgroup array declared in the kernel is")
			return a, false, false
		case name == "stage_in":
			u.errorf(at, "[[stage_in]] is a vertex or fragment function's, and vcx compiles kernels")
			return a, false, false
		default:
			if _, isBuiltin := metalBuiltinArgs[name]; isBuiltin {
				a.builtin = name
			}
		}
	}

	t := types.Unqualify(p.SymType)
	if a.builtin != "" {
		typ, good := metalBuiltinType(t)
		if !good || typ != "uint" && typ != "ushort" && !metalBuiltinArgs[a.builtin] {
			u.errorf(at, "type '%s' is not valid for attribute '%s'", p.SymType.String(), a.builtin)
			return a, false, false
		}
		a.typ = typ
		return a, false, true
	}
	_, isPtr := t.(*types.Pointer)
	if !isPtr && !types.IsReference(t) {
		if explicit {
			u.errorf(at, "type '%s' is not valid for attribute 'buffer'", p.SymType.String())
		} else {
			u.errorf(at, "invalid type '%s' for input declaration in a kernel function", p.SymType.String())
		}
		return a, false, false
	}
	switch {
	case len(spaceNames) == 0:
		u.errorf(at, "pointer type must have explicit address space qualifier: kernel parameter %s is a buffer, device or constant", p.SymName)
		return a, false, false
	case spaceNames[0] == "device" || spaceNames[0] == "constant":
		a.space = spaceNames[0]
	default:
		u.errorf(at, "a %s pointer is not a buffer: a kernel's buffers are device or constant", spaceNames[0])
		return a, false, false
	}
	return a, explicit, true
}

// metalBuiltinType is a built-in argument's MSL type name: uint or
// ushort, or a 2- or 3-vector of one.
func metalBuiltinType(t types.Type) (string, bool) {
	if b, ok := t.(*types.Basic); ok {
		switch b.K {
		case types.UInt:
			return "uint", true
		case types.UShort:
			return "ushort", true
		}
		return "", false
	}
	if rec := types.AsRecord(t); rec != nil {
		switch rec.Name {
		case "uint2", "uint3", "ushort2", "ushort3":
			return rec.Name, true
		}
	}
	return "", false
}

// attrInt is an attribute's one integer argument: the 3 of buffer(3).
func (u *unit) attrInt(attr *ast.Attr) (int, bool) {
	if attr.Args.Hi-attr.Args.Lo != 1 {
		return 0, false
	}
	s := strings.TrimRight(u.unit.Text(attr.Args.Lo), "uUlL")
	n, err := strconv.ParseInt(s, 0, 32)
	return int(n), err == nil
}

// metalParamDecls is the parameters of fn's definition as written, which
// is where their attributes are.
func metalParamDecls(fn *sema.FuncSymbol) []*ast.ParamDecl {
	if fn.Decl == nil {
		return nil
	}
	var params []*ast.ParamDecl
	for d := fn.Decl.Decl; d != nil; {
		switch x := d.(type) {
		case *ast.FuncDeclarator:
			params, d = x.Params, x.Inner
		case *ast.PointerDeclarator:
			d = x.Inner
		case *ast.ArrayDeclarator:
			d = x.Inner
		case *ast.ParenDeclarator:
			d = x.Inner
		default:
			return params
		}
	}
	return params
}

// nameLeaf is the declarator's name, where an attribute on the declared
// entity is written.
func nameLeaf(d ast.Declarator) *ast.NameDeclarator {
	for d != nil {
		switch x := d.(type) {
		case *ast.NameDeclarator:
			return x
		case *ast.PointerDeclarator:
			d = x.Inner
		case *ast.ArrayDeclarator:
			d = x.Inner
		case *ast.ParenDeclarator:
			d = x.Inner
		case *ast.FuncDeclarator:
			d = x.Inner
		default:
			return nil
		}
	}
	return nil
}

// metalCall lowers the __metal_* builtins.
func (fl *fn) metalCall(name string, args []ir.Value, e *ast.CallExpr) ir.Value {
	b := fl.blk
	base := strings.TrimPrefix(name, "__metal_")
	f := func(i int) ir.F32 { return args[i].(ir.F32) }
	i32 := func(i int) ir.I32 { return args[i].(ir.I32) }
	all := b.I32.Const(-1) // every lane of the SIMD group
	switch base {
	case "threadgroup_barrier":
		b.Barrier()
		return nil
	case "simdgroup_barrier":
		// The SIMD group runs in lockstep: there is nothing to wait for.
		return nil

	case "sqrt":
		return b.F32.Sqrt(f(0))
	case "rsqrt":
		return b.F32.RsqrtApprox(f(0))
	case "rcp":
		return b.F32.RcpApprox(f(0))
	case "fabs":
		return b.F32.Abs(f(0))
	case "fmin":
		return b.F32.MinNum(f(0), f(1))
	case "fmax":
		return b.F32.MaxNum(f(0), f(1))
	case "floor":
		return b.F32.Floor(f(0))
	case "ceil":
		return b.F32.Ceil(f(0))
	case "trunc":
		return b.F32.Trunc(f(0))
	case "rint":
		return b.F32.Nearest(f(0))
	case "fma":
		return b.F32.FMA(f(0), f(1), f(2))
	case "copysign":
		return b.F32.CopySign(f(0), f(1))
	case "exp2":
		return b.F32.Exp2Approx(f(0))
	case "log2":
		return b.F32.Log2Approx(f(0))
	case "sin":
		return b.F32.SinApprox(f(0))
	case "cos":
		return b.F32.CosApprox(f(0))

	case "clz_u":
		return b.I32.Clz(i32(0))
	case "ctz_u":
		return b.I32.Ctz(i32(0))
	case "popcount_u":
		return b.I32.Popcnt(i32(0))
	case "mulhi_i":
		return b.I32.SMulHi(i32(0), i32(1))
	case "mulhi_u":
		return b.I32.UMulHi(i32(0), i32(1))
	case "as_uint":
		return b.I32.BitcastF32(f(0))
	case "as_float":
		return b.F32.BitcastI32(i32(0))

	case "simd_shuffle_u":
		return b.I32.WaveShflIdx(i32(0), i32(1), all)
	case "simd_shuffle_up_u":
		return b.I32.WaveShflUp(i32(0), i32(1), all)
	case "simd_shuffle_down_u":
		return b.I32.WaveShflDown(i32(0), i32(1), all)
	case "simd_shuffle_xor_u":
		return b.I32.WaveShflXor(i32(0), i32(1), all)
	case "simd_broadcast_first_u":
		return b.I32.WaveReadFirstLane(i32(0))
	case "simd_ballot":
		// A bool is an int here, as everywhere in the lowering.
		return b.I64.WaveBallot(fl.i1Of(args[0]), all)
	case "simd_any":
		return b.I32.ZExtI1(b.I1.WaveAny(fl.i1Of(args[0]), all))
	case "simd_all":
		return b.I32.ZExtI1(b.I1.WaveAll(fl.i1Of(args[0]), all))
	case "simd_lane":
		return b.I32.LaneID()
	case "simd_width":
		return b.I32.WaveSize()
	}
	if rest, ok := strings.CutPrefix(base, "atomic_"); ok {
		return fl.metalAtomic(rest, args, e)
	}
	fl.u.errorf(e.Pos(), "internal: no lowering for %s", name)
	return nil
}

// metalAtomic lowers __metal_atomic_op_t: relaxed, which is every
// ordering Metal's atomics have, at the device's scope.
func (fl *fn) metalAtomic(rest string, args []ir.Value, e *ast.CallExpr) ir.Value {
	b := fl.blk
	op, typ, _ := strings.Cut(rest, "_")
	p := args[0].(ir.Ptr)
	o, scope := ir.Monotonic, ir.DeviceScope
	if typ == "f" {
		switch op {
		case "add":
			return b.F32.AtomicRmwAdd(args[1].(ir.F32), p, o, scope)
		case "sub":
			return b.F32.AtomicRmwAdd(b.F32.Neg(args[1].(ir.F32)), p, o, scope)
		case "xchg":
			return b.F32.BitcastI32(b.I32.AtomicRmwXchg(b.I32.BitcastF32(args[1].(ir.F32)), p, o, scope))
		case "load":
			return b.F32.BitcastI32(b.I32.AtomicLoad(p, o, scope))
		case "store":
			b.I32.AtomicStore(b.I32.BitcastF32(args[1].(ir.F32)), p, o, scope)
			return nil
		}
	}
	n := b.I32
	switch op {
	case "load":
		return n.AtomicLoad(p, o, scope)
	case "store":
		n.AtomicStore(args[1].(ir.I32), p, o, scope)
		return nil
	case "cas":
		return n.AtomicCas(args[1].(ir.I32), args[2].(ir.I32), p, o, o, scope)
	}
	v := args[1].(ir.I32)
	switch op {
	case "add":
		return n.AtomicRmwAdd(v, p, o, scope)
	case "sub":
		return n.AtomicRmwSub(v, p, o, scope)
	case "and":
		return n.AtomicRmwAnd(v, p, o, scope)
	case "or":
		return n.AtomicRmwOr(v, p, o, scope)
	case "xor":
		return n.AtomicRmwXor(v, p, o, scope)
	case "xchg":
		return n.AtomicRmwXchg(v, p, o, scope)
	case "min":
		if typ == "u" {
			return n.AtomicRmwUMin(v, p, o, scope)
		}
		return n.AtomicRmwSMin(v, p, o, scope)
	case "max":
		if typ == "u" {
			return n.AtomicRmwUMax(v, p, o, scope)
		}
		return n.AtomicRmwSMax(v, p, o, scope)
	}
	fl.u.errorf(e.Pos(), "internal: no lowering for __metal_atomic_%s", rest)
	return nil
}
