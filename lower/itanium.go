package lower

import (
	"strconv"
	"strings"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/mangle"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// The Itanium C++ ABI's half of what virtual.go and dtor.go do.
//
// Under Microsoft a class has one constructor symbol, one destructor, and a
// deleting destructor that takes flags. Under Itanium each of those is a
// family: the complete-object constructor C1 and the base-object C2, the
// complete-object destructor D1, the base-object D2 and the deleting D0.
// Without virtual bases the complete and base variants do the same work,
// and a unit could get by with only the complete ones -- until a class
// compiled by clang derives from one of vcx's and its constructor calls C2,
// which is what tests/link found. So every structor vcx defines gets its
// base variant beside it, forwarding to the complete one.
//
// And the tables are one symbol per class, a vtable group: each table's
// offset-to-top and type-info entry in front of its functions, the object's
// pointer aimed past them at the first function.

// structorVariant defines the base-object variant of a constructor or
// destructor vcx just defined: C2 beside C1, D2 beside D1.
func (u *unit) structorVariant(sym *sema.FuncSymbol, target *ir.Func) {
	if u.model.ABI.IsMicrosoft() || sym == nil || sym.InClass == nil || sym.Static || target == nil {
		return
	}
	kind := mangle.Ordinary
	switch sym.SymName {
	case sym.InClass.Name:
		kind = mangle.CtorBase
	case "~" + sym.InClass.Name:
		kind = mangle.DtorBase
	default:
		return
	}
	if len(types.VirtualBases(sym.InClass)) > 0 {
		// C2 and D2 skip the virtual bases C1 and D1 construct and
		// destroy, and take a VTT to find the construction tables by.
		// Forwarding would construct a shared base twice.
		u.errorf(sym.SymPos, "lowering: %s has virtual bases, whose structors are not lowered under the Itanium ABI yet", sym.InClass.Name)
		return
	}
	d := mangle.Describe(sym)
	d.Kind = kind
	name, err := mangle.FunctionName(u.opt.ABI, d)
	if err != nil {
		u.errorf(sym.SymPos, "%v", err)
		return
	}
	f := u.mod.Func(u.symbolName(name)).Export()
	if sym.Inline {
		f.Comdat()
	}
	this := f.ParamPtr("this")
	args := []ir.Value{this}
	for _, p := range sym.Params {
		args = append(args, u.declareParam(f, p))
	}
	returnsThis := u.ctorReturnsThis(sym)
	if returnsThis {
		f.ReturnsPtr()
	}
	body := f.Block("body")
	f.Entry().Br(body.To())
	body.Call(target, args...)
	if returnsThis {
		body.Return(this)
	} else {
		body.Return()
	}
}

// deletingDtorItanium is a class's D0: the complete-object destructor and
// then operator delete on the same address. A virtual destructor's second
// slot holds it, and `delete p` through a base reaches it there; the object
// it frees is the complete one, which is the only thing that knows its own
// size. It returns nothing, under ARM's convention as well.
func (u *unit) deletingDtorItanium(rec *types.Record) ir.Callee {
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
	d := mangle.Describe(dtor)
	d.Kind = mangle.DtorDeleting
	name, err := mangle.FunctionName(u.opt.ABI, d)
	if err != nil {
		u.errorf(dtor.SymPos, "%v", err)
		return nil
	}
	f := u.mod.Func(u.symbolName(name)).Export().Comdat()
	this := f.ParamPtr("this")
	body := f.Block("body")
	f.Entry().Br(body.To())
	body.Call(target, this)
	if align := u.overAlignment(rec); align != 0 {
		body.Call(u.operatorDeleteAligned(), this, body.I64.Const(align))
	} else {
		body.Call(u.operatorDelete(), this)
	}
	body.Return()
	u.deletingDtors[rec] = f
	return f
}

// deletingDtorItaniumType is the signature a call through a D0 slot
// carries: the object, and nothing back.
func (u *unit) deletingDtorItaniumType() *ir.Type {
	if u.deletingDtorSig == nil {
		u.deletingDtorSig = u.mod.FuncType("sig_deleting_dtor", ir.NewSig().Param(ir.TypePtr))
	}
	return u.deletingDtorSig
}

// declareVTableGroup emits a class's vtable group as one symbol.
//
// Each table is, in address order: its vbase offsets, the last virtual
// base's first; the distance back to the top of the complete object; the
// type-info pointer; then the function entries. The pointer an object
// holds for the subobject a table serves is aimed at the first function
// entry -- the table's address point -- which is what makes slot n the
// same n under both compilers.
func (u *unit) declareVTableGroup(r *types.Record) {
	tables := u.model.VTables(r)
	if len(tables) == 0 {
		return
	}
	name, err := mangle.VTableName(u.opt.ABI, r, nil)
	if err != nil {
		u.errorf(ast.NoTok, "%v", err)
		return
	}
	var entries []ir.Init
	var points []vtable
	for _, table := range tables {
		for i := len(table.VBaseOffsets) - 1; i >= 0; i-- {
			entries = append(entries, ir.Lit(ir.Int(table.VBaseOffsets[i])))
		}
		entries = append(entries, ir.Lit(ir.Int(-table.Offset)))
		if ti := u.typeInfo(r); ti != nil {
			entries = append(entries, ir.RelocInit(ti))
		} else {
			entries = append(entries, ir.Lit(ir.Int(0)))
		}
		points = append(points, vtable{offset: table.Offset, point: int64(len(entries)) * u.model.SizePtr})
		for _, slot := range table.Slots {
			fn := u.slotFunc(r, slot)
			if fn == nil {
				entries = append(entries, ir.Lit(ir.Int(0)))
				continue
			}
			if adjust := u.model.ThunkAdjust(r, table, slot); adjust != 0 {
				fn = u.thunk(r, slot, fn, adjust)
			}
			entries = append(entries, ir.RelocInit(fn))
		}
	}
	g := u.mod.Global(u.symbolName(name), ir.RO, ir.Array(uint64(len(entries)), ir.StorePtr.FType())).Export().Comdat()
	g.Align(uint64(u.model.SizePtr))
	g.Init(ir.List(entries...))
	for i := range points {
		points[i].global = g
	}
	u.vtables[r] = points
}

// deleteArrayItanium is `delete[] p` for elements with a destructor: the
// count from the cookie, each element's D1 last first, and operator
// delete[] on the allocation, which starts at the cookie.
func (fl *fn) deleteArrayItanium(p ir.Ptr, rec *types.Record, ptrType types.Type) {
	dtor := fl.u.destructor(rec)
	size, _ := fl.u.sizeAlign(rec)
	cookie := fl.u.arrayCookie(rec)
	b := fl.blk
	idx := fl.alloc(types.Typ(types.LongLong), "__i")
	b.I64.Store(b.I64.Load(b.Ptr.Add(p, b.I64.Const(-8))), idx)
	head := fl.block("delarr_head")
	each := fl.block("delarr_each")
	exit := fl.block("delarr_exit")
	b.Br(head.To())

	fl.blk = head
	head.BrIf(head.I64.SLt(head.I64.Const(0), head.I64.Load(idx)), each.To(), exit.To())

	fl.blk = each
	i := each.I64.Sub(each.I64.Load(idx), each.I64.Const(1))
	each.I64.Store(i, idx)
	if dtor != nil {
		each.Call(dtor, each.Ptr.Add(p, each.I64.Mul(i, each.I64.Const(size))))
	}
	each.Br(head.To())

	fl.blk = exit
	fl.deallocate(exit.Ptr.Add(p, exit.I64.Const(-cookie)), ptrType, true)
}

// typeInfo is a class's std::type_info object, `_ZTI`, emitted on first
// demand with the name string it points at, `_ZTS`.
//
// A table's second entry points at it, so every unit that emits a table
// emits it too -- and has to, because a class whose key function vcx
// defined is one whose type information clang expects to find in vcx's
// object: clang's `typeinfo for Derived` names `typeinfo for Base` and
// does not define it. Like the table it is a COMDAT.
//
// The object is one of three runtime classes, chosen as clang chooses:
// __class_type_info for a class with no bases, __si_class_type_info for one
// public non-virtual base the class shares its dynamic-ness with, and
// __vmi_class_type_info for anything else. Its first word points into that
// class's own table, sixteen bytes in, the same address point every table
// has.
func (u *unit) typeInfo(r *types.Record) ir.Symbol {
	if ti, done := u.typeInfos[r]; done {
		return ti
	}
	if u.model.SizePtr != 8 {
		// The layouts below are written in eight-byte words.
		u.typeInfos[r] = nil
		return nil
	}
	tiName, err := mangle.TypeInfoName(u.opt.ABI, r)
	if err != nil {
		u.errorf(ast.NoTok, "%v", err)
		u.typeInfos[r] = nil
		return nil
	}
	tsName, _ := mangle.TypeNameName(u.opt.ABI, r)
	enc, _ := mangle.TypeName(u.opt.ABI, r)

	name := u.mod.Global(u.symbolName(tsName), ir.RO, ir.Array(uint64(len(enc)+1), ir.StoreI8.FType())).Export().Comdat()
	name.Init(ir.Str(enc + "\x00"))

	var words []ir.Init
	runtime := "__class_type_info"
	baseOffs := make([]int64, len(r.Bases))
	u.model.LayoutWithBases(r, make([]int64, len(r.Fields)), baseOffs)
	switch {
	case len(r.Bases) == 0:
	case u.rttiSingleInheritance(r):
		runtime = "__si_class_type_info"
	default:
		runtime = "__vmi_class_type_info"
	}
	table := u.cxxabiTable(runtime)
	words = append(words, ir.RelocInit(table).Plus(ir.Int(16)), ir.RelocInit(name))

	// Placeholder first: a base's type information may name this class's
	// through nothing, but a cycle through the map must not recurse.
	g := u.mod.Global(u.symbolName(tiName), ir.RO, ir.Array(uint64(u.typeInfoWords(r)), ir.StorePtr.FType())).Export().Comdat()
	g.Align(8)
	u.typeInfos[r] = g

	switch runtime {
	case "__si_class_type_info":
		br := types.Unqualify(r.Bases[0].Type).(*types.Record)
		words = append(words, ir.RelocInit(u.typeInfo(br)))
	case "__vmi_class_type_info":
		// flags and base_count are two 32-bit fields in one word, the
		// low half first.
		words = append(words, ir.Lit(ir.Int(int64(u.vmiFlags(r))|int64(len(r.Bases))<<32)))
		for i, b := range r.Bases {
			br, _ := types.Unqualify(b.Type).(*types.Record)
			var ti ir.Init = ir.Lit(ir.Int(0))
			if br != nil {
				if s := u.typeInfo(br); s != nil {
					ti = ir.RelocInit(s)
				}
			}
			flags := int64(0)
			if b.Access == types.AccessPublic {
				flags |= 2
			}
			if b.Virtual {
				flags |= 1
			}
			words = append(words, ti, ir.Lit(ir.Int(baseOffs[i]<<8|flags)))
		}
	}
	g.Init(ir.List(words...))
	return g
}

// typeInfoWords is how many eight-byte words a class's type_info takes.
func (u *unit) typeInfoWords(r *types.Record) int {
	switch {
	case len(r.Bases) == 0:
		return 2
	case u.rttiSingleInheritance(r):
		return 3
	}
	return 3 + 2*len(r.Bases)
}

// rttiSingleInheritance is clang's test for __si_class_type_info: one base,
// public and not virtual, dynamic exactly when the class is -- unless the
// base is empty, when that does not matter.
func (u *unit) rttiSingleInheritance(r *types.Record) bool {
	if len(r.Bases) != 1 || r.Bases[0].Virtual {
		return false
	}
	b := r.Bases[0]
	if !(b.Access == types.AccessPublic) {
		return false
	}
	br, ok := types.Unqualify(b.Type).(*types.Record)
	if !ok {
		return false
	}
	return types.IsEmptyRecord(br) || types.IsDynamic(br) == types.IsDynamic(r)
}

// vmiFlags is __vmi_class_type_info's flags word: 1 when some base class
// appears more than once, not by virtual inheritance; 2 when one appears
// more than once, by virtual inheritance.
func (u *unit) vmiFlags(r *types.Record) int {
	seen := map[*types.Record]bool{}
	virtualSeen := map[*types.Record]bool{}
	flags := 0
	var walk func(*types.Record, bool)
	walk = func(c *types.Record, viaVirtual bool) {
		for _, b := range c.Bases {
			br, ok := types.Unqualify(b.Type).(*types.Record)
			if !ok {
				continue
			}
			if b.Virtual {
				if virtualSeen[br] {
					flags |= 2
				}
				virtualSeen[br] = true
			} else {
				if seen[br] {
					flags |= 1
				}
				seen[br] = true
			}
			walk(br, b.Virtual)
		}
	}
	walk(r, false)
	return flags
}

// cxxabiTable is the import of one of the runtime's type_info tables.
func (u *unit) cxxabiTable(class string) ir.Symbol {
	if s, ok := u.cxxabiTables[class]; ok {
		return s
	}
	name := "_ZTVN10__cxxabiv1" + strconv.Itoa(len(class)) + class + "E"
	s := u.mod.ImportGlobal(u.symbolName(name), ir.StorePtr.FType())
	u.cxxabiTables[class] = s
	return s
}

// dynamicCastFn is the Itanium ABI's runtime cast:
//
//	void* __dynamic_cast(const void* sub, const __class_type_info* src,
//	                     const __class_type_info* dst, ptrdiff_t src2dst);
//
// The last argument is a hint about where src sits inside dst; -1 is "no
// hint", which is always correct and which the runtime answers by walking
// the type-info graph.
func (u *unit) dynamicCastFn() ir.Callee {
	if u.dynCast == nil {
		sig := ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypeI64).Ret(ir.TypePtr)
		u.dynCast = u.mod.ImportFunc(u.symbolName("__dynamic_cast"), sig)
	}
	return u.dynCast
}

