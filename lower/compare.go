package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// Three-way comparison and the comparisons written in terms of it.
//
// The ordering classes are the runtime's (<compare>), and their value is
// one signed char: less is -1, equal 0, greater 1, and partial_ordering's
// unordered is -128. That is what the platform's header declares and what
// its hidden friends read, so an ordering built here is the byte stored
// into a temporary of the class.

// spaceship lowers `a <=> b` on scalars and constructs an ordering object.
func (fl *fn) spaceship(e *ast.BinaryExpr) ir.Value {
	rec := classOf(fl.typeOf(e))
	if rec == nil {
		fl.u.errorf(e.Pos(), "lowering: <=> that is not an ordering")
		return nil
	}
	lt, rt := fl.typeOf(e.X), fl.typeOf(e.Y)
	common := types.CommonType(lt, rt)
	if common == nil {
		common = lt
	}
	l := fl.value(e.X, lt, common)
	if l == nil {
		return nil
	}
	r := fl.value(e.Y, rt, common)
	if r == nil {
		return nil
	}
	order := fl.threeWay(l, r, common)
	if order == nil {
		fl.u.errorf(e.Pos(), "lowering does not handle <=> on these operand types yet")
		return nil
	}
	tmp := fl.alloc(rec, "")
	fl.temporary(tmp, rec)
	fl.blk.I32.Store8(*order, tmp)
	return tmp
}

// threeWay computes the ordering of two scalar values as a byte: -1, 0, 1,
// or -128 for unordered floats.
func (fl *fn) threeWay(l, r ir.Value, common types.Type) *ir.I32 {
	b := fl.blk
	signed := isSigned(common)
	var less, greater, equal ir.I1
	switch li := l.(type) {
	case ir.I32:
		ri, ok := r.(ir.I32)
		if !ok {
			return nil
		}
		if signed {
			less, greater = b.I32.SLt(li, ri), b.I32.SLt(ri, li)
		} else {
			less, greater = b.I32.ULt(li, ri), b.I32.ULt(ri, li)
		}
		equal = b.I32.Eq(li, ri)
	case ir.I64:
		ri, ok := r.(ir.I64)
		if !ok {
			return nil
		}
		if signed {
			less, greater = b.I64.SLt(li, ri), b.I64.SLt(ri, li)
		} else {
			less, greater = b.I64.ULt(li, ri), b.I64.ULt(ri, li)
		}
		equal = b.I64.Eq(li, ri)
	case ir.F64:
		ri, ok := r.(ir.F64)
		if !ok {
			return nil
		}
		less, greater, equal = b.F64.Lt(li, ri), b.F64.Lt(ri, li), b.F64.Eq(li, ri)
	case ir.F32:
		ri, ok := r.(ir.F32)
		if !ok {
			return nil
		}
		less, greater, equal = b.F32.Lt(li, ri), b.F32.Lt(ri, li), b.F32.Eq(li, ri)
	case ir.Ptr:
		ri, ok := r.(ir.Ptr)
		if !ok {
			return nil
		}
		less, greater, equal = b.Ptr.Lt(li, ri), b.Ptr.Lt(ri, li), b.Ptr.Eq(li, ri)
	default:
		return nil
	}
	// less ? -1 : greater ? 1 : equal ? 0 : unordered
	v := b.I32.Select(equal, b.I32.Const(0), b.I32.Const(-128))
	v = b.I32.Select(greater, b.I32.Const(1), v)
	v = b.I32.Select(less, b.I32.Const(-1), v)
	return &v
}

