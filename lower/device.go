package lower

import (
	"fmt"
	"math"
	"strings"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/literal"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// An offload unit -- CUDA or HIP -- is lowered twice: once for the host,
// once for the device. Each pass lowers the functions and objects that
// belong to it, and a reference across the line is an error the way
// clang reports it, at the call rather than the declaration, since a
// __host__ __device__ function may name a host function in a branch
// only the host takes.

// offload reports whether the unit is written in an offload language.
func (u *unit) offload() bool { return u.model.Offload != types.NoOffload }

// devicePass reports whether this is the device pass of an offload unit.
func (u *unit) devicePass() bool { return u.model.DevicePass }

// lowersFunc reports whether this pass defines fn: every function of a
// C++ unit; the device pass a kernel and every __device__ function; the
// host pass the host's. (A kernel's launch stub in the host pass is
// Phase C's.)
func (u *unit) lowersFunc(fn *sema.FuncSymbol) bool {
	if !u.offload() {
		return true
	}
	if u.devicePass() {
		return fn.Space.OnDevice()
	}
	// A kernel has a host side too: its launch stub.
	return fn.Space.OnHost() || fn.Space == sema.SpaceGlobal
}

// lowersGlobal reports whether this pass defines the object: the device
// pass the ones with a memory space, the host pass everything but
// __shared__ storage, which has no host instance.
func (u *unit) lowersGlobal(v *sema.VarSymbol) bool {
	if !u.offload() {
		return true
	}
	if u.devicePass() {
		return v.Memory.OnDevice()
	}
	return v.Memory != sema.MemShared
}

// domainOf is the storage domain an object's memory space names.
func (u *unit) domainOf(v *sema.VarSymbol) ir.Domain {
	if u.devicePass() {
		switch v.Memory {
		case sema.MemShared:
			return ir.Shared
		case sema.MemConstant:
			return ir.RO
		}
	}
	return ir.RW
}

// checkDeviceCall is the line between the two passes at a call: device
// code may not call a host function, and host code may not call a
// device one. It returns false when the call is refused.
func (fl *fn) checkDeviceCall(callee *sema.FuncSymbol, at ast.Tok, launch bool) bool {
	u := fl.u
	if !u.offload() || callee.Intrinsic {
		return true
	}
	if launch {
		return true
	}
	if u.devicePass() {
		if callee.Space.OnDevice() {
			return true
		}
		if callee.Space == sema.SpaceHost && callee.Body == nil && u.deviceMathVerb(callee) != "" {
			// A math function the device library provides, declared
			// by the host's <cmath> as well.
			return true
		}
		u.errorf(at, "reference to __host__ function %q in %s function %q", callee.SymName, fl.sym.Space, fl.sym.SymName)
		return false
	}
	if callee.Space == sema.SpaceDevice {
		u.errorf(at, "reference to __device__ function %q in __host__ function %q", callee.SymName, fl.sym.SymName)
		return false
	}
	if callee.Space == sema.SpaceGlobal {
		u.errorf(at, "a __global__ function is launched, not called; the <<<>>> launch is not lowered yet")
		return false
	}
	return true
}

// deviceMathVerb is the verb a device-library math function lowers to,
// or "" for one that is not lowered yet or is no math function.
func (u *unit) deviceMathVerb(callee *sema.FuncSymbol) string {
	if !u.devicePass() {
		return ""
	}
	name := callee.SymName
	if callee.LinkName != "" {
		name = callee.LinkName
	}
	if _, ok := deviceMath[name]; ok {
		return name
	}
	return ""
}

// deviceMath is the device math library's surface, by the C name, and
// how each lowers: a verb the hardware has, a short sequence, or -- for
// what needs a polynomial -- nothing yet, which the call reports.
var deviceMath = map[string]string{
	"sqrtf": "sqrt", "sqrt": "sqrt", "fabsf": "abs", "fabs": "abs",
	"floorf": "floor", "floor": "floor", "ceilf": "ceil", "ceil": "ceil",
	"truncf": "trunc", "trunc": "trunc", "rintf": "nearest", "rint": "nearest",
	"nearbyintf": "nearest", "nearbyint": "nearest", "roundf": "round", "round": "round",
	"fminf": "min", "fmin": "min", "fmaxf": "max", "fmax": "max",
	"fmaf": "fma", "fma": "fma", "copysignf": "copysign", "copysign": "copysign",
	"rsqrtf": "rsqrt", "rsqrt": "rsqrt",
	"abs": "iabs", "labs": "labs", "llabs": "labs",
	// What the header-only device library computes: a call to the C
	// name becomes a call to the __vcx_ function the wrapper defined.
	"expf": "lib", "exp": "lib", "exp2f": "lib", "exp2": "lib", "exp10f": "lib", "exp10": "lib",
	"logf": "lib", "log": "lib", "log2f": "lib", "log2": "lib", "log10f": "lib", "log10": "lib",
	"sinf": "lib", "sin": "lib", "cosf": "lib", "cos": "lib", "tanf": "lib", "tan": "lib",
	"sincosf": "lib", "sincos": "lib", "powf": "lib", "pow": "lib", "fmodf": "lib", "fmod": "lib",
	"sinhf": "lib", "sinh": "lib", "coshf": "lib", "cosh": "lib", "tanhf": "lib", "tanh": "lib",
	"atanf": "lib", "atan": "lib", "atan2f": "lib", "atan2": "lib", "asinf": "lib", "asin": "lib",
	"acosf": "lib", "acos": "lib", "ldexpf": "lib", "ldexp": "lib",
}

// deviceMathCall lowers a call to the device math library.
func (fl *fn) deviceMathCall(callee *sema.FuncSymbol, e *ast.CallExpr) ir.Value {
	b := fl.blk
	name := fl.u.deviceMathVerb(callee)
	verb := deviceMath[name]
	if verb == "lib" {
		return fl.deviceLibCall(name, callee, e)
	}
	if verb == "" {
		fl.u.errorf(e.Pos(), "%s is not in the device math library yet: the hardware has no instruction for it, and the polynomial is not written", name)
		return nil
	}
	args, ok := fl.scalarArgs(callee, e.Args)
	if !ok {
		return nil
	}
	switch verb {
	case "iabs":
		x := args[0].(ir.I32)
		return b.I32.Select(b.I32.SLt(x, b.I32.Const(0)), b.I32.Neg(x), x)
	case "labs":
		if x, is32 := args[0].(ir.I32); is32 {
			return b.I32.Select(b.I32.SLt(x, b.I32.Const(0)), b.I32.Neg(x), x)
		}
		x := args[0].(ir.I64)
		return b.I64.Select(b.I64.SLt(x, b.I64.Const(0)), b.I64.Neg(x), x)
	}
	if x, is32 := args[0].(ir.F32); is32 {
		n := b.F32
		switch verb {
		case "sqrt":
			return n.Sqrt(x)
		case "abs":
			return n.Abs(x)
		case "floor":
			return n.Floor(x)
		case "ceil":
			return n.Ceil(x)
		case "trunc":
			return n.Trunc(x)
		case "nearest":
			return n.Nearest(x)
		case "round":
			// Half away from zero: the truncation, and one more in x's
			// direction where the fraction is at least a half.
			t := n.Trunc(x)
			away := n.Add(t, n.CopySign(n.Const(1), x))
			return n.Select(n.Lt(n.Abs(n.Sub(x, t)), n.Const(0.5)), t, away)
		case "min":
			return n.MinNum(x, args[1].(ir.F32))
		case "max":
			return n.MaxNum(x, args[1].(ir.F32))
		case "fma":
			return n.FMA(x, args[1].(ir.F32), args[2].(ir.F32))
		case "copysign":
			return n.CopySign(x, args[1].(ir.F32))
		case "rsqrt":
			return n.Div(n.Const(1), n.Sqrt(x))
		}
	}
	x := args[0].(ir.F64)
	n := b.F64
	switch verb {
	case "sqrt":
		return n.Sqrt(x)
	case "abs":
		return n.Abs(x)
	case "floor":
		return n.Floor(x)
	case "ceil":
		return n.Ceil(x)
	case "trunc":
		return n.Trunc(x)
	case "nearest":
		return n.Nearest(x)
	case "round":
		t := n.Trunc(x)
		away := n.Add(t, n.CopySign(n.Const(1), x))
		return n.Select(n.Lt(n.Abs(n.Sub(x, t)), n.Const(0.5)), t, away)
	case "min":
		return n.MinNum(x, args[1].(ir.F64))
	case "max":
		return n.MaxNum(x, args[1].(ir.F64))
	case "fma":
		return n.FMA(x, args[1].(ir.F64), args[2].(ir.F64))
	case "copysign":
		return n.CopySign(x, args[1].(ir.F64))
	case "rsqrt":
		return n.Div(n.Const(1), n.Sqrt(x))
	}
	fl.u.errorf(e.Pos(), "internal: no lowering for device math %s", name)
	return nil
}

// deviceLibCall routes a C math function to the device library's
// definition of it: __vcx_<name>, a __device__ function the runtime
// wrapper defined, called with the arguments converted to the C
// function's parameter types.
func (fl *fn) deviceLibCall(name string, callee *sema.FuncSymbol, e *ast.CallExpr) ir.Value {
	lib := fl.u.deviceLib(name)
	if lib == nil {
		fl.u.errorf(e.Pos(), "%s is in the device math library, but no __vcx_%s is declared: the runtime wrapper header was not read", name, name)
		return nil
	}
	target := fl.u.callee(lib)
	if target == nil {
		fl.u.errorf(e.Pos(), "lowering has no symbol for __vcx_%s", name)
		return nil
	}
	args, ok := fl.scalarArgs(callee, e.Args)
	if !ok {
		return nil
	}
	res := fl.blk.Call(target, args...)
	if res.Len() == 0 {
		return nil
	}
	return res.Value(0)
}

// deviceLib is the device library's function for a C math name, found
// in the global scope by its __vcx_ name.
func (u *unit) deviceLib(name string) *sema.FuncSymbol {
	if u.res.GlobalScope == nil {
		return nil
	}
	for _, sym := range u.res.GlobalScope.LookupLocal("__vcx_" + name) {
		if fn, isFn := sym.(*sema.FuncSymbol); isFn && fn.Body != nil {
			return fn
		}
	}
	return nil
}

// scalarArgs evaluates a call's arguments converted to the callee's
// parameter types: what an intrinsic reads, none of which takes a class.
func (fl *fn) scalarArgs(callee *sema.FuncSymbol, argExprs []ast.Expr) ([]ir.Value, bool) {
	args := make([]ir.Value, 0, len(argExprs))
	for i, a := range argExprs {
		var want types.Type
		if i < len(callee.FuncType.Params) {
			want = callee.FuncType.Params[i].Type
		}
		v := fl.expr(a)
		if v == nil {
			return nil, false
		}
		if want != nil {
			v = fl.convert(v, fl.typeOf(a), want)
		}
		args = append(args, v)
	}
	return args, true
}

// intrinsicCall lowers a device builtin to its verb.
func (fl *fn) intrinsicCall(callee *sema.FuncSymbol, e *ast.CallExpr) ir.Value {
	if !fl.u.devicePass() {
		fl.u.errorf(e.Pos(), "%s is a device builtin, and this is the host pass: the function that calls it runs on the host", callee.SymName)
		return nil
	}
	args, ok := fl.scalarArgs(callee, e.Args)
	if !ok {
		return nil
	}
	name := callee.SymName
	if strings.HasPrefix(name, "__nvvm_") {
		return fl.nvvmCall(name, args, e)
	}
	if strings.HasPrefix(name, "__builtin_amdgcn_") {
		return fl.amdgcnCall(name, args, e)
	}
	fl.u.errorf(e.Pos(), "internal: %s is marked a device builtin and has no lowering", name)
	return nil
}

// axisOf is the axis a builtin's _x, _y or _z suffix names.
func axisOf(name string) (ir.Axis, bool) {
	switch {
	case strings.HasSuffix(name, "_x"):
		return ir.X, true
	case strings.HasSuffix(name, "_y"):
		return ir.Y, true
	case strings.HasSuffix(name, "_z"):
		return ir.Z, true
	}
	return 0, false
}

// i1Of is a builtin's int predicate as a condition.
func (fl *fn) i1Of(v ir.Value) ir.I1 {
	return fl.blk.I32.Ne(v.(ir.I32), fl.blk.I32.Const(0))
}

// nvvmCall lowers the __nvvm_* builtins.
func (fl *fn) nvvmCall(name string, args []ir.Value, e *ast.CallExpr) ir.Value {
	b := fl.blk
	base := strings.TrimPrefix(name, "__nvvm_")
	if rest, ok := strings.CutPrefix(base, "read_ptx_sreg_"); ok {
		axis, hasAxis := axisOf(rest)
		switch {
		case rest == "laneid":
			return b.I32.LaneID()
		case rest == "warpsize":
			return b.I32.WaveSize()
		case !hasAxis:
		case strings.HasPrefix(rest, "tid_"):
			return b.I32.WorkitemID(axis)
		case strings.HasPrefix(rest, "ctaid_"):
			return b.I32.WorkgroupID(axis)
		case strings.HasPrefix(rest, "ntid_"):
			return b.I32.WorkgroupSize(axis)
		case strings.HasPrefix(rest, "nctaid_"):
			return b.I32.NumWorkgroups(axis)
		}
	}
	if strings.HasPrefix(base, "atom_") {
		return fl.nvvmAtomic(base, args, e)
	}
	full := func() ir.I32 { return b.I32.Const(-1) }
	switch base {
	case "barrier0":
		b.Barrier()
		return nil
	case "bar_warp_sync":
		// The lanes of a wave run in lockstep here: there is nothing to
		// wait for.
		return nil
	case "membar_cta":
		b.Fence(ir.SeqCst, ir.FenceWorkgroup)
		return nil
	case "membar_gl":
		b.Fence(ir.SeqCst, ir.FenceDevice)
		return nil
	case "membar_sys":
		b.Fence(ir.SeqCst, ir.FenceSystem)
		return nil
	case "shfl_sync_idx_i32", "shfl_sync_up_i32", "shfl_sync_down_i32", "shfl_sync_bfly_i32",
		"shfl_sync_idx_f32", "shfl_sync_up_f32", "shfl_sync_down_f32", "shfl_sync_bfly_f32":
		// (mask, val, lane, clamp): the clamp is the header's to fold
		// into the lane, and a whole-wave shuffle ignores it.
		mask, lane := args[0].(ir.I32), args[2].(ir.I32)
		val, isFloat := args[1].(ir.F32)
		var v ir.I32
		if isFloat {
			v = b.I32.BitcastF32(val)
		} else {
			v = args[1].(ir.I32)
		}
		var r ir.I32
		switch {
		case strings.Contains(base, "_idx_"):
			r = b.I32.WaveShflIdx(v, lane, mask)
		case strings.Contains(base, "_up_"):
			r = b.I32.WaveShflUp(v, lane, mask)
		case strings.Contains(base, "_down_"):
			r = b.I32.WaveShflDown(v, lane, mask)
		default:
			r = b.I32.WaveShflXor(v, lane, mask)
		}
		if isFloat {
			return b.F32.BitcastI32(r)
		}
		return r
	case "vote_all_sync":
		return b.I32.ZExtI1(b.I1.WaveAll(fl.i1Of(args[1]), args[0].(ir.I32)))
	case "vote_any_sync":
		return b.I32.ZExtI1(b.I1.WaveAny(fl.i1Of(args[1]), args[0].(ir.I32)))
	case "vote_ballot_sync":
		return b.I32.WrapI64(b.I64.WaveBallot(fl.i1Of(args[1]), args[0].(ir.I32)))
	case "bitcast_f2i":
		return b.I32.BitcastF32(args[0].(ir.F32))
	case "bitcast_i2f":
		return b.F32.BitcastI32(args[0].(ir.I32))
	case "bitcast_d2ll":
		return b.I64.BitcastF64(args[0].(ir.F64))
	case "bitcast_ll2d":
		return b.F64.BitcastI64(args[0].(ir.I64))
	case "brev32":
		return fl.bitReverse32(args[0].(ir.I32))
	case "brev64":
		x := args[0].(ir.I64)
		lo := fl.bitReverse32(b.I32.WrapI64(x))
		hi := fl.bitReverse32(b.I32.WrapI64(b.I64.UShr(x, b.I64.Const(32))))
		return b.I64.Or(b.I64.Shl(b.I64.ZExtI32(lo), b.I64.Const(32)), b.I64.ZExtI32(hi))
	case "mul24_i", "mul24_ui":
		// The product of the low 24 bits, which is the product where
		// the operands fit them.
		return b.I32.Mul(args[0].(ir.I32), args[1].(ir.I32))
	case "mulhi_i":
		return b.I32.SMulHi(args[0].(ir.I32), args[1].(ir.I32))
	case "mulhi_ui":
		return b.I32.UMulHi(args[0].(ir.I32), args[1].(ir.I32))
	case "mulhi_ll":
		return b.I64.SMulHi(args[0].(ir.I64), args[1].(ir.I64))
	case "mulhi_ull":
		return b.I64.UMulHi(args[0].(ir.I64), args[1].(ir.I64))
	case "fshl", "fshr":
		// (hi, lo, shift): the 64-bit pair shifted, and the half wanted.
		pair := b.I64.Or(b.I64.Shl(b.I64.ZExtI32(args[0].(ir.I32)), b.I64.Const(32)), b.I64.ZExtI32(args[1].(ir.I32)))
		s := b.I64.And(b.I64.ZExtI32(args[2].(ir.I32)), b.I64.Const(31))
		if base == "fshl" {
			return b.I32.WrapI64(b.I64.UShr(b.I64.Shl(pair, s), b.I64.Const(32)))
		}
		return b.I32.WrapI64(b.I64.UShr(pair, s))
	case "sad_i":
		x, y := args[0].(ir.I32), args[1].(ir.I32)
		d := b.I32.Select(b.I32.SLt(x, y), b.I32.Sub(y, x), b.I32.Sub(x, y))
		return b.I32.Add(d, args[2].(ir.I32))
	case "sad_ui":
		x, y := args[0].(ir.I32), args[1].(ir.I32)
		d := b.I32.Select(b.I32.ULt(x, y), b.I32.Sub(y, x), b.I32.Sub(x, y))
		return b.I32.Add(d, args[2].(ir.I32))
	case "sqrt_rn_f":
		return b.F32.Sqrt(args[0].(ir.F32))
	case "sqrt_rn_d":
		return b.F64.Sqrt(args[0].(ir.F64))
	case "rsqrt_approx_f":
		return b.F32.RsqrtApprox(args[0].(ir.F32))
	case "rcp_approx_ftz_f":
		return b.F32.RcpApprox(args[0].(ir.F32))
	case "ex2_approx_f":
		return b.F32.Exp2Approx(args[0].(ir.F32))
	case "lg2_approx_f":
		return b.F32.Log2Approx(args[0].(ir.F32))
	case "sin_approx_f":
		return b.F32.SinApprox(args[0].(ir.F32))
	case "cos_approx_f":
		return b.F32.CosApprox(args[0].(ir.F32))
	case "fma_rn_f":
		return b.F32.FMA(args[0].(ir.F32), args[1].(ir.F32), args[2].(ir.F32))
	case "fma_rn_d":
		return b.F64.FMA(args[0].(ir.F64), args[1].(ir.F64), args[2].(ir.F64))
	case "f2i_rn":
		return b.I32.SCvtSatF32(b.F32.Nearest(args[0].(ir.F32)))
	case "barrier0_popc", "barrier0_and", "barrier0_or", "prmt":
		fl.u.errorf(e.Pos(), "%s is not lowered yet", name)
		return nil
	}
	_ = full
	fl.u.errorf(e.Pos(), "internal: no lowering for %s", name)
	return nil
}

// bitReverse32 reverses the bits of a word in five exchanges.
func (fl *fn) bitReverse32(x ir.I32) ir.I32 {
	b := fl.blk
	for _, step := range []struct {
		shift int64
		mask  int64
	}{{1, 0x55555555}, {2, 0x33333333}, {4, 0x0f0f0f0f}, {8, 0x00ff00ff}, {16, 0x0000ffff}} {
		s, m := b.I32.Const(step.shift), b.I32.Const(step.mask)
		lo := b.I32.And(b.I32.UShr(x, s), m)
		hi := b.I32.Shl(b.I32.And(x, m), s)
		x = b.I32.Or(lo, hi)
	}
	return x
}

// nvvmAtomic lowers __nvvm_atom_[cta_|sys_]op_gen_t: relaxed, at the
// device's scope unless narrowed or widened.
func (fl *fn) nvvmAtomic(base string, args []ir.Value, e *ast.CallExpr) ir.Value {
	b := fl.blk
	rest := strings.TrimPrefix(base, "atom_")
	scope := ir.DeviceScope
	if r, ok := strings.CutPrefix(rest, "cta_"); ok {
		scope, rest = ir.WorkgroupScope, r
	} else if r, ok := strings.CutPrefix(rest, "sys_"); ok {
		scope, rest = ir.SystemScope, r
	}
	op, typ, _ := strings.Cut(rest, "_gen_")
	p := args[0].(ir.Ptr)
	o := ir.Monotonic
	switch typ {
	case "f":
		if op == "add" {
			return b.F32.AtomicRmwAdd(args[1].(ir.F32), p, o, scope)
		}
	case "d":
		if op == "add" {
			return b.F64.AtomicRmwAdd(args[1].(ir.F64), p, o, scope)
		}
	case "i", "ui":
		n := b.I32
		v := args[1].(ir.I32)
		switch op {
		case "add":
			return n.AtomicRmwAdd(v, p, o, scope)
		case "xchg":
			return n.AtomicRmwXchg(v, p, o, scope)
		case "and":
			return n.AtomicRmwAnd(v, p, o, scope)
		case "or":
			return n.AtomicRmwOr(v, p, o, scope)
		case "xor":
			return n.AtomicRmwXor(v, p, o, scope)
		case "max":
			if typ == "ui" {
				return n.AtomicRmwUMax(v, p, o, scope)
			}
			return n.AtomicRmwSMax(v, p, o, scope)
		case "min":
			if typ == "ui" {
				return n.AtomicRmwUMin(v, p, o, scope)
			}
			return n.AtomicRmwSMin(v, p, o, scope)
		case "cas":
			return n.AtomicCas(v, args[2].(ir.I32), p, o, o, scope)
		}
	case "ll", "ull":
		n := b.I64
		v := args[1].(ir.I64)
		switch op {
		case "add":
			return n.AtomicRmwAdd(v, p, o, scope)
		case "xchg":
			return n.AtomicRmwXchg(v, p, o, scope)
		case "and":
			return n.AtomicRmwAnd(v, p, o, scope)
		case "or":
			return n.AtomicRmwOr(v, p, o, scope)
		case "xor":
			return n.AtomicRmwXor(v, p, o, scope)
		case "max":
			if typ == "ull" {
				return n.AtomicRmwUMax(v, p, o, scope)
			}
			return n.AtomicRmwSMax(v, p, o, scope)
		case "min":
			if typ == "ull" {
				return n.AtomicRmwUMin(v, p, o, scope)
			}
			return n.AtomicRmwSMin(v, p, o, scope)
		case "cas":
			return n.AtomicCas(v, args[2].(ir.I64), p, o, o, scope)
		}
	}
	fl.u.errorf(e.Pos(), "internal: no lowering for __nvvm_%s", base)
	return nil
}

// amdgcnCall lowers the __builtin_amdgcn_* builtins.
func (fl *fn) amdgcnCall(name string, args []ir.Value, e *ast.CallExpr) ir.Value {
	b := fl.blk
	base := strings.TrimPrefix(name, "__builtin_amdgcn_")
	if axis, ok := axisOf(base); ok {
		switch strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(base, "_x"), "_y"), "_z") {
		case "workitem_id":
			return b.I32.WorkitemID(axis)
		case "workgroup_id":
			return b.I32.WorkgroupID(axis)
		case "workgroup_size":
			return b.I32.WorkgroupSize(axis)
		case "grid_size":
			return b.I32.Mul(b.I32.NumWorkgroups(axis), b.I32.WorkgroupSize(axis))
		}
	}
	full := func() ir.I32 { return b.I32.Const(-1) }
	switch base {
	case "wavefrontsize":
		return b.I32.WaveSize()
	case "s_barrier":
		b.Barrier()
		return nil
	case "wave_barrier", "s_sleep":
		return nil
	case "fence":
		order, ok := fl.u.evalInt(e.Args[0])
		if ok != nil {
			fl.u.errorf(e.Args[0].Pos(), "__builtin_amdgcn_fence wants a constant ordering")
			return nil
		}
		scopeLit, isLit := unparen(e.Args[1]).(*ast.StringLit)
		if !isLit {
			fl.u.errorf(e.Args[1].Pos(), "__builtin_amdgcn_fence wants a string literal scope")
			return nil
		}
		decoded, err := literal.Decode(fl.u.unit, scopeLit)
		if err != nil {
			fl.u.errorf(e.Args[1].Pos(), "%v", err)
			return nil
		}
		scope := ir.FenceSystem
		switch string(decoded.Bytes()) {
		case "workgroup", "wavefront":
			scope = ir.FenceWorkgroup
		case "agent":
			scope = ir.FenceDevice
		}
		o := ir.SeqCst
		switch order {
		case 2:
			o = ir.Acquire
		case 3:
			o = ir.Release
		case 4:
			o = ir.AcqRel
		}
		b.Fence(o, scope)
		return nil
	case "mbcnt_lo", "mbcnt_hi":
		// base + the count of set bits of mask below this lane, in the
		// low or the high word of the wave's mask.
		mask, at := args[0].(ir.I32), args[1].(ir.I32)
		lane := b.I32.LaneID()
		var below ir.I32
		if base == "mbcnt_lo" {
			// (1 << lane) - 1, or every bit from lane 32 on.
			n := b.I32.Sub(b.I32.Shl(b.I32.Const(1), lane), b.I32.Const(1))
			below = b.I32.Select(b.I32.ULt(lane, b.I32.Const(32)), n, b.I32.Const(-1))
		} else {
			hi := b.I32.Sub(lane, b.I32.Const(32))
			n := b.I32.Sub(b.I32.Shl(b.I32.Const(1), hi), b.I32.Const(1))
			below = b.I32.Select(b.I32.ULt(lane, b.I32.Const(32)), b.I32.Const(0), n)
		}
		return b.I32.Add(at, b.I32.Popcnt(b.I32.And(mask, below)))
	case "ds_bpermute":
		// The byte address of the lane's dword.
		lane := b.I32.UShr(args[0].(ir.I32), b.I32.Const(2))
		return b.I32.WaveShflIdx(args[1].(ir.I32), lane, full())
	case "readfirstlane":
		return b.I32.WaveReadFirstLane(args[0].(ir.I32))
	case "readlane":
		return b.I32.WaveReadFirstLane(b.I32.WaveShflIdx(args[0].(ir.I32), args[1].(ir.I32), full()))
	case "uicmp":
		// (a, b, cond): the wave's mask of lanes where the comparison
		// holds; 32 is eq, 33 ne, 34 ugt, 35 uge, 36 ult, 37 ule.
		cond, err := fl.u.evalInt(e.Args[2])
		if err != nil {
			fl.u.errorf(e.Args[2].Pos(), "__builtin_amdgcn_uicmp wants a constant condition")
			return nil
		}
		x, y := args[0].(ir.I32), args[1].(ir.I32)
		var c ir.I1
		switch cond {
		case 32:
			c = b.I32.Eq(x, y)
		case 33:
			c = b.I32.Ne(x, y)
		case 34:
			c = b.I32.ULt(y, x)
		case 35:
			c = b.I1.Not(b.I32.ULt(x, y))
		case 36:
			c = b.I32.ULt(x, y)
		default:
			c = b.I1.Not(b.I32.ULt(y, x))
		}
		return b.I64.WaveBallot(c, full())
	case "read_exec":
		return b.I64.WaveBallot(b.I1.Const(true), full())
	case "sqrtf":
		return b.F32.Sqrt(args[0].(ir.F32))
	case "rsqf":
		return b.F32.RsqrtApprox(args[0].(ir.F32))
	case "rcpf":
		return b.F32.RcpApprox(args[0].(ir.F32))
	case "exp2f":
		return b.F32.Exp2Approx(args[0].(ir.F32))
	case "logf":
		return b.F32.Log2Approx(args[0].(ir.F32))
	case "sinf", "cosf":
		// v_sin_f32 takes its argument in turns.
		x := b.F32.Mul(args[0].(ir.F32), b.F32.Const(2*math.Pi))
		if base == "sinf" {
			return b.F32.SinApprox(x)
		}
		return b.F32.CosApprox(x)
	case "ds_swizzle", "s_memtime":
		fl.u.errorf(e.Pos(), "%s is not lowered yet", name)
		return nil
	}
	fl.u.errorf(e.Pos(), "internal: no lowering for %s", name)
	return nil
}