// badCastFn throws std::bad_cast, which is what a failed dynamic_cast to a
// reference does. It does not return.
func (u *unit) badCastFn() ir.Callee {
	if u.badCast == nil {
		u.badCast = u.mod.ImportFunc(u.symbolName("__cxa_bad_cast"), ir.NewSig())
	}
	return u.badCast
}

// dynamicCast lowers dynamic_cast.
//
// A cast to the same class or to a base of it is the static adjustment and
// no runtime call at all -- the answer is known here. Anything else asks
// __dynamic_cast, which walks the object's own type-info and answers with
// the subobject's address, or null where the object is not of that type.
// A cast to void* is the whole object, which every table records as its
// distance back to the top.
//
// The pointer form guards the call: a null operand is a null result, and
// the runtime would dereference it looking for a table. The reference form
// needs no such guard, and throws where the pointer form would answer null.
func (fl *fn) dynamicCast(e *ast.NamedCastExpr) ir.Value {
	to := fl.typeOf(e)
	ref := isReference(to)

	target := types.RemoveReference(to)
	if !ref {
		p, isPtr := types.Unqualify(to).(*types.Pointer)
		if !isPtr {
			fl.u.errorf(e.Pos(), "lowering: dynamic_cast to %s, which is neither a pointer nor a reference", to)
			return nil
		}
		target = p.Elem
	}

	// The operand: the object a reference designates, or the pointer.
	var src ir.Ptr
	fromT := types.Unqualify(types.RemoveReference(fl.typeOf(e.X)))
	srcClass := classOf(fromT)
	if ref {
		p, _, ok := fl.lvalue(e.X)
		if !ok {
			fl.u.errorf(e.Pos(), "lowering: the operand of dynamic_cast has no address")
			return nil
		}
		src = p
	} else {
		fp, isPtr := fromT.(*types.Pointer)
		if !isPtr {
			fl.u.errorf(e.Pos(), "lowering: the operand of dynamic_cast is not a pointer")
			return nil
		}
		srcClass = classOf(fp.Elem)
		v := fl.expr(e.X)
		p, isP := v.(ir.Ptr)
		if !isP {
			fl.u.errorf(e.Pos(), "lowering: the operand of dynamic_cast is not an address")
			return nil
		}
		src = p
	}

	dstClass := classOf(types.Unqualify(target))
	toVoid := types.IsVoid(types.Unqualify(target))

	// An upcast is static: the same class needs nothing, a base needs the
	// subobject's offset, and convert() already knows it.
	if !toVoid && dstClass != nil && srcClass != nil &&
		(dstClass == srcClass || types.IsBaseOf(dstClass, srcClass)) {
		if ref {
			return fl.convert(src, &types.Pointer{Elem: srcClass}, &types.Pointer{Elem: dstClass})
		}
		return fl.convert(src, fl.typeOf(e.X), to)
	}

	if srcClass == nil || fl.u.vtablesOf(srcClass) == nil {
		fl.u.errorf(e.Pos(), "dynamic_cast needs a polymorphic operand")
		return nil
	}
	if !toVoid && dstClass == nil {
		fl.u.errorf(e.Pos(), "lowering: dynamic_cast to %s, which is no class", target)
		return nil
	}

	if ref {
		return fl.dynamicCastBody(src, srcClass, dstClass, toVoid, true, e)
	}

	// `p ? __dynamic_cast(p, ...) : nullptr`, through a slot: the call is
	// its own block.
	slot := fl.alloc(&types.Pointer{Elem: types.Typ(types.Void)}, "")
	fl.blk.Ptr.Store(fl.blk.Ptr.Const(), slot)
	do := fl.block("dyncast_do")
	join := fl.block("dyncast_join")
	fl.blk.BrIf(fl.blk.Ptr.Eq(src, fl.blk.Ptr.Const()), join.To(), do.To())

	fl.blk = do
	if v := fl.dynamicCastBody(src, srcClass, dstClass, toVoid, false, e); v != nil {
		if p, isP := v.(ir.Ptr); isP {
			fl.blk.Ptr.Store(p, slot)
		}
	}
	if fl.blk != nil {
		fl.blk.Br(join.To())
	}

	fl.blk = join
	return fl.blk.Ptr.Load(slot)
}