// defaultedComparison synthesizes the body of a defaulted operator== or operator<=>.
func (u *unit) defaultedComparison(sym *sema.FuncSymbol) ir.Callee {
	if f, done := u.funcs[sym]; done {
		return f
	}
	rec := sym.InClass
	friend := rec == nil
	if friend {
		rec = defaultedFriendClass(sym)
	}
	f := u.declareFunc(sym)
	// A function defaulted on its first declaration is inline, emitted as COMDAT.
	f.Comdat()
	u.funcs[sym] = f
	params := u.params[sym]
	// A declaration with no body has no parameter symbols for declareFunc
	// to declare, so the operands are declared from the signature here --
	// a class by value as the ABI passes one, a reference as an address.
	// Before the entry block, which freezes the parameter list.
	want := len(sym.FuncType.Params)
	if !friend {
		want++ // `this`
	}
	for i := len(params); i < want; i++ {
		pt := sym.FuncType.Params[i-(want-len(sym.FuncType.Params))].Type
		params = append(params, u.declareParam(f, &sema.VarSymbol{SymName: "other", SymType: pt, IsParam: true}))
	}
	u.params[sym] = params
	var this, other ir.Ptr
	if len(params) > 0 {
		this, _ = params[0].(ir.Ptr)
	}
	if len(params) > 1 {
		other, _ = params[len(params)-1].(ir.Ptr)
	}
	fl := &fn{u: u, sym: sym, f: f, slots: map[sema.Symbol]ir.Ptr{}}
	fl.entry = f.Entry()
	body := f.Block("body")
	fl.blk = body
	if p, has := u.srets[sym]; has {
		fl.sret, fl.hasSRet = p, true
	}
	fl.this, fl.hasThis = this, true
	fl.pushScope()

	spaceship := sym.SymName == "operator<=>"
	retRec := classOf(sym.FuncType.Ret)

	// A subobject's comparison: the base at an offset, or a field.
	fieldOffs := make([]int64, len(rec.Fields))
	baseOffs := make([]int64, len(rec.Bases))
	u.model.LayoutWithBases(rec, fieldOffs, baseOffs)

	fail := f.Block("differ")
	var differing ir.Ptr // for <=>: the ordering that decided
	if spaceship && retRec != nil {
		differing = fl.entry.Ptr.Alloc(1, 1).Named("__order")
	}

	compareAt := func(t types.Type, off int64) bool {
		a := this
		b := other
		if off != 0 {
			a = fl.blk.Ptr.Add(a, fl.blk.I64.Const(off))
			b = fl.blk.Ptr.Add(b, fl.blk.I64.Const(off))
		}
		return fl.compareSubobject(t, a, b, spaceship, differing, fail)
	}
	for i, b := range rec.Bases {
		if b.Virtual {
			continue
		}
		if !compareAt(b.Type, baseOffs[i]) {
			return f
		}
	}
	for i, fld := range rec.Fields {
		if !compareAt(fld.Type, fieldOffs[i]) {
			return f
		}
	}

	// Everything agreed: true, or equal.
	if spaceship {
		if fl.hasSRet {
			fl.blk.I32.Store8(fl.blk.I32.Const(0), fl.sret)
			fl.blk.Return()
		} else {
			fl.blk.Return(fl.blk.I32.Const(0))
		}
	} else {
		fl.blk.Return(fl.blk.I32.Const(1))
	}

	// Something differed: false, or the ordering that said so.
	fl.blk = fail
	if spaceship {
		if fl.hasSRet {
			fl.blk.I32.Store8(fl.blk.I32.SLoad8(differing), fl.sret)
			fl.blk.Return()
		} else {
			fl.blk.Return(fl.blk.I32.SLoad8(differing))
		}
	} else {
		fl.blk.Return(fl.blk.I32.Const(0))
	}
	fl.entry.Br(body.To())
	return f
}

