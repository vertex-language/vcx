package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// Lowering of implicit copy constructors and copy assignment operators.
// Copies member by member when subobjects require non-trivial copying,
// or via direct memory copy when trivial.

// copyAssignment is a class's user-provided copy assignment operator, or
// nil: `T &operator=(const T &)` or `T &operator=(T)`.
func (u *unit) copyAssignment(rec *types.Record) *sema.FuncSymbol {
	for _, m := range rec.Methods {
		if m.Defaulted || m.Template || m.Name != "operator=" || len(m.Func.Params) != 1 {
			continue
		}
		p := m.Func.Params[0].Type
		if _, isRValue := p.(*types.RValueReference); isRValue {
			continue
		}
		if isReferenceTo(p, rec) || classOf(p) == rec {
			return u.declaredFor(&sema.FuncSymbol{SymName: m.Name, FuncType: m.Func, InClass: rec})
		}
	}
	return nil
}

// memberwise reports whether copying an object of rec (constructing
// when ctor, assigning otherwise) has to go member by member: something
// below it has a user-provided copy of that kind, or a virtual base.
func (u *unit) memberwise(rec *types.Record, ctor bool) bool {
	for _, b := range rec.Bases {
		br := classOf(b.Type)
		if br == nil {
			continue
		}
		if b.Virtual || u.userCopy(br, ctor) || u.memberwise(br, ctor) {
			return true
		}
	}
	for _, f := range rec.Fields {
		if fr := classOf(elementOf(f.Type)); fr != nil && (u.userCopy(fr, ctor) || u.memberwise(fr, ctor)) {
			return true
		}
	}
	return false
}

// userCopy is whether a class has a user-provided copy constructor (ctor)
// or copy assignment of its own.
func (u *unit) userCopy(rec *types.Record, ctor bool) bool {
	if ctor {
		return u.copyConstructor(rec) != nil
	}
	return u.copyAssignment(rec) != nil
}

// implicitCopy is the synthesized copy constructor (ctor) or copy
// assignment operator of rec.
func (u *unit) implicitCopy(rec *types.Record, ctor bool) ir.Callee {
	key := copyKey{rec, ctor}
	if f, done := u.implicitCopies[key]; done {
		return f
	}
	param := types.AddLValueReference(types.AddConst(rec))
	var sym *sema.FuncSymbol
	if ctor {
		sym = &sema.FuncSymbol{
			SymName: rec.Name, SymScope: u.res.GlobalScope, InClass: rec, Access: types.AccessPublic, Inline: true,
			FuncType: &types.Func{Ret: types.Typ(types.Void), Params: []types.Param{{Name: "other", Type: param}}},
		}
	} else {
		sym = &sema.FuncSymbol{
			SymName: "operator=", SymScope: u.res.GlobalScope, InClass: rec, Access: types.AccessPublic, Inline: true,
			FuncType: &types.Func{Ret: types.AddLValueReference(rec), Params: []types.Param{{Name: "other", Type: param}}},
		}
	}
	sym.Params = []*sema.VarSymbol{{SymName: "other", SymType: param, IsParam: true}}
	f := u.declareFunc(sym)
	u.funcs[sym] = f
	u.implicitCopies[key] = f

	fl := &fn{u: u, sym: sym, f: f, slots: map[sema.Symbol]ir.Ptr{}}
	fl.entry = f.Entry()
	body := f.Block("body")
	fl.blk = body
	params := u.params[sym]
	this, _ := params[0].(ir.Ptr)
	other, _ := params[1].(ir.Ptr)
	fl.this, fl.hasThis = this, true

	fl.copyMembers(this, other, rec, ctor, sym.SymPos)
	if ctor {
		fl.installVPtr(this, rec)
		if u.ctorReturnsThis(sym) {
			fl.blk.Return(this)
		} else {
			fl.blk.Return()
		}
	} else {
		fl.blk.Return(this)
	}
	fl.entry.Br(body.To())
	return f
}

// copyKey names an implicit copy apart: the class, and which of the two.
type copyKey struct {
	rec  *types.Record
	ctor bool
}