// dynamicCastBody is the cast itself, on an operand already known to be
// non-null: the table's distance to the top for void*, the runtime call
// otherwise, and for a reference the throw that a null answer means.
func (fl *fn) dynamicCastBody(src ir.Ptr, srcClass, dstClass *types.Record, toVoid, ref bool, e *ast.NamedCastExpr) ir.Value {
	b := fl.blk
	if toVoid {
		// The table's first word before its address point is the
		// distance back to the most derived object.
		vptr := b.Ptr.Load(src)
		top := b.I64.Load(b.Ptr.Add(vptr, b.I64.Const(-2*fl.u.model.SizePtr)))
		return b.Ptr.Add(src, top)
	}

	srcTI, dstTI := fl.u.typeInfo(srcClass), fl.u.typeInfo(dstClass)
	if srcTI == nil || dstTI == nil {
		fl.u.errorf(e.Pos(), "lowering: dynamic_cast has no type-info for %s or %s", srcClass.Name, dstClass.Name)
		return nil
	}
	res := b.Call(fl.u.dynamicCastFn(), src, b.Ptr.GetAddr(srcTI), b.Ptr.GetAddr(dstTI), b.I64.Const(-1))
	if res.Len() == 0 {
		return nil
	}
	out, isP := res.Value(0).(ir.Ptr)
	if !isP {
		return nil
	}
	if !ref {
		return out
	}

	// A reference cannot be null, so a null answer is a failed cast.
	bad := fl.block("dyncast_bad")
	ok := fl.block("dyncast_ok")
	fl.blk.BrIf(fl.blk.Ptr.Eq(out, fl.blk.Ptr.Const()), bad.To(), ok.To())
	fl.blk = bad
	fl.emitCall(fl.u.badCastFn())
	fl.blk.Br(ok.To())
	fl.blk = ok
	return out
}