// compareSubobject compares one subobject of the two objects and branches
// to fail when they differ, leaving the current block for the case that
// they agree. For <=> the ordering byte is written to differing first.
func (fl *fn) compareSubobject(t types.Type, a, b ir.Ptr, spaceship bool, differing ir.Ptr, fail *ir.Block) bool {
	if arr, isArr := types.Unqualify(t).(*types.Array); isArr {
		elemSize, _ := fl.u.sizeAlign(arr.Elem)
		for i := int64(0); i < arr.Len; i++ {
			ea, eb := a, b
			if i > 0 {
				ea = fl.blk.Ptr.Add(a, fl.blk.I64.Const(i*elemSize))
				eb = fl.blk.Ptr.Add(b, fl.blk.I64.Const(i*elemSize))
			}
			if !fl.compareSubobject(arr.Elem, ea, eb, spaceship, differing, fail) {
				return false
			}
		}
		return true
	}
	next := fl.block("agree")
	if rec := classOf(t); rec != nil {
		name := "operator=="
		if spaceship {
			name = "operator<=>"
		}
		op := fl.u.comparisonOf(rec, name)
		if op == nil {
			fl.u.errorf(fl.sym.SymPos, "cannot default %s for %s: its member of type %s has no %s", fl.sym.SymName, fl.sym.InClass.Name, rec.Name, name)
			return false
		}
		var v ir.Value
		if op.InClass != nil && !op.Static && !op.Friend {
			v = fl.invoke(op, a, []ir.Value{b}, ir.Ptr{}, fl.sym.SymPos)
		} else {
			v = fl.invoke(op, ir.Ptr{}, []ir.Value{a, b}, ir.Ptr{}, fl.sym.SymPos)
		}
		if v == nil {
			return false
		}
		if spaceship {
			// The ordering came back as an object: its byte decides.
			var order ir.I32
			if p, isPtr := v.(ir.Ptr); isPtr {
				order = fl.blk.I32.SLoad8(p)
			} else if n, isI32 := v.(ir.I32); isI32 {
				order = n
			} else {
				return false
			}
			fl.blk.I32.Store8(order, differing)
			fl.blk.BrIf(fl.blk.I32.Eq(order, fl.blk.I32.Const(0)), next.To(), fail.To())
		} else {
			fl.blk.BrIf(fl.toBool(v, op.FuncType.Ret), next.To(), fail.To())
		}
		fl.blk = next
		return true
	}
	// A scalar.
	l := fl.load(a, t)
	r := fl.load(b, t)
	if spaceship {
		order := fl.threeWay(l, r, t)
		if order == nil {
			fl.u.errorf(fl.sym.SymPos, "cannot default operator<=> for %s: a member of type %s has no three-way comparison", fl.sym.InClass.Name, t)
			return false
		}
		fl.blk.I32.Store8(*order, differing)
		fl.blk.BrIf(fl.blk.I32.Eq(*order, fl.blk.I32.Const(0)), next.To(), fail.To())
	} else {
		eq := fl.scalarEqual(l, r, t)
		if eq == nil {
			fl.u.errorf(fl.sym.SymPos, "cannot default operator== for %s: a member of type %s has no equality", fl.sym.InClass.Name, t)
			return false
		}
		fl.blk.BrIf(*eq, next.To(), fail.To())
	}
	fl.blk = next
	return true
}

// scalarEqual is the built-in == on two scalar values of a type.
func (fl *fn) scalarEqual(l, r ir.Value, t types.Type) *ir.I1 {
	b := fl.blk
	switch li := l.(type) {
	case ir.I32:
		if ri, ok := r.(ir.I32); ok {
			c := b.I32.Eq(li, ri)
			return &c
		}
	case ir.I64:
		if ri, ok := r.(ir.I64); ok {
			c := b.I64.Eq(li, ri)
			return &c
		}
	case ir.F64:
		if ri, ok := r.(ir.F64); ok {
			c := b.F64.Eq(li, ri)
			return &c
		}
	case ir.F32:
		if ri, ok := r.(ir.F32); ok {
			c := b.F32.Eq(li, ri)
			return &c
		}
	case ir.Ptr:
		if ri, ok := r.(ir.Ptr); ok {
			c := b.Ptr.Eq(li, ri)
			return &c
		}
	case ir.I1:
		if ri, ok := r.(ir.I1); ok {
			c := b.I1.Not(b.I1.Xor(li, ri))
			return &c
		}
	}
	return nil
}

// comparisonOf is a class's operator== or operator<=> taking one of its
// own: a member with one parameter, or a hidden friend with two.
func (u *unit) comparisonOf(rec *types.Record, name string) *sema.FuncSymbol {
	for _, m := range rec.Methods {
		if m.Name != name {
			continue
		}
		want := 1
		if m.Friend || m.Static {
			want = 2
		}
		if len(m.Func.Params) != want {
			continue
		}
		if sym := u.declaredFor(&sema.FuncSymbol{SymName: m.Name, FuncType: m.Func, InClass: rec}); sym != nil {
			return sym
		}
	}
	return nil
}

// defaultedFriendClass returns the class compared by a defaulted friend comparison.
func defaultedFriendClass(sym *sema.FuncSymbol) *types.Record {
	if sym.FuncType == nil || len(sym.FuncType.Params) != 2 {
		return nil
	}
	return classOf(types.RemoveReference(sym.FuncType.Params[0].Type))
}
