package lower

import (
	"fmt"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// A class in the object file.
//
// In VIR, class-typed values are represented by addresses. Lowering describes
// the class as a named struct type for backend ABI classification. Classes with
// user-provided copy constructors, destructors, or virtual functions are not
// passed by value at machine level; they are passed via pointer.

// recordType is the IR struct type describing a class's storage: every
// base and field at the offset the layout gave it, filler where the
// layout left gaps, and the size and alignment the layout computed.
func (u *unit) recordType(rec *types.Record) *ir.Type {
	if t, done := u.recordTypes[rec]; done {
		return t
	}
	u.ntypes++
	t := u.mod.Struct(fmt.Sprintf("%s_%d", identOf(rec.Name), u.ntypes))
	u.recordTypes[rec] = t

	fieldOffs := make([]int64, len(rec.Fields))
	baseOffs := make([]int64, len(rec.Bases))
	u.model.LayoutWithBases(rec, fieldOffs, baseOffs)
	size, align := u.sizeAlign(rec)

	type piece struct {
		off  int64
		size int64
		ft   ir.FType
		name string
	}
	var pieces []piece

	if u.model.VTables(rec) != nil && !u.baseSuppliesVPtr(rec) {
		pieces = append(pieces, piece{0, u.model.SizePtr, ir.StorePtr.FType(), "vfptr"})
	}
	for i, b := range rec.Bases {
		br, isRec := types.Unqualify(b.Type).(*types.Record)
		if !isRec || b.Virtual {
			continue
		}
		bsize, _ := u.sizeAlign(br)
		pieces = append(pieces, piece{baseOffs[i], bsize, u.recordType(br).FType(), "base_" + identOf(br.Name)})
	}
	for i, f := range rec.Fields {
		if f.BitField {
			// A bit-field's storage unit is bytes to the backend; several
			// share one and only the first reaches the list.
			continue
		}
		fsize, _ := u.sizeAlign(f.Type)
		pieces = append(pieces, piece{fieldOffs[i], fsize, u.storageType(f.Type), identOf(f.Name)})
	}

	// In offset order, with filler for whatever the layout skipped --
	// alignment padding, bit-field units, the byte an empty base takes.
	for i := 1; i < len(pieces); i++ {
		for j := i; j > 0 && pieces[j].off < pieces[j-1].off; j-- {
			pieces[j], pieces[j-1] = pieces[j-1], pieces[j]
		}
	}
	var end int64
	fillers := 0
	for _, p := range pieces {
		if p.off < end {
			continue // a union member, or an empty base sharing an address
		}
		if p.off > end {
			fillers++
			t.FieldAt(fmt.Sprintf("pad_%d", fillers), ir.Array(uint64(p.off-end), ir.StoreI8.FType()), uint64(end))
		}
		t.FieldAt(p.name, p.ft, uint64(p.off))
		end = p.off + p.size
	}
	if end < size {
		fillers++
		t.FieldAt(fmt.Sprintf("pad_%d", fillers), ir.Array(uint64(size-end), ir.StoreI8.FType()), uint64(end))
	}
	t.Align(uint64(align))
	return t
}

// baseSuppliesVPtr reports whether a polymorphic class's table pointer
// comes from a base rather than being its own first member.
func (u *unit) baseSuppliesVPtr(rec *types.Record) bool {
	for _, b := range rec.Bases {
		if br, isRec := types.Unqualify(b.Type).(*types.Record); isRec && !b.Virtual && u.model.VTables(br) != nil {
			return true
		}
	}
	return false
}

// storageType is the store-type of a field: a scalar's own, a class's
// struct, an array of either.
func (u *unit) storageType(t types.Type) ir.FType {
	switch t := types.Unqualify(t).(type) {
	case *types.Record:
		return u.recordType(t).FType()
	case *types.Array:
		if t.Incomplete {
			return ir.Array(0, u.storageType(t.Elem))
		}
		return ir.Array(uint64(t.Len), u.storageType(t.Elem))
	}
	size, _ := u.sizeAlign(t)
	return u.ftype(t, size)
}

// plainForCalls is whether a class travels through a call as bytes -- in
// a register or as a copy the ABI makes -- rather than as a pointer to an
// object the caller constructed.
//
// The Itanium ABI calls the other kind "non-trivial for the purposes of
// calls": a user-provided copy or move constructor, or a user-provided
// destructor. Microsoft's rule for passing is the same one; its rule for
// returning is narrower, and is plainForReturn.
func (u *unit) plainForCalls(rec *types.Record) bool {
	for _, m := range rec.Methods {
		if m.Defaulted {
			continue
		}
		switch {
		case m.Name == "~"+rec.Name:
			return false
		case m.Name == rec.Name && len(m.Func.Params) == 1 && isReferenceTo(m.Func.Params[0].Type, rec):
			return false // a copy or move constructor
		}
	}
	for _, b := range rec.Bases {
		if br, isRec := types.Unqualify(b.Type).(*types.Record); isRec && !u.plainForCalls(br) {
			return false
		}
	}
	for _, f := range rec.Fields {
		if fr := classOf(f.Type); fr != nil && !u.plainForCalls(fr) {
			return false
		}
	}
	return true
}

// plainForReturn is whether a class comes back from a call as bytes,
// which the backend may put in a register. Under the Itanium ABI that is
// the same question as plainForCalls. Under Microsoft's it is narrower:
// a class with any user-declared constructor, a virtual function or a
// base class is returned through a hidden pointer whatever its size, and
// the pointer is not `sret` -- it is an ordinary parameter, after `this`.
func (u *unit) plainForReturn(rec *types.Record) bool {
	if !u.plainForCalls(rec) {
		return false
	}
	if !u.model.ABI.IsMicrosoft() {
		return true
	}
	if len(rec.Bases) > 0 || u.model.VTables(rec) != nil {
		return false
	}
	for _, m := range rec.Methods {
		if !m.Defaulted && m.Name == rec.Name {
			return false
		}
	}
	for _, f := range rec.Fields {
		if fr := classOf(f.Type); fr != nil && !u.plainForReturn(fr) {
			return false
		}
	}
	return true
}

func isReferenceTo(t types.Type, rec *types.Record) bool {
	return types.AsRecord(types.Unqualify(types.RemoveReference(t))) == rec && isReference(t)
}

// copyObject copies src into dst using the class copy constructor if present,
// memberwise copying if needed, or byte copy otherwise.
func (fl *fn) copyObject(dst, src ir.Ptr, rec *types.Record, at ir.Value) bool {
	if ctor := fl.u.copyConstructor(rec); ctor != nil {
		target := fl.u.callee(ctor)
		if target == nil {
			fl.u.errorf(fl.sym.SymPos, "lowering has no symbol for the copy constructor of %s", rec.Name)
			return false
		}
		fl.blk.Call(target, dst, src)
		return true
	}
	if fl.u.memberwise(rec, true) {
		// Call implicit copy constructor for memberwise copy.
		fl.blk.Call(fl.u.implicitCopy(rec, true), dst, src)
		return true
	}
	return fl.copyObjectBytes(dst, src, rec)
}

// copyObjectBytes copies an object's storage: what a prvalue's direct
// initialization comes to once the temporary and the object are one, and
// what the implicit copy constructor of a plain class does.
func (fl *fn) copyObjectBytes(dst, src ir.Ptr, rec *types.Record) bool {
	size, _ := fl.u.sizeAlign(rec)
	fl.blk.MemCpy(dst, src, fl.blk.I64.Const(size))
	return true
}

// copyConstructor is a class's user-provided copy constructor, or nil.
func (u *unit) copyConstructor(rec *types.Record) *sema.FuncSymbol {
	for _, m := range rec.Methods {
		if !m.Defaulted && !m.Template && m.Name == rec.Name && len(m.Func.Params) == 1 && isReferenceTo(m.Func.Params[0].Type, rec) {
			if _, isRValue := m.Func.Params[0].Type.(*types.RValueReference); isRValue {
				continue
			}
			if declared := u.declaredFor(&sema.FuncSymbol{SymName: m.Name, FuncType: m.Func, InClass: rec}); declared != nil {
				return declared
			}
			return &sema.FuncSymbol{SymName: m.Name, FuncType: m.Func, InClass: rec}
		}
	}
	return nil
}

// needsConstruction is whether default-initializing a class does
// anything: a constructor of its own, a table pointer, a member with an
// initializer, or a base or member that needs it.
func (u *unit) needsConstruction(rec *types.Record) bool {
	for _, m := range rec.Methods {
		if m.Name == rec.Name && !m.Defaulted {
			return true
		}
	}
	if u.model.VTables(rec) != nil {
		return true
	}
	for _, b := range rec.Bases {
		if br := classOf(b.Type); br != nil && u.needsConstruction(br) {
			return true
		}
	}
	for _, f := range rec.Fields {
		if f.HasInit {
			return true
		}
		if fr := classOf(elementOf(f.Type)); fr != nil && u.needsConstruction(fr) {
			return true
		}
	}
	return false
}

// hasUserCopy reports a user-provided copy constructor anywhere in the
// class -- its own, a base's, a member's.
func (u *unit) hasUserCopy(rec *types.Record) bool {
	for _, m := range rec.Methods {
		if !m.Defaulted && !m.Template && m.Name == rec.Name && len(m.Func.Params) == 1 && isReferenceTo(m.Func.Params[0].Type, rec) {
			return true
		}
	}
	for _, b := range rec.Bases {
		if br, isRec := types.Unqualify(b.Type).(*types.Record); isRec && u.hasUserCopy(br) {
			return true
		}
	}
	for _, f := range rec.Fields {
		if fr := classOf(f.Type); fr != nil && u.hasUserCopy(fr) {
			return true
		}
	}
	return false
}

// objectOf is the address of a class-typed expression: the object an
// lvalue denotes, or the temporary a prvalue was built in.
func (fl *fn) objectOf(e ast.Expr) (ir.Ptr, bool) {
	if addr, _, ok := fl.lvalueQuiet(e); ok {
		return addr, true
	}
	v := fl.expr(e)
	if v == nil {
		return ir.Ptr{}, false
	}
	p, isPtr := v.(ir.Ptr)
	if !isPtr {
		fl.u.errorf(fl.sym.SymPos, "lowering: a class-typed expression that is not an address")
		return ir.Ptr{}, false
	}
	return p, true
}

// exprInto initializes the object at dst from a class-typed expression.
//
// A prvalue is constructed directly into dst (copy elision). An lvalue is copied
// using copyObject.
func (fl *fn) exprInto(dst ir.Ptr, e ast.Expr, rec *types.Record) bool {
	// If the analysis chose a converting constructor, build the object with it.
	if ctor := fl.u.res.Info.Conversions[e]; ctor != nil {
		fl.constructWith(dst, ctor, []ast.Expr{e}, e.Pos())
		return true
	}
	switch x := unparen(e).(type) {
	case *ast.InitList:
		fl.initList(dst, rec, x)
		return true
	case *ast.CondExpr:
		// Whichever branch is chosen constructs into dst.
		c := fl.truth(x.Cond)
		if c == nil {
			return false
		}
		then := fl.block("cond_then")
		els := fl.block("cond_else")
		join := fl.block("cond_join")
		fl.blk.BrIf(*c, then.To(), els.To())
		fl.blk = then
		ok := fl.exprInto(dst, x.Then, rec)
		if fl.blk != nil {
			fl.blk.Br(join.To())
		}
		fl.blk = els
		ok = fl.exprInto(dst, x.Else, rec) && ok
		if fl.blk != nil {
			fl.blk.Br(join.To())
		}
		fl.blk = join
		return ok
	case *ast.CallExpr:
		if ctor := fl.u.res.Info.Temporaries[x]; ctor != nil {
			fl.constructObject(dst, rec, ctor, x.Args, x.Pos())
			return true
		}
		if op := fl.u.res.Info.Operators[x]; op != nil && classOf(op.FuncType.Ret) == rec {
			fl.resultInto = dst
			v := fl.operatorRaw(x, op, x.Fun, x.Args)
			fl.resultInto = ir.Ptr{}
			return v != nil
		}
		if callee := fl.u.res.Info.Calls[x]; callee != nil && classOf(callee.FuncType.Ret) == rec {
			fl.resultInto = dst
			v := fl.callRaw(x, callee)
			fl.resultInto = ir.Ptr{}
			return v != nil
		}
	case *ast.BinaryExpr:
		if op := fl.u.res.Info.Operators[x]; op != nil && classOf(op.FuncType.Ret) == rec {
			fl.resultInto = dst
			v := fl.operatorRaw(x, op, x.X, []ast.Expr{x.Y})
			fl.resultInto = ir.Ptr{}
			return v != nil
		}
	case *ast.UnaryExpr:
		if op := fl.u.res.Info.Operators[x]; op != nil && classOf(op.FuncType.Ret) == rec {
			fl.resultInto = dst
			v := fl.operatorRaw(x, op, x.X, nil)
			fl.resultInto = ir.Ptr{}
			return v != nil
		}
	}
	src, ok := fl.objectOf(e)
	if !ok {
		return false
	}
	if fl.isLValueShape(e) {
		return fl.copyObject(dst, src, rec, nil)
	}
	return fl.copyObjectBytes(dst, src, rec)
}

// classOf is the class a type is a value of, and nil for anything else --
// a reference to a class in particular, which is an address and travels
// as one. Asking types.AsRecord, which sees through references, was how a
// `Box<int> &` parameter came to be passed by value.
func classOf(t types.Type) *types.Record {
	if isReference(t) {
		return nil
	}
	return types.AsRecord(types.Unqualify(t))
}