// cxaAtexit is the Itanium runtime's registration of a static object's
// destructor:
//
//	int __cxa_atexit(void (*f)(void*), void* p, void* dso);
//
// The runtime runs what is registered in reverse, interleaved with the
// plain atexit handlers, which is the order [basic.start.term]/4 asks for:
// one list, walked backwards.
func (u *unit) cxaAtexit() ir.Callee {
	if u.atexitFn == nil {
		sig := ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypePtr).Ret(ir.TypeI32)
		u.atexitFn = u.mod.ImportFunc(u.symbolName("__cxa_atexit"), sig)
	}
	return u.atexitFn
}

// dsoHandle identifies this image, which __cxa_atexit records so that
// unloading a library runs that library's registrations and no others.
func (u *unit) dsoHandle() ir.Symbol {
	if u.dso == nil {
		u.dso = u.mod.ImportGlobal(u.symbolName("__dso_handle"), ir.StorePtr.FType())
	}
	return u.dso
}

// registerStaticDtor arranges for a static-duration object's destructor to
// run at exit, after the constructor that built it: without this a global
// or a function-local static with a destructor is never destroyed.
//
// An array of such objects wants a helper that walks it in reverse, which
// is not written yet; an array is left unregistered rather than registered
// with a function that would destroy only its first element.
func (fl *fn) registerStaticDtor(obj ir.Ptr, t types.Type) {
	if fl.blk == nil || !fl.u.model.ABI.IsItanium() {
		return
	}
	rec := classOf(t)
	if rec == nil || !fl.u.needsDestructor(rec) {
		return
	}
	dtor := fl.u.destructor(rec)
	if dtor == nil {
		return
	}
	b := fl.blk
	b.Call(fl.u.cxaAtexit(), b.Ptr.GetAddr(dtor), obj, b.Ptr.GetAddr(fl.u.dsoHandle()))
}

