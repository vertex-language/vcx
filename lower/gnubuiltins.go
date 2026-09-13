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

	case "constant_p":
		// Not evaluated, as gcc does not: a question about the operand's
		// form, whose answer at run time is always no.
		return b.I32.Const(0), true

	case "trap", "verbose_trap":
		b.Trap()
		return nil, true

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
