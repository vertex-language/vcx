package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/mangle"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// A scope tracks objects requiring destruction upon leaving a block.
// Destructors execute in reverse order of construction at block exits
// (fallthrough, return, break, continue).
type scope struct {
	objs []localObj
}

// localObj is an object with a destructor to run.
type localObj struct {
	addr ir.Ptr
	rec  *types.Record
}

// pushScope opens a block.
func (fl *fn) pushScope() {
	fl.scopes = append(fl.scopes, &scope{})
}

// popScope closes a block, destroying what it declared if control reaches
// the end. A block that ended in a terminator already ran its destructors
// on the way out, so nothing is emitted for it here.
func (fl *fn) popScope() {
	top := fl.scopes[len(fl.scopes)-1]
	fl.scopes = fl.scopes[:len(fl.scopes)-1]
	if fl.blk != nil {
		fl.destroyScope(top)
	}
}

// track registers a freshly constructed object with the current scope,
// if its class has a destructor to run.
func (fl *fn) track(addr ir.Ptr, t types.Type) {
	rec := classOf(t)
	if rec == nil || len(fl.scopes) == 0 || fl.u.destructor(rec) == nil {
		return
	}
	top := fl.scopes[len(fl.scopes)-1]
	top.objs = append(top.objs, localObj{addr: addr, rec: rec})
}

// temporary registers a full-expression temporary for destruction at the
// end of the full-expression, in reverse order of creation.
func (fl *fn) temporary(addr ir.Ptr, rec *types.Record) {
	if rec == nil || fl.u.destructor(rec) == nil {
		return
	}
	fl.temps = append(fl.temps, localObj{addr: addr, rec: rec})
}

// endFullExpr is the end of a full-expression: its temporaries die here.
func (fl *fn) endFullExpr() {
	temps := fl.temps
	fl.temps = nil
	for i := len(temps) - 1; i >= 0; i-- {
		fl.destroy(temps[i].addr, temps[i].rec)
	}
}

// extendTemporary extends the lifetime of a temporary bound to a reference,
// transferring it to the enclosing scope.
func (fl *fn) extendTemporary(addr ir.Ptr) {
	kept := fl.temps[:0:0]
	for _, t := range fl.temps {
		if t.addr == addr {
			if len(fl.scopes) > 0 {
				top := fl.scopes[len(fl.scopes)-1]
				top.objs = append(top.objs, t)
			}
			continue
		}
		kept = append(kept, t)
	}
	fl.temps = kept
	fl.endFullExpr()
}

// destroyScope runs one scope's destructors, last constructed first.
func (fl *fn) destroyScope(s *scope) {
	for i := len(s.objs) - 1; i >= 0; i-- {
		fl.destroy(s.objs[i].addr, s.objs[i].rec)
	}
}

// destroyFrom runs the destructors of every scope from depth outward --
// what a jump out of those scopes has to do before it goes.
func (fl *fn) destroyFrom(depth int) {
	for i := len(fl.scopes) - 1; i >= depth; i-- {
		fl.destroyScope(fl.scopes[i])
	}
}

// destroy calls a class's destructor on an object.
func (fl *fn) destroy(addr ir.Ptr, rec *types.Record) {
	if fl.blk == nil {
		return
	}
	if d := fl.u.destructor(rec); d != nil {
		fl.blk.Call(d, addr)
	}
}

// destructor is the function this unit holds for a class's destructor:
// the one it declared, the one synthesized for it (implicitDestructor),
// or nil for a class with nothing to destroy.
func (u *unit) destructor(rec *types.Record) ir.Callee {
	if sym := u.destructorSymbol(rec); sym != nil {
		return u.callee(sym)
	}
	return nil
}

// destructorSymbol is the class's destructor: its own as declared, or the
// implicit one when a member or base has a destructor to run.
func (u *unit) destructorSymbol(rec *types.Record) *sema.FuncSymbol {
	for _, m := range rec.Methods {
		if m.Name == "~"+rec.Name && !m.Defaulted {
			// Declared by the class: that one, or nothing -- never a
			// synthesized one over it.
			return u.declaredFor(&sema.FuncSymbol{SymName: m.Name, FuncType: m.Func, InClass: rec})
		}
	}
	return u.implicitDestructor(rec)
}

// needsDestructor reports whether a class's destructor is not trivial:
// one declared explicitly, or an inherited base or member with a non-trivial destructor.
func (u *unit) needsDestructor(rec *types.Record) bool {
	for _, m := range rec.Methods {
		if m.Name == "~"+rec.Name && !m.Defaulted {
			return true
		}
	}
	if u.model.ABI.IsItanium() && types.HasVirtualDtor(rec) {
		// A virtual destructor is never trivial.
		return true
	}
	for _, b := range rec.Bases {
		if br, isRec := types.Unqualify(b.Type).(*types.Record); isRec && u.needsDestructor(br) {
			return true
		}
	}
	for _, f := range rec.Fields {
		if fr := classOf(elementOf(f.Type)); fr != nil && u.needsDestructor(fr) {
			return true
		}
	}
	return false
}