// copyMembers copies the bases and members of rec from src to dst, each
// the way its own type copies.
func (fl *fn) copyMembers(dst, src ir.Ptr, rec *types.Record, ctor bool, at ast.Tok) {
	fieldOffs := make([]int64, len(rec.Fields))
	baseOffs := make([]int64, len(rec.Bases))
	fl.u.model.LayoutWithBases(rec, fieldOffs, baseOffs)
	for i, b := range rec.Bases {
		br := classOf(b.Type)
		if br == nil || b.Virtual {
			continue
		}
		d, s := dst, src
		if baseOffs[i] != 0 {
			d = fl.blk.Ptr.Add(dst, fl.blk.I64.Const(baseOffs[i]))
			s = fl.blk.Ptr.Add(src, fl.blk.I64.Const(baseOffs[i]))
		}
		fl.copyOne(d, s, br, ctor, at)
	}
	for i, f := range rec.Fields {
		if f.BitField {
			continue
		}
		d, s := dst, src
		if fieldOffs[i] != 0 {
			d = fl.blk.Ptr.Add(dst, fl.blk.I64.Const(fieldOffs[i]))
			s = fl.blk.Ptr.Add(src, fl.blk.I64.Const(fieldOffs[i]))
		}
		fl.copyElements(d, s, f.Type, ctor, at)
	}
	// Bit-fields share storage units; their bytes go together.
	for i, f := range rec.Fields {
		if !f.BitField {
			continue
		}
		size, _ := fl.u.sizeAlign(f.Type)
		d := fl.blk.Ptr.Add(dst, fl.blk.I64.Const(fieldOffs[i]))
		s := fl.blk.Ptr.Add(src, fl.blk.I64.Const(fieldOffs[i]))
		fl.blk.MemCpy(d, s, fl.blk.I64.Const(size))
	}
}

// copyElements copies one member: an array element by element, a class
// by its copy, a scalar by value.
func (fl *fn) copyElements(dst, src ir.Ptr, t types.Type, ctor bool, at ast.Tok) {
	if arr, isArr := types.Unqualify(t).(*types.Array); isArr {
		if classOf(elementOf(t)) == nil {
			size, _ := fl.u.sizeAlign(t)
			fl.blk.MemCpy(dst, src, fl.blk.I64.Const(size))
			return
		}
		elemSize, _ := fl.u.sizeAlign(arr.Elem)
		for i := int64(0); i < arr.Len; i++ {
			d, s := dst, src
			if i > 0 {
				d = fl.blk.Ptr.Add(dst, fl.blk.I64.Const(i*elemSize))
				s = fl.blk.Ptr.Add(src, fl.blk.I64.Const(i*elemSize))
			}
			fl.copyElements(d, s, arr.Elem, ctor, at)
		}
		return
	}
	if rec := classOf(t); rec != nil {
		fl.copyOne(dst, src, rec, ctor, at)
		return
	}
	if isReference(t) {
		// A reference member is not assignable; copying the object that
		// holds one is the pointer it is.
		fl.blk.Ptr.Store(fl.blk.Ptr.Load(src), dst)
		return
	}
	fl.store(dst, fl.load(src, t), t)
}

// copyOne copies a subobject of class rec: by its user-provided copy, its
// synthesized one, or its bytes.
func (fl *fn) copyOne(dst, src ir.Ptr, rec *types.Record, ctor bool, at ast.Tok) {
	if ctor {
		fl.copyObject(dst, src, rec, nil)
		return
	}
	fl.assignObject(dst, src, rec, at)
}

// assignObject lowers `dst = src` for two objects of class rec.
func (fl *fn) assignObject(dst, src ir.Ptr, rec *types.Record, at ast.Tok) bool {
	if op := fl.u.copyAssignment(rec); op != nil {
		target := fl.u.callee(op)
		if target == nil {
			fl.u.errorf(at, "lowering has no symbol for the copy assignment of %s", rec.Name)
			return false
		}
		arg := ir.Value(src)
		if p := op.FuncType.Params[0].Type; !isReference(p) && !fl.u.plainForCalls(rec) {
			// By value: a copy the caller makes.
			tmp := fl.alloc(rec, "")
			if !fl.copyObject(tmp, src, rec, nil) {
				return false
			}
			arg = tmp
		}
		fl.blk.Call(target, dst, arg)
		return true
	}
	if fl.u.memberwise(rec, false) {
		fl.blk.Call(fl.u.implicitCopy(rec, false), dst, src)
		return true
	}
	return fl.copyObjectBytes(dst, src, rec)
}
