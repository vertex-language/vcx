package lower

import (
	"math"
	"strings"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/types"
)

// The GNU expression builtins sema admitted (see sema.gnuBuiltinCall):
// what gcc and clang compute inline rather than call a library for.
//
// The floating classifications widen their operand to double, which keeps
// every property they ask about -- a NaN, an infinity and a sign survive
// the widening exactly -- except where a value is normal, which depends on
// the type it started in: a float's smallest normal value is a double's
// normal too. So that one question is asked against the operand's own
// threshold.
func (fl *fn) gnuBuiltinCall(name string, e *ast.CallExpr) (ir.Value, bool) {
	base, ok := strings.CutPrefix(name, "__builtin_")
	if !ok {
		return nil, false
	}
	b := fl.blk
	dbl := types.Typ(types.Double)
	toInt := func(c ir.I1) ir.Value { return b.I32.ZExtI1(c) }
	floatArg := func(i int) (ir.F64, types.Type, bool) {
		if i >= len(e.Args) {
			return ir.F64{}, nil, false
		}
		t := types.Unqualify(types.RemoveReference(fl.typeOf(e.Args[i])))
		v := fl.expr(e.Args[i])
		if v == nil {
			return ir.F64{}, nil, false
		}
		f, isF64 := fl.convert(v, t, dbl).(ir.F64)
		return f, t, isF64
	}
	inf := func() ir.F64 { return b.F64.Const(math.Inf(1)) }
	minNormal := func(t types.Type) float64 {
		if bt, isBasic := t.(*types.Basic); isBasic && bt.K == types.Float {
			return float64(math.SmallestNonzeroFloat32 * (1 << 23)) // FLT_MIN
		}
		return 0x1p-1022 // DBL_MIN
	}

	switch base {
	case "isnan", "isinf", "isfinite", "isnormal", "signbit":
		x, t, ok := floatArg(0)
		if !ok {
			return nil, true
		}
		switch base {
		case "isnan":
			return toInt(b.F64.Uno(x, x)), true
		case "isinf":
			return toInt(b.F64.Eq(b.F64.Abs(x), inf())), true
		case "isfinite":
			return toInt(b.F64.Lt(b.F64.Abs(x), inf())), true
		case "isnormal":
			a := b.F64.Abs(x)
			return toInt(b.I1.And(b.F64.Le(b.F64.Const(minNormal(t)), a), b.F64.Lt(a, inf()))), true
		default:
			return toInt(b.I64.SLt(b.I64.BitcastF64(x), b.I64.Const(0))), true
		}

	case "fpclassify":
		if len(e.Args) != 6 {
			return nil, true
		}
		var classes [5]ir.I32
		for i := 0; i < 5; i++ {
			c, _ := fl.convert(fl.expr(e.Args[i]), fl.typeOf(e.Args[i]), types.Typ(types.Int)).(ir.I32)
			classes[i] = c
		}
		x, t, ok := floatArg(5)
		if !ok {
			return nil, true
		}
		a := b.F64.Abs(x)
		r := b.I32.Select(b.F64.Eq(a, b.F64.Const(0)), classes[4], classes[3])
		r = b.I32.Select(b.F64.Le(b.F64.Const(minNormal(t)), a), classes[2], r)
		r = b.I32.Select(b.F64.Eq(a, inf()), classes[1], r)
		return b.I32.Select(b.F64.Uno(x, x), classes[0], r), true

	case "isgreater", "isgreaterequal", "isless", "islessequal", "islessgreater", "isunordered":
		x, _, ok1 := floatArg(0)
		y, _, ok2 := floatArg(1)
		if !ok1 || !ok2 {
			return nil, true
		}
		switch base {
		case "isgreater":
			return toInt(b.F64.Lt(y, x)), true
		case "isgreaterequal":
			return toInt(b.F64.Le(y, x)), true
		case "isless":
			return toInt(b.F64.Lt(x, y)), true
		case "islessequal":
			return toInt(b.F64.Le(x, y)), true
		case "islessgreater":
			return toInt(b.I1.Or(b.F64.Lt(x, y), b.F64.Lt(y, x))), true
		default:
			return toInt(b.F64.Uno(x, y)), true
		}

	case "huge_val", "huge_valf", "huge_vall", "inf", "inff", "infl",
		"nan", "nanf", "nanl", "nans", "nansf", "nansl":
		v := b.F64.Const(math.Inf(1))
		switch {
		case strings.HasPrefix(base, "nans"):
			v = b.F64.BitcastI64(b.I64.Const(0x7FF4000000000000))
		case strings.HasPrefix(base, "nan"):
			v = b.F64.Const(math.NaN())
		}
		return fl.convert(v, dbl, fl.typeOf(e)), true

	case "clz", "clzl", "clzll", "clzg", "ctz", "ctzl", "ctzll", "ctzg",
		"popcount", "popcountl", "popcountll", "popcountg", "parity", "parityl", "parityll":
		return fl.bitCount(base, e), true

	case "bitreverse8", "bitreverse16", "bitreverse32", "bitreverse64":
		return fl.bitReverse(base, e), true

	case "bswap16", "bswap32", "bswap64":
		if len(e.Args) != 1 {
			return nil, true
		}
		v := fl.expr(e.Args[0])
		switch base {
		case "bswap64":
			x, _ := fl.convert(v, fl.typeOf(e.Args[0]), types.Typ(types.ULongLong)).(ir.I64)
			return b.I64.Bswap(x), true
		case "bswap32":
			x, _ := fl.convert(v, fl.typeOf(e.Args[0]), types.Typ(types.UInt)).(ir.I32)
			return b.I32.Bswap(x), true
		default:
			x, _ := fl.convert(v, fl.typeOf(e.Args[0]), types.Typ(types.UInt)).(ir.I32)
			return b.I32.UShr(b.I32.Bswap(x), b.I32.Const(16)), true
		}

	case "add_overflow", "sub_overflow", "mul_overflow":
		return fl.overflow(base, e), true

	case "expect", "expect_with_probability":
		if len(e.Args) == 0 {
			return nil, true
		}
		for _, arg := range e.Args[1:] {
			fl.expr(arg)
		}
		return fl.convert(fl.expr(e.Args[0]), fl.typeOf(e.Args[0]), types.Typ(types.Long)), true

	case "return_address", "frame_address":
		// The level is taken to be 0, this function's own: the only one
		// VIR can say (§D3). GCC requires a constant there, and callers
		// that walk outward do it through a runtime's unwinder.
		if base == "frame_address" {
			return b.Ptr.FrameAddr(), true
		}
		return b.Ptr.ReturnAddr(), true

	case "constant_p":
		// Not evaluated, as gcc does not: a question about the operand's
		// form, whose answer at run time is always no.
		return b.I32.Const(0), true

	case "trap", "verbose_trap", "unreachable":
		b.Trap()
		// A trap ends its block, but it is an expression and what follows
		// it in the source is still lowered. That goes into a block nothing
		// branches to, which is what code after a call that cannot return
		// is.
		fl.blk = fl.block("after_trap")
		return nil, true

	case "atomic_load", "atomic_store", "atomic_add", "atomic_sub", "atomic_xchg", "atomic_cas":
		return fl.atomic(base, e), true

	case "launder", "assume_aligned":
		if len(e.Args) == 0 {
			return nil, true
		}
		for _, arg := range e.Args[1:] {
			fl.expr(arg)
		}
		return fl.expr(e.Args[0]), true
	}
	return nil, false
}