// hipAtomicCall lowers HIP's generic atomics: __hip_atomic_fetch_add(p,
// v, order, scope) and its kin, on the object p points to.
func (fl *fn) hipAtomicCall(name string, e *ast.CallExpr) (ir.Value, bool) {
	op, ok := strings.CutPrefix(name, "__hip_atomic_")
	if !ok || !fl.u.offload() {
		return nil, false
	}
	b := fl.blk
	if !fl.u.devicePass() {
		fl.u.errorf(e.Pos(), "%s is a device builtin, and this is the host pass", name)
		return nil, true
	}
	pv := fl.expr(e.Args[0])
	if pv == nil {
		return nil, true
	}
	p, isPtr := pv.(ir.Ptr)
	if !isPtr {
		return nil, true
	}
	elem := types.Unqualify(types.Unqualify(types.RemoveReference(fl.typeOf(e.Args[0]))).(*types.Pointer).Elem)
	orderOf := func(x ast.Expr) ir.Ordering {
		n, err := fl.u.evalInt(x)
		if err != nil {
			fl.u.errorf(x.Pos(), "%s wants a constant memory order", name)
			return ir.Monotonic
		}
		switch n {
		case 1, 2:
			return ir.Acquire
		case 3:
			return ir.Release
		case 4:
			return ir.AcqRel
		case 5:
			return ir.SeqCst
		}
		return ir.Monotonic
	}
	scopeOf := func(x ast.Expr) ir.MemAttr {
		n, err := fl.u.evalInt(x)
		if err != nil {
			fl.u.errorf(x.Pos(), "%s wants a constant memory scope", name)
			return ir.DeviceScope
		}
		switch n {
		case 1, 2, 3:
			return ir.WorkgroupScope
		case 5:
			return ir.SystemScope
		}
		return ir.DeviceScope
	}
	value := func(i int) ir.Value {
		v := fl.expr(e.Args[i])
		if v == nil {
			return nil
		}
		return fl.convert(v, fl.typeOf(e.Args[i]), elem)
	}
	switch op {
	case "load":
		o, s := orderOf(e.Args[1]), scopeOf(e.Args[2])
		switch fl.u.regType(elem) {
		case ir.TypeI32:
			return b.I32.AtomicLoad(p, o, s), true
		case ir.TypeI64:
			return b.I64.AtomicLoad(p, o, s), true
		case ir.TypePtr:
			return b.Ptr.AtomicLoad(p, o, s), true
		}
	case "store":
		v := value(1)
		if v == nil {
			return nil, true
		}
		o, s := orderOf(e.Args[2]), scopeOf(e.Args[3])
		switch x := v.(type) {
		case ir.I32:
			b.I32.AtomicStore(x, p, o, s)
			return nil, true
		case ir.I64:
			b.I64.AtomicStore(x, p, o, s)
			return nil, true
		case ir.Ptr:
			b.Ptr.AtomicStore(x, p, o, s)
			return nil, true
		}
	case "compare_exchange_strong", "compare_exchange_weak":
		// (p, expected*, desired, succ, fail, scope): true when the
		// exchange happened, and *expected is what was seen otherwise.
		ev := fl.expr(e.Args[1])
		if ev == nil {
			return nil, true
		}
		ep, isPtr := ev.(ir.Ptr)
		if !isPtr {
			return nil, true
		}
		desired := value(2)
		if desired == nil {
			return nil, true
		}
		succ, fail, s := orderOf(e.Args[3]), orderOf(e.Args[4]), scopeOf(e.Args[5])
		switch d := desired.(type) {
		case ir.I32:
			expect := b.I32.Load(ep)
			seen := b.I32.AtomicCas(expect, d, p, succ, fail, s)
			b.I32.Store(seen, ep)
			return b.I32.ZExtI1(b.I32.Eq(seen, expect)), true
		case ir.I64:
			expect := b.I64.Load(ep)
			seen := b.I64.AtomicCas(expect, d, p, succ, fail, s)
			b.I64.Store(seen, ep)
			return b.I32.ZExtI1(b.I64.Eq(seen, expect)), true
		}
	default:
		v := value(1)
		if v == nil {
			return nil, true
		}
		o, s := orderOf(e.Args[2]), scopeOf(e.Args[3])
		unsigned := types.IsUnsigned(elem)
		switch x := v.(type) {
		case ir.F32:
			if op == "fetch_add" {
				return b.F32.AtomicRmwAdd(x, p, o, s), true
			}
			if op == "fetch_sub" {
				return b.F32.AtomicRmwAdd(b.F32.Neg(x), p, o, s), true
			}
			if op == "exchange" {
				return b.F32.BitcastI32(b.I32.AtomicRmwXchg(b.I32.BitcastF32(x), p, o, s)), true
			}
		case ir.F64:
			if op == "fetch_add" {
				return b.F64.AtomicRmwAdd(x, p, o, s), true
			}
			if op == "fetch_sub" {
				return b.F64.AtomicRmwAdd(b.F64.Neg(x), p, o, s), true
			}
			if op == "exchange" {
				return b.F64.BitcastI64(b.I64.AtomicRmwXchg(b.I64.BitcastF64(x), p, o, s)), true
			}
		case ir.I32:
			n := b.I32
			switch op {
			case "fetch_add":
				return n.AtomicRmwAdd(x, p, o, s), true
			case "fetch_sub":
				return n.AtomicRmwSub(x, p, o, s), true
			case "fetch_and":
				return n.AtomicRmwAnd(x, p, o, s), true
			case "fetch_or":
				return n.AtomicRmwOr(x, p, o, s), true
			case "fetch_xor":
				return n.AtomicRmwXor(x, p, o, s), true
			case "exchange":
				return n.AtomicRmwXchg(x, p, o, s), true
			case "fetch_min":
				if unsigned {
					return n.AtomicRmwUMin(x, p, o, s), true
				}
				return n.AtomicRmwSMin(x, p, o, s), true
			case "fetch_max":
				if unsigned {
					return n.AtomicRmwUMax(x, p, o, s), true
				}
				return n.AtomicRmwSMax(x, p, o, s), true
			}
		case ir.I64:
			n := b.I64
			switch op {
			case "fetch_add":
				return n.AtomicRmwAdd(x, p, o, s), true
			case "fetch_sub":
				return n.AtomicRmwSub(x, p, o, s), true
			case "fetch_and":
				return n.AtomicRmwAnd(x, p, o, s), true
			case "fetch_or":
				return n.AtomicRmwOr(x, p, o, s), true
			case "fetch_xor":
				return n.AtomicRmwXor(x, p, o, s), true
			case "exchange":
				return n.AtomicRmwXchg(x, p, o, s), true
			case "fetch_min":
				if unsigned {
					return n.AtomicRmwUMin(x, p, o, s), true
				}
				return n.AtomicRmwSMin(x, p, o, s), true
			case "fetch_max":
				if unsigned {
					return n.AtomicRmwUMax(x, p, o, s), true
				}
				return n.AtomicRmwSMax(x, p, o, s), true
			}
		}
	}
	fl.u.errorf(e.Pos(), fmt.Sprintf("%s is not lowered for %s", name, elem))
	return nil, true
}

