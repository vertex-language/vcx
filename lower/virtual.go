package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/mangle"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// Virtual dispatch: the tables, the pointer to them, and the call through.
//
// The layout of a table -- which slot each function lands in -- is
// types.Model's, and tests/abi has already diffed it against cl entry for
// entry. What is here is the other half: writing that table into the object
// file, storing its address into every new object, and reading it back at
// each call. None of those three is visible in a layout dump, and all three
// have to agree with the table for a call through a base pointer to arrive
// anywhere.

// declareVTables emits one read-only data symbol per virtual table.
//
// A table is an array of code pointers, each a relocation to the function
// the slot holds -- the most-derived override, which the model already
// resolved. A table is emitted where an object of the class comes into
// existence -- a constructor of the class, an object declared without
// one -- which is where cl emits its own, and the linker keeps one copy.
// Emitting one for every polymorphic class a unit sees would pull in the
// destructors and overrides its slots name, as far as the runtime's own
// (a unit including <exception> would carry ~nested_exception).
func (u *unit) declareVTables() {
	u.vtables = map[*types.Record][]vtable{}
	u.typeInfos = map[*types.Record]ir.Symbol{}
	u.cxxabiTables = map[string]ir.Symbol{}
	u.thunks = map[thunkKey]ir.Callee{}
	u.deletingDtors = map[*types.Record]ir.Callee{}
	u.implicitDtors = map[*types.Record]*sema.FuncSymbol{}
	u.invokers = map[*types.Record]ir.Callee{}
	u.implicitCopies = map[copyKey]ir.Callee{}
}

// vtablesOf is a class's tables, emitted on first demand.
func (u *unit) vtablesOf(r *types.Record) []vtable {
	if tables, done := u.vtables[r]; done {
		return tables
	}
	u.vtables[r] = nil
	u.declareVTable(r)
	return u.vtables[r]
}

func (u *unit) declareVTable(r *types.Record) {
	if u.model.ABI.IsItanium() {
		u.declareVTableGroup(r)
		return
	}
	tables := u.model.VTables(r)
	if len(tables) == 0 {
		return
	}

	// A class with one table names it plainly; one with several -- one per
	// polymorphic base -- names each for its base, the primary included.
	// That is cl's rule and the linker's expectation, and the thing the
	// name is for.
	for i, table := range tables {
		var base []*types.Record
		if len(tables) > 1 {
			base = []*types.Record{types.FindBase(r, table.Base)}
		}
		name, err := mangle.VTableName(u.opt.ABI, r, base)
		if err != nil {
			u.errorf(ast.NoTok, "%v", err)
			return
		}

		var entries []ir.Init
		for _, slot := range table.Slots {
			fn := u.slotFunc(r, slot)
			if fn == nil {
				// A pure virtual or missing definer gets a null slot pointer.
				entries = append(entries, ir.Lit(ir.Int(0)))
				continue
			}
			if adjust := u.model.ThunkAdjust(r, table, slot); adjust != 0 {
				fn = u.thunk(r, slot, fn, adjust)
			}
			entries = append(entries, ir.RelocInit(fn))
		}

		// Every unit that constructs an object of the class emits its
		// table, and the linker keeps one: the same COMDAT rule an
		// inline function follows, and cl's own.
		g := u.mod.Global(u.symbolName(name), ir.RO, ir.Array(uint64(len(entries)), ir.StorePtr.FType())).Export().Comdat()
		g.Align(uint64(u.model.SizePtr))
		g.Init(ir.List(entries...))
		u.vtables[r] = append(u.vtables[r], vtable{global: g, offset: table.Offset})
		_ = i
	}
}

// vtable is one emitted table and where in the object its pointer goes.
// point is how far into the global the pointer aims: zero for a
// Microsoft table, which is its own symbol, and the address point within
// the group for an Itanium one.
type vtable struct {
	global *ir.Global
	offset int64
	point  int64
}