// typeInfoSymbol is the std::type_info object of any type: the one this
// unit emits for a class, and the runtime's for everything else -- the
// fundamental types' objects live in libc++abi.
func (u *unit) typeInfoSymbol(t types.Type) ir.Symbol {
	if t == nil {
		return nil
	}
	bare := types.Unqualify(t)
	if rec := classOf(bare); rec != nil {
		return u.typeInfo(rec)
	}
	if ptr, isPtr := bare.(*types.Pointer); isPtr && !runtimeHasTypeInfo(bare) && u.model.SizePtr == 8 {
		return u.pointerTypeInfo(bare, ptr)
	}
	name, err := mangle.TypeInfoOf(u.opt.ABI, bare)
	if err != nil {
		return nil
	}
	if s, done := u.typeInfoRefs[name]; done {
		return s
	}
	if u.typeInfoRefs == nil {
		u.typeInfoRefs = map[string]ir.Symbol{}
	}
	s := u.mod.ImportGlobal(u.symbolName(name), ir.StorePtr.FType())
	u.typeInfoRefs[name] = s
	return s
}

// runtimeHasTypeInfo reports whether libc++abi already defines a type's
// type_info. The Itanium ABI has the runtime provide them for the
// fundamental types and for pointers to them; everything else is the
// compiler's to emit.
func runtimeHasTypeInfo(t types.Type) bool {
	bare := types.Unqualify(t)
	if p, isPtr := bare.(*types.Pointer); isPtr {
		e := types.Unqualify(p.Elem)
		return types.IsArithmetic(e) || types.IsVoid(e) || types.IsNullptr(e)
	}
	return types.IsArithmetic(bare) || types.IsVoid(bare) || types.IsNullptr(bare)
}