// The builtin variables. threadIdx, blockIdx, blockDim, gridDim and
// warpSize are declared by the runtime's headers as ordinary extern
// objects, the way nvcc's device_launch_parameters.h declares them, and
// are nothing of the kind: the device pass reads them from the hardware.
// threadIdx.x is the verb itself; a use of the whole object is a copy of
// the three coordinates into a temporary.

// builtinVar is the builtin variable an expression names, or "".
func (fl *fn) builtinVar(e ast.Expr) (string, *sema.VarSymbol) {
	if !fl.u.devicePass() {
		return "", nil
	}
	id, isIdent := unparen(e).(*ast.Ident)
	if !isIdent {
		return "", nil
	}
	v, isVar := fl.u.res.Info.Uses[id].(*sema.VarSymbol)
	if !isVar || v.InClass != nil || v.Memory != sema.MemDefault || v.SymScope == nil || v.SymScope.Kind != sema.GlobalScope {
		return "", nil
	}
	switch v.SymName {
	case "threadIdx", "blockIdx", "blockDim", "gridDim", "warpSize":
		return v.SymName, v
	}
	return "", nil
}

// builtinVarAxis is one coordinate of a builtin variable.
func (fl *fn) builtinVarAxis(name string, axis ir.Axis) ir.I32 {
	b := fl.blk
	switch name {
	case "threadIdx":
		return b.I32.WorkitemID(axis)
	case "blockIdx":
		return b.I32.WorkgroupID(axis)
	case "blockDim":
		return b.I32.WorkgroupSize(axis)
	case "gridDim":
		return b.I32.NumWorkgroups(axis)
	}
	return b.I32.WaveSize()
}