// bitReverse is __builtin_bitreverse8/16/32/64: the bits in the opposite
// order, within the width the name gives.
//
// The 32-bit shape does the whole job, since a narrower width is that one
// with the result brought back down, and a 64-bit one is the two halves
// reversed and swapped.
func (fl *fn) bitReverse(base string, e *ast.CallExpr) ir.Value {
	if len(e.Args) != 1 {
		return nil
	}
	b := fl.blk
	v := fl.expr(e.Args[0])
	if v == nil {
		return nil
	}
	if base == "bitreverse64" {
		x, isI64 := fl.convert(v, fl.typeOf(e.Args[0]), types.Typ(types.ULongLong)).(ir.I64)
		if !isI64 {
			return nil
		}
		lo := fl.bitReverse32(b.I32.WrapI64(x))
		hi := fl.bitReverse32(b.I32.WrapI64(b.I64.UShr(x, b.I64.Const(32))))
		return b.I64.Or(b.I64.Shl(b.I64.ZExtI32(lo), b.I64.Const(32)), b.I64.ZExtI32(hi))
	}
	x, isI32 := fl.convert(v, fl.typeOf(e.Args[0]), types.Typ(types.UInt)).(ir.I32)
	if !isI32 {
		return nil
	}
	r := fl.bitReverse32(x)
	switch base {
	case "bitreverse8":
		return b.I32.UShr(r, b.I32.Const(24))
	case "bitreverse16":
		return b.I32.UShr(r, b.I32.Const(16))
	}
	return r
}