// thunk is the adjustor a secondary table holds for an override defined
// in the derived class: it moves `this` from the base subobject back to
// the complete object and tail-calls the real function.
func (u *unit) thunk(r *types.Record, slot types.VSlot, target ir.Callee, adjust int64) ir.Callee {
	definer := findRecord(r, slot.Definer)
	itaniumDtor := slot.Name == "{dtor}" && u.model.ABI.IsItanium()
	var sym *sema.FuncSymbol
	if itaniumDtor {
		sym = u.destructorSymbol(definer)
	} else {
		sym = sema.MethodOf(u.res, definer, slot)
	}
	if sym == nil {
		return target
	}
	key := thunkKey{sym, adjust, slot.Deleting}
	if t, done := u.thunks[key]; done {
		return t
	}
	desc := mangle.Describe(sym)
	if slot.Deleting {
		desc.Kind = mangle.DtorDeleting
	}
	name, err := mangle.ThunkName(u.opt.ABI, desc, adjust)
	if err != nil {
		u.errorf(sym.SymPos, "%v", err)
		return target
	}
	if _, defined := u.funcs[sym]; !defined && !itaniumDtor {
		// The override is defined in another object, and so is its
		// thunk: the table refers to it by name.
		imp := u.mod.ImportFunc(u.symbolName(name), u.signature(sym))
		u.thunks[key] = imp
		return imp
	}

	f := u.mod.Func(u.symbolName(name)).Export()
	if u.model.ABI.IsItanium() {
		// A thunk is emitted wherever its table is, and the table is a
		// COMDAT; so is the thunk, or the second unit is a duplicate.
		f.Comdat()
	}
	this := f.ParamPtr("this")
	args := []ir.Value{this}
	if itaniumDtor {
		if !slot.Deleting && u.ctorReturnsThis(sym) {
			f.ReturnsPtr()
		}
	} else {
		for _, p := range sym.Params {
			args = append(args, u.declareParam(f, p))
		}
		u.declareResult(f, sym.FuncType.Ret)
	}
	body := f.Block("body")
	f.Entry().Br(body.To())
	args[0] = body.Ptr.Add(this, body.I64.Const(-adjust))
	results := body.Call(target, args...)
	if results.Len() > 0 {
		body.Return(results.Value(0))
	} else {
		body.Return()
	}
	u.thunks[key] = f
	return f
}

// thunkKey names a thunk apart: the function it reaches and by how much,
// and for an Itanium destructor which of its two slots.
type thunkKey struct {
	fn       *sema.FuncSymbol
	adjust   int64
	deleting bool
}

// slotFunc is the function a table slot holds, or nil if this unit does not
// define it.
func (u *unit) slotFunc(r *types.Record, slot types.VSlot) ir.Callee {
	if slot.Pure {
		// A pure virtual slot holds the runtime's purecall helper.
		return u.purecall()
	}
	definer := findRecord(r, slot.Definer)
	if definer == nil {
		return nil
	}
	if slot.Name == "{dtor}" {
		if u.model.ABI.IsItanium() {
			if slot.Deleting {
				return u.deletingDtorItanium(definer)
			}
			return u.destructor(definer)
		}
		return u.deletingDtor(definer)
	}
	return u.method(definer, slot.Name, slot.Signature)
}

// purecall is the import of the runtime's pure-virtual trap: the C
// runtime's _purecall under Microsoft, the C++ ABI library's
// __cxa_pure_virtual under Itanium.
func (u *unit) purecall() ir.Callee {
	if u.purecallFn == nil {
		name := "_purecall"
		if u.model.ABI.IsItanium() {
			name = "__cxa_pure_virtual"
		}
		u.purecallFn = u.mod.ImportFunc(u.symbolName(name), ir.NewSig())
	}
	return u.purecallFn
}

// findRecord walks a class and its bases for the one with a given name --
// the definer a slot names, which is somewhere above the class whose table
// is being built.
func findRecord(r *types.Record, name string) *types.Record {
	if r == nil {
		return nil
	}
	if r.Name == name {
		return r
	}
	for _, b := range r.Bases {
		if br, ok := types.Unqualify(b.Type).(*types.Record); ok {
			if found := findRecord(br, name); found != nil {
				return found
			}
		}
	}
	return nil
}

// installVPtr stores a class's table address into an object.
//
// Each constructor overwrites the vptr with its own class vtable before its
// body runs, ensuring virtual calls dispatch to the currently constructed type.
func (fl *fn) installVPtr(obj ir.Ptr, r *types.Record) {
	for _, vt := range fl.u.vtablesOf(r) {
		at := obj
		if vt.offset != 0 {
			at = fl.blk.Ptr.Add(obj, fl.blk.I64.Const(vt.offset))
		}
		table := fl.blk.Ptr.GetAddr(vt.global)
		if vt.point != 0 {
			table = fl.blk.Ptr.Add(table, fl.blk.I64.Const(vt.point))
		}
		fl.blk.Ptr.Store(table, at)
	}
}