// elementOf is an array's innermost element type, or t itself.
func elementOf(t types.Type) types.Type {
	for {
		arr, isArr := types.Unqualify(t).(*types.Array)
		if !isArr {
			return t
		}
		t = arr.Elem
	}
}

// implicitDestructor synthesizes an implicit destructor for a class that declared
// none (or defaulted it) when members or bases have destructors to run.
func (u *unit) implicitDestructor(rec *types.Record) *sema.FuncSymbol {
	if sym, done := u.implicitDtors[rec]; done {
		return sym
	}
	if !u.needsDestructor(rec) {
		u.implicitDtors[rec] = nil
		return nil
	}
	virtual := false
	for _, t := range u.model.VTables(rec) {
		for _, s := range t.Slots {
			if s.Name == "{dtor}" {
				virtual = true
			}
		}
	}
	sym := &sema.FuncSymbol{
		SymName:  "~" + rec.Name,
		SymScope: u.res.GlobalScope,
		FuncType: &types.Func{Ret: types.Typ(types.Void)},
		InClass:  rec,
		Access:   types.AccessPublic,
		Virtual:  virtual,
		Inline:   true,
	}
	u.implicitDtors[rec] = sym
	f := u.declareFunc(sym)
	u.funcs[sym] = f

	fl := &fn{u: u, sym: sym, f: f, slots: map[sema.Symbol]ir.Ptr{}}
	fl.entry = f.Entry()
	fl.blk = f.Block("body")
	if p, isPtr := u.params[sym][0].(ir.Ptr); isPtr {
		fl.this, fl.hasThis = p, true
	}
	fl.destroyMembersAndBases(sym)
	fl.entry.Br(fl.f.Blocks()[1].To())
	if u.ctorReturnsThis(sym) {
		fl.blk.Return(fl.this)
	} else {
		fl.blk.Return()
	}
	u.structorVariant(sym, f)
	return sym
}

// deletingDtor is Microsoft's vector deleting destructor for a class with
// a destructor, `??_EC@@UEAAPEAXI@Z`: the function a virtual destructor's
// table slot holds, and so the one `delete p` reaches through a base, and
// the one `delete[] p` calls directly. It runs the destructor and, when bit
// 0 of its flags is set, deallocates -- the callee frees under this ABI,
// which is how a delete through a base pointer frees the complete object
// without knowing its size. Bit 1 says `this` is an array: the count is
// in the cookie before it, the elements are destroyed last first, and the
// allocation freed is the one that starts at the cookie.
//
// cl defines the scalar one, `??_G`, in every unit that emits the table,
// as a COMDAT, and makes `??_E` a weak external defaulting to it; the
// linker applies that default only to the unit that declared it, so a
// reference from an object with no such record stays unresolved. Until
// the object writer can spell a weak alias, both are defined here, one
// COMDAT copy each, wherever the table is: a cl unit's weak `??_E` then
// resolves to this definition and its own `??_G` merges with this one.
func (u *unit) deletingDtor(rec *types.Record) ir.Callee {
	if c, done := u.deletingDtors[rec]; done {
		return c
	}
	dtor := u.destructorSymbol(rec)
	if dtor == nil {
		return nil
	}
	target := u.callee(dtor)
	if target == nil {
		return nil
	}
	u.deletingDtors[rec] = nil
	vector := u.defineDeletingDtor(dtor, mangle.DeletingDtor, target)
	u.defineDeletingDtor(dtor, mangle.ScalarDeletingDtor, target)
	u.deletingDtors[rec] = vector
	return vector
}