// bitCount is clz, ctz, popcount and parity in their int, long, long long
// and type-generic forms. The g forms take the operand's own type and an
// optional value for a zero operand; the others are undefined on zero for
// clz and ctz, and the IR's defined answer, the width, is as good as any.
func (fl *fn) bitCount(base string, e *ast.CallExpr) ir.Value {
	if len(e.Args) == 0 {
		return nil
	}
	b := fl.blk
	var opT types.Type = types.Typ(types.UInt)
	switch {
	case strings.HasSuffix(base, "ll"):
		opT = types.Typ(types.ULongLong)
	case strings.HasSuffix(base, "l"):
		opT = types.Typ(types.ULong)
	case strings.HasSuffix(base, "g"):
		opT = types.Unqualify(types.RemoveReference(fl.typeOf(e.Args[0])))
	}
	op := strings.TrimRight(strings.TrimSuffix(base, "g"), "l")
	v := fl.convert(fl.expr(e.Args[0]), fl.typeOf(e.Args[0]), opT)
	intT := types.Typ(types.Int)
	var r ir.Value
	var zero ir.I1
	switch x := v.(type) {
	case ir.I32:
		bits, _ := fl.u.sizeAlign(opT)
		zero = b.I32.Eq(x, b.I32.Const(0))
		switch op {
		case "clz":
			r = b.I32.Sub(b.I32.Clz(x), b.I32.Const(32-int64(bits*8)))
		case "ctz":
			r = b.I32.Ctz(x)
		case "popcount":
			r = b.I32.Popcnt(x)
		case "parity":
			r = b.I32.And(b.I32.Popcnt(x), b.I32.Const(1))
		}
	case ir.I64:
		zero = b.I64.Eq(x, b.I64.Const(0))
		var w ir.I64
		switch op {
		case "clz":
			w = b.I64.Clz(x)
		case "ctz":
			w = b.I64.Ctz(x)
		case "popcount":
			w = b.I64.Popcnt(x)
		case "parity":
			w = b.I64.And(b.I64.Popcnt(x), b.I64.Const(1))
		}
		r = b.I32.WrapI64(w)
	default:
		fl.u.errorf(e.Pos(), "lowering: __builtin_%s of this type is not handled", base)
		return nil
	}
	if strings.HasSuffix(base, "g") && len(e.Args) == 2 && (op == "clz" || op == "ctz") {
		fallback, _ := fl.convert(fl.expr(e.Args[1]), fl.typeOf(e.Args[1]), intT).(ir.I32)
		if rr, isI32 := r.(ir.I32); isI32 {
			return b.I32.Select(zero, fallback, rr)
		}
	}
	return r
}

// overflow is add_overflow, sub_overflow and mul_overflow: the wrapped
// result stored through the third operand, and whether it wrapped. The
// operands are brought to the result's type first, which is exact whenever
// they fit in it -- the case every use in a library header has.
func (fl *fn) overflow(base string, e *ast.CallExpr) ir.Value {
	if len(e.Args) != 3 {
		return nil
	}
	b := fl.blk
	resPtrT, isPtr := types.Unqualify(fl.typeOf(e.Args[2])).(*types.Pointer)
	if !isPtr {
		return nil
	}
	t := types.Unqualify(resPtrT.Elem)
	signed := isSigned(t)
	x := fl.convert(fl.expr(e.Args[0]), fl.typeOf(e.Args[0]), t)
	y := fl.convert(fl.expr(e.Args[1]), fl.typeOf(e.Args[1]), t)
	dst, _ := fl.expr(e.Args[2]).(ir.Ptr)
	var flag ir.I1
	var r ir.Value
	switch xv := x.(type) {
	case ir.I32:
		yv, _ := y.(ir.I32)
		switch base {
		case "add_overflow":
			r = b.I32.Add(xv, yv)
			flag = b.I32.UAddO(xv, yv)
			if signed {
				flag = b.I32.SAddO(xv, yv)
			}
		case "sub_overflow":
			r = b.I32.Sub(xv, yv)
			flag = b.I32.ULt(xv, yv)
			if signed {
				flag = b.I32.SSubO(xv, yv)
			}
		default:
			r = b.I32.Mul(xv, yv)
			flag = b.I32.UMulO(xv, yv)
			if signed {
				flag = b.I32.SMulO(xv, yv)
			}
		}
	case ir.I64:
		yv, _ := y.(ir.I64)
		switch base {
		case "add_overflow":
			r = b.I64.Add(xv, yv)
			flag = b.I64.UAddO(xv, yv)
			if signed {
				flag = b.I64.SAddO(xv, yv)
			}
		case "sub_overflow":
			r = b.I64.Sub(xv, yv)
			flag = b.I64.ULt(xv, yv)
			if signed {
				flag = b.I64.SSubO(xv, yv)
			}
		default:
			r = b.I64.Mul(xv, yv)
			flag = b.I64.UMulO(xv, yv)
			if signed {
				flag = b.I64.SMulO(xv, yv)
			}
		}
	default:
		fl.u.errorf(e.Pos(), "lowering: __builtin_%s on this type is not handled", base)
		return nil
	}
	fl.store(dst, r, t)
	return flag
}