// virtualCall dispatches a call through the object's table.
//
// The pointer at offset zero names the table; slot n is n pointers into it;
// the function found there takes the object as its first argument, the same
// way a direct member call would pass it. The signature named on the
// indirect call is the callee's, which is what keeps the call well-typed
// without the callee being known.
func (fl *fn) virtualCall(e *ast.CallExpr, callee *sema.FuncSymbol, obj ir.Value, args []ir.Value, objIdx int) ir.Value {
	objPtr, ok := obj.(ir.Ptr)
	if !ok {
		fl.u.errorf(e.Pos(), "lowering: a virtual call on something that is not an address")
		return nil
	}

	// The slot is in the table of whichever subobject introduced the
	// function, which for a virtual inherited from a secondary base is
	// not the one at offset zero. That table's pointer sits at the
	// subobject, and the subobject's address is what the call passes --
	// the function in the slot expects exactly that (see ThisOffset).
	tableOff, slot, found := fl.u.vslot(callee)
	if !found {
		fl.u.errorf(e.Pos(), "lowering found no table slot for %s::%s", callee.InClass.Name, callee.SymName)
		return nil
	}

	b := fl.blk
	sub := objPtr
	if tableOff != 0 {
		sub = b.Ptr.Add(objPtr, b.I64.Const(tableOff))
	}
	vptr := b.Ptr.Load(sub)
	entry := vptr
	if slot > 0 {
		entry = b.Ptr.Add(vptr, b.I64.Const(int64(slot)*fl.u.model.SizePtr))
	}
	target := b.Ptr.Load(entry)

	args[objIdx] = sub
	res := b.CallInd(target, fl.u.funcType(callee), args...)
	if res.Len() == 0 {
		return nil
	}
	return res.Value(0)
}

// vslot is the table a member function's slot is in, by its offset in the
// object, and the slot's index in it.
func (u *unit) vslot(fn *sema.FuncSymbol) (int64, int, bool) {
	name := fn.SymName
	if name == "~"+fn.InClass.Name {
		name = "{dtor}"
	}
	for _, table := range u.model.VTables(fn.InClass) {
		for i, s := range table.Slots {
			if s.Name != name {
				continue
			}
			if s.Signature != nil && fn.FuncType != nil && !sameParams(s.Signature, fn.FuncType) {
				continue
			}
			return table.Offset, i, true
		}
	}
	return 0, 0, false
}

func sameParams(a, b *types.Func) bool {
	if len(a.Params) != len(b.Params) {
		return false
	}
	for i := range a.Params {
		if !a.Params[i].Type.Equal(b.Params[i].Type) {
			return false
		}
	}
	return true
}

// funcType is the named signature an indirect call to a member function
// carries: `this` first, then the declared parameters.
func (u *unit) funcType(fn *sema.FuncSymbol) *ir.Type {
	if t, ok := u.funcTypes[fn]; ok {
		return t
	}
	// The same shape declareFunc gives a definition: the hidden result
	// first or after `this`, `this`, then the parameters with a plain
	// class marked byval.
	sig := u.signature(fn)
	t := u.mod.FuncType(u.funcTypeKey(fn), sig)
	u.funcTypes[fn] = t
	return t
}

func (u *unit) regTypeOrPtr(t types.Type) ir.RegType {
	if rt := u.regType(t); rt != ir.TypeNone {
		return rt
	}
	return ir.TypePtr
}

// isVirtualCall reports whether a member call dispatches through the vtable:
// the function is virtual and the call is not explicitly qualified.
func isVirtualCall(e *ast.CallExpr, callee *sema.FuncSymbol) bool {
	if callee == nil || callee.InClass == nil || !isVirtualMember(callee) {
		return false
	}
	if mem, ok := unparen(e.Fun).(*ast.MemberExpr); ok {
		if _, qualified := mem.Sel.(*ast.QualifiedName); qualified {
			return false
		}
	}
	return true
}

// isVirtualMember reports whether a member function is virtual, counting
// the ones that override a base's virtual without repeating the keyword.
func isVirtualMember(fn *sema.FuncSymbol) bool {
	if fn.Virtual {
		return true
	}
	// An override is virtual even if not explicitly marked virtual.
	for _, m := range fn.InClass.Methods {
		if m.Name == fn.SymName && m.Virtual {
			return true
		}
	}
	for _, b := range fn.InClass.Bases {
		br, ok := types.Unqualify(b.Type).(*types.Record)
		if !ok {
			continue
		}
		if isVirtualMember(&sema.FuncSymbol{SymName: fn.SymName, FuncType: fn.FuncType, InClass: br}) {
			return true
		}
	}
	return false
}
