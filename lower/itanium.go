package lower

import (
	"strconv"

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