// atomic is vcx's atomic builtins: load, store, and the read-modify-writes
// that answer with the value the object held before them. They are the
// instructions VIR has, at sequential consistency, which is the ordering
// std::atomic promises and the one that needs no argument about which
// weaker one a caller could have got away with.
//
// The object is a 32- or 64-bit integer or a pointer; a pointer is
// operated on as the 64-bit integer of its address, which is the width
// every target this compiler has gives one.
func (fl *fn) atomic(base string, e *ast.CallExpr) ir.Value {
	b := fl.blk
	pt, isPtr := types.Unqualify(types.RemoveReference(fl.typeOf(e.Args[0]))).(*types.Pointer)
	if !isPtr {
		return nil
	}
	t := types.Unqualify(pt.Elem)
	dst, _ := fl.expr(e.Args[0]).(ir.Ptr)
	arg := func(i int) ir.Value {
		return fl.convert(fl.expr(e.Args[i]), fl.typeOf(e.Args[i]), t)
	}
	const o = ir.SeqCst
	_, isPointer := t.(*types.Pointer)
	size, _ := fl.u.sizeAlign(t)
	switch {
	case isPointer || size == 8:
		i64 := func(v ir.Value) ir.I64 {
			switch x := v.(type) {
			case ir.I64:
				return x
			case ir.Ptr:
				return b.I64.FromPtr(x)
			}
			return b.I64.Const(0)
		}
		back := func(v ir.I64) ir.Value {
			if isPointer {
				return b.Ptr.FromI64(v)
			}
			return v
		}
		switch base {
		case "atomic_load":
			return back(b.I64.AtomicLoad(dst, o))
		case "atomic_store":
			b.I64.AtomicStore(i64(arg(1)), dst, o)
			return nil
		case "atomic_add":
			return back(b.I64.AtomicRmwAdd(i64(arg(1)), dst, o))
		case "atomic_sub":
			return back(b.I64.AtomicRmwSub(i64(arg(1)), dst, o))
		case "atomic_xchg":
			return back(b.I64.AtomicRmwXchg(i64(arg(1)), dst, o))
		case "atomic_cas":
			return back(b.I64.AtomicCas(i64(arg(1)), i64(arg(2)), dst, o, o))
		}
	case size == 4:
		i32 := func(v ir.Value) ir.I32 { x, _ := v.(ir.I32); return x }
		switch base {
		case "atomic_load":
			return b.I32.AtomicLoad(dst, o)
		case "atomic_store":
			b.I32.AtomicStore(i32(arg(1)), dst, o)
			return nil
		case "atomic_add":
			return b.I32.AtomicRmwAdd(i32(arg(1)), dst, o)
		case "atomic_sub":
			return b.I32.AtomicRmwSub(i32(arg(1)), dst, o)
		case "atomic_xchg":
			return b.I32.AtomicRmwXchg(i32(arg(1)), dst, o)
		case "atomic_cas":
			return b.I32.AtomicCas(i32(arg(1)), i32(arg(2)), dst, o, o)
		}
	}
	fl.u.errorf(e.Pos(), "lowering: __builtin_%s on this type is not handled", base)
	return nil
}