// pointerTypeInfo emits the type_info of a pointer type the runtime does
// not provide -- a pointer to a class, most of the time.
//
// A __pointer_type_info is the runtime class's table, the name string, the
// cv-qualifiers of the type pointed to, and that type's own type_info.
func (u *unit) pointerTypeInfo(bare types.Type, ptr *types.Pointer) ir.Symbol {
	name, err := mangle.TypeInfoOf(u.opt.ABI, bare)
	if err != nil {
		return nil
	}
	if s, done := u.typeInfoRefs[name]; done {
		return s
	}
	pointee := u.typeInfoSymbol(ptr.Elem)
	if pointee == nil {
		return nil
	}
	enc := strings.TrimPrefix(name, "_ZTI")
	str := u.mod.Global(u.symbolName("_ZTS"+enc), ir.RO, ir.Array(uint64(len(enc)+1), ir.StoreI8.FType())).Export().Comdat()
	str.Init(ir.Str(enc + "\x00"))

	// __pbase_type_info's flags are the pointee's cv-qualifiers: const is
	// bit 0, volatile bit 1.
	var flags int64
	if types.IsConst(ptr.Elem) {
		flags |= 1
	}
	if types.IsVolatile(ptr.Elem) {
		flags |= 2
	}
	g := u.mod.Global(u.symbolName(name), ir.RO, ir.Array(4, ir.StorePtr.FType())).Export().Comdat()
	g.Align(8)
	if u.typeInfoRefs == nil {
		u.typeInfoRefs = map[string]ir.Symbol{}
	}
	u.typeInfoRefs[name] = g
	g.Init(ir.List(
		ir.RelocInit(u.cxxabiTable("__pointer_type_info")).Plus(ir.Int(16)),
		ir.RelocInit(str),
		ir.Lit(ir.Int(flags)),
		ir.RelocInit(pointee),
	))
	return g
}