// builtinVarMember is threadIdx.x and its kin as the verb.
func (fl *fn) builtinVarMember(e *ast.MemberExpr) (ir.Value, bool) {
	name, _ := fl.builtinVar(e.X)
	if name == "" || name == "warpSize" || e.Op != token.PERIOD {
		return nil, false
	}
	id, isIdent := e.Sel.(*ast.Ident)
	if !isIdent {
		return nil, false
	}
	switch id.Text(fl.u.unit) {
	case "x":
		return fl.builtinVarAxis(name, ir.X), true
	case "y":
		return fl.builtinVarAxis(name, ir.Y), true
	case "z":
		return fl.builtinVarAxis(name, ir.Z), true
	}
	return nil, false
}

// builtinVarAddr is a builtin variable used whole: a temporary holding
// its value.
func (fl *fn) builtinVarAddr(v *sema.VarSymbol) (ir.Ptr, bool) {
	name := ""
	if fl.u.devicePass() && v.InClass == nil && v.Memory == sema.MemDefault && v.SymScope != nil && v.SymScope.Kind == sema.GlobalScope {
		switch v.SymName {
		case "threadIdx", "blockIdx", "blockDim", "gridDim", "warpSize":
			name = v.SymName
		}
	}
	if name == "" {
		return ir.Ptr{}, false
	}
	b := fl.blk
	tmp := fl.alloc(v.SymType, name)
	if name == "warpSize" {
		b.I32.Store(fl.builtinVarAxis(name, ir.X), tmp)
		return tmp, true
	}
	for i, axis := range []ir.Axis{ir.X, ir.Y, ir.Z} {
		b.I32.Store(fl.builtinVarAxis(name, axis), b.Ptr.Add(tmp, b.I64.Const(int64(4*i))))
	}
	return tmp, true
}