func (u *unit) defineDeletingDtor(dtor *sema.FuncSymbol, kind mangle.Kind, target ir.Callee) ir.Callee {
	d := mangle.Describe(dtor)
	d.Kind = kind
	d.Type = &types.Func{
		Ret:    &types.Pointer{Elem: types.Typ(types.Void)},
		Params: []types.Param{{Type: types.Typ(types.UInt)}},
		Quals:  dtor.FuncType.Quals,
	}
	name, err := mangle.FunctionName(u.opt.ABI, d)
	if err != nil {
		u.errorf(dtor.SymPos, "%v", err)
		return nil
	}
	f := u.mod.Func(u.symbolName(name)).Export().Comdat()
	this := f.ParamPtr("this")
	flags := f.ParamI32("flags")
	f.ReturnsPtr()
	entry := f.Entry()
	body := f.Block("body")
	free := f.Block("free")
	done := f.Block("done")
	rec := dtor.InClass
	size, _ := u.sizeAlign(rec)
	cookie := u.arrayCookie(rec)

	var idx ir.Ptr
	if kind == mangle.DeletingDtor {
		idx = entry.Ptr.Alloc(8, 8).Named("__i")
	}
	entry.Br(body.To())

	if kind == mangle.DeletingDtor {
		// flags & 2: the array branch.
		arr := f.Block("array")
		head := f.Block("array_head")
		each := f.Block("array_each")
		afree := f.Block("array_free")
		afreeDo := f.Block("array_free_do")
		scalar := f.Block("scalar")
		body.BrIf(body.I32.Ne(body.I32.And(flags, body.I32.Const(2)), body.I32.Const(0)), arr.To(), scalar.To())

		// i = count; while (i > 0) { --i; ~T(this + i*size); }
		arr.I64.Store(arr.I64.Load(arr.Ptr.Add(this, arr.I64.Const(-cookie))), idx)
		arr.Br(head.To())
		head.BrIf(head.I64.SLt(head.I64.Const(0), head.I64.Load(idx)), each.To(), afree.To())
		i := each.I64.Sub(each.I64.Load(idx), each.I64.Const(1))
		each.I64.Store(i, idx)
		each.Call(target, each.Ptr.Add(this, each.I64.Mul(i, each.I64.Const(size))))
		each.Br(head.To())
		afree.BrIf(afree.I32.Ne(afree.I32.And(flags, afree.I32.Const(1)), afree.I32.Const(0)), afreeDo.To(), done.To())
		if align := u.overAlignment(rec); align != 0 {
			afreeDo.Call(u.operatorDeleteArrayAligned(), afreeDo.Ptr.Add(this, afreeDo.I64.Const(-cookie)), afreeDo.I64.Const(align))
		} else {
			afreeDo.Call(u.operatorDeleteArray(), afreeDo.Ptr.Add(this, afreeDo.I64.Const(-cookie)))
		}
		afreeDo.Br(done.To())

		body = scalar
	}
	body.Call(target, this)
	body.BrIf(body.I32.Ne(body.I32.And(flags, body.I32.Const(1)), body.I32.Const(0)), free.To(), done.To())
	if align := u.overAlignment(rec); align != 0 {
		free.Call(u.operatorDeleteAligned(), this, free.I64.Const(align))
	} else {
		free.Call(u.operatorDelete(), this)
	}
	free.Br(done.To())
	done.Return(this)
	return f
}

// destroyMembersAndBases is the end of a destructor's body: it destroys non-static
// data members in reverse declaration order, followed by base subobjects in reverse construction order.
func (fl *fn) destroyMembersAndBases(sym *sema.FuncSymbol) {
	if fl.blk == nil || !fl.hasThis {
		return
	}
	rec := sym.InClass
	fieldOffs := make([]int64, len(rec.Fields))
	baseOffs := make([]int64, len(rec.Bases))
	fl.u.model.LayoutWithBases(rec, fieldOffs, baseOffs)
	for i := len(rec.Fields) - 1; i >= 0; i-- {
		f := rec.Fields[i]
		fr := classOf(elementOf(f.Type))
		if fr == nil || fl.u.destructor(fr) == nil {
			continue
		}
		at := fl.this
		if fieldOffs[i] != 0 {
			at = fl.blk.Ptr.Add(at, fl.blk.I64.Const(fieldOffs[i]))
		}
		fl.destroyElements(at, f.Type)
	}
	for i := len(rec.Bases) - 1; i >= 0; i-- {
		br, isRec := types.Unqualify(rec.Bases[i].Type).(*types.Record)
		if !isRec || rec.Bases[i].Virtual {
			continue
		}
		obj := fl.this
		if baseOffs[i] != 0 {
			obj = fl.blk.Ptr.Add(obj, fl.blk.I64.Const(baseOffs[i]))
		}
		fl.destroy(obj, br)
	}
}

// destroyElements destroys an object of type t at addr: the elements of
// an array, last first, or the class itself.
func (fl *fn) destroyElements(addr ir.Ptr, t types.Type) {
	if arr, isArr := types.Unqualify(t).(*types.Array); isArr {
		elemSize, _ := fl.u.sizeAlign(arr.Elem)
		for i := arr.Len - 1; i >= 0; i-- {
			at := addr
			if i > 0 {
				at = fl.blk.Ptr.Add(addr, fl.blk.I64.Const(i*elemSize))
			}
			fl.destroyElements(at, arr.Elem)
		}
		return
	}
	if rec := classOf(t); rec != nil {
		fl.destroy(addr, rec)
	}
}