// typeidExpr lowers typeid.
//
// For a type, and for an expression whose type is not polymorphic, the
// answer is settled here: the type_info object of the type as written,
// and the operand is not evaluated. For a glvalue of polymorphic class
// type it is the object's own dynamic type ([expr.typeid]/3), which its
// table carries one word in front of the address point.
func (fl *fn) typeidExpr(e *ast.TypeidExpr) ir.Value {
	if e.X != nil {
		operand := types.Unqualify(types.RemoveReference(fl.typeOf(e.X)))
		if rec := classOf(operand); rec != nil && types.IsPolymorphic(rec) {
			if p, _, ok := fl.lvalueQuiet(e.X); ok {
				b := fl.blk
				vptr := b.Ptr.Load(p)
				return b.Ptr.Load(b.Ptr.Add(vptr, b.I64.Const(-fl.u.model.SizePtr)))
			}
		}
		return fl.typeInfoAddr(operand, e.Pos())
	}
	return fl.typeInfoAddr(fl.u.res.Info.TypeIds[e.Type], e.Pos())
}

// typeInfoAddr is the address of a type's type_info object.
func (fl *fn) typeInfoAddr(t types.Type, at ast.Tok) ir.Value {
	sym := fl.u.typeInfoSymbol(t)
	if sym == nil {
		fl.u.errorf(at, "lowering: no type information for %s", t)
		return nil
	}
	return fl.blk.Ptr.GetAddr(sym)
}
