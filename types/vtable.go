package types

// Virtual table layout: slot ordering, method overrides, and `this` pointer adjustments.

// VSlot is one entry in a virtual table.
type VSlot struct {
	// Name is the function's name; a destructor is named "{dtor}", which is
	// what cl calls it and what avoids inventing a spelling of our own.
	Name string

	// Introduced is the class that first declared this function virtual --
	// the one that owns the slot number.
	Introduced string

	// Definer is the class whose definition this slot holds, which is the
	// most-derived override.
	Definer string

	// Adjust is what a call through this table subtracts from `this` before
	// entering the function, and is the base's offset for a secondary table.
	Adjust int64

	Pure bool

	// Deleting marks the second of the two slots Itanium gives a virtual
	// destructor: the one that frees the object after destroying it. The
	// first, named the same, is the complete-object destructor. Microsoft
	// has one slot, a deleting destructor that takes a flag, and never sets
	// this.
	Deleting bool

	// Signature is the function the slot was introduced with, so that an
	// overload of the same name in a derived class does not claim it.
	Signature *Func
}

// VTable is one virtual function table of a class.
// A table's Base is the base whose slots it begins with: the primary
// base for the primary table, empty when the class has none, and the
// secondary base for each other table. cl labels a table with it only when
// there is more than one, and so does the symbol name.
type VTable struct {
	// Base names the base subobject this table serves, and is empty for the
	// primary table.
	Base string

	// Offset is where in the complete object this table's pointer sits.
	Offset int64

	// VBaseOffsets is Itanium's vbase_offset entries, the ones in front of
	// a table's offset-to-top: where each virtual base sits relative to
	// this table's subobject, in the order clang prints them. Microsoft
	// keeps these in a VBTable instead and leaves this empty.
	VBaseOffsets []int64

	Slots []VSlot
}

// VTables computes every virtual table of a class, primary first.
//
// A class that is not polymorphic has none, which is not the same as having
// an empty one: the difference is whether the object carries a pointer.
func (m Model) VTables(r *Record) []VTable {
	if m.ABI.IsItanium() {
		return m.vtablesItanium(r)
	}
	if r == nil || !isPolymorphic(r, make(map[*Record]bool)) {
		return nil
	}

	baseOffs := make([]int64, len(r.Bases))
	m.LayoutWithBases(r, make([]int64, len(r.Fields)), baseOffs)

	order := baseOrder(r)
	var primaryIdx = -1
	for _, i := range order {
		br, ok := Unqualify(r.Bases[i].Type).(*Record)
		if ok && !r.Bases[i].Virtual && isPolymorphic(br, make(map[*Record]bool)) {
			primaryIdx = i
			break
		}
	}

	// The primary table: the primary base's slots, then this class's own
	// new virtuals appended after them.
	primary := VTable{}
	if primaryIdx >= 0 {
		base, _ := Unqualify(r.Bases[primaryIdx].Type).(*Record)
		primary.Base = base.Name
		if sub := m.VTables(base); len(sub) > 0 {
			primary.Slots = append(primary.Slots, sub[0].Slots...)
		}
	}
	applyOverrides(r, primary.Slots)
	primary.Slots = append(primary.Slots, newVirtuals(m, r, primary.Slots)...)

	tables := []VTable{primary}

	// One more table per secondary polymorphic base. Its slots are that
	// base's, with this class's overrides substituted in, and every call
	// through it walks back to the complete object first.
	for _, i := range order {
		if i == primaryIdx || r.Bases[i].Virtual {
			continue
		}
		base, ok := Unqualify(r.Bases[i].Type).(*Record)
		if !ok || !isPolymorphic(base, make(map[*Record]bool)) {
			continue
		}
		sub := m.VTables(base)
		if len(sub) == 0 {
			continue
		}
		slots := append([]VSlot(nil), sub[0].Slots...)
		applyOverrides(r, slots)
		for j := range slots {
			slots[j].Adjust = baseOffs[i]
		}
		tables = append(tables, VTable{Base: base.Name, Offset: baseOffs[i], Slots: slots})
	}

	return tables
}

// applyOverrides replaces each slot's definition with this class's, where
// this class declares a function that overrides it. The slot number does not
// move: that is what makes a call through a base pointer find the derived
// definition.
func applyOverrides(r *Record, slots []VSlot) {
	for i := range slots {
		for _, meth := range r.Methods {
			if !overrides(meth, slots[i], r) {
				continue
			}
			slots[i].Definer = r.Name
			slots[i].Pure = meth.PureVirtual
			break
		}
	}
}

// newVirtuals collects virtual functions introduced by this class.
// Overloads are grouped; on Windows MSVC, overloads are ordered in reverse declaration order.
func newVirtuals(m Model, r *Record, existing []VSlot) []VSlot {
	var order []string
	groups := map[string][]*Method{}

	for _, meth := range r.Methods {
		if !meth.Virtual {
			continue
		}
		name := virtualName(meth, r)
		if name == "" {
			continue
		}
		// Overriding methods do not introduce new slots.
		taken := false
		for _, s := range existing {
			if overrides(meth, s, r) {
				taken = true
				break
			}
		}
		if !taken && OverridesVirtual(r, meth.Name, meth.Func) {
			taken = true
		}
		if taken {
			continue
		}
		if _, seen := groups[name]; !seen {
			order = append(order, name)
		}
		groups[name] = append(groups[name], meth)
	}

	var out []VSlot
	for _, name := range order {
		group := groups[name]
		if m.ABI.IsMicrosoft() {
			for i, j := 0, len(group)-1; i < j; i, j = i+1, j-1 {
				group[i], group[j] = group[j], group[i]
			}
		}
		for _, meth := range group {
			out = append(out, VSlot{
				Name:       name,
				Introduced: r.Name,
				Definer:    r.Name,
				Pure:       meth.PureVirtual,
				Signature:  meth.Func,
			})
		}
	}
	return out
}

// overrides reports whether a member function overrides the virtual slot.
func overrides(meth *Method, slot VSlot, r *Record) bool {
	if virtualName(meth, r) != slot.Name {
		return false
	}
	if slot.Signature == nil || meth.Func == nil {
		return true
	}
	return sameSignature(meth.Func, slot.Signature)
}

// SameSignature is sameSignature for callers outside the package: the test
// that tells two overloads of one name apart.
func SameSignature(a, b *Func) bool { return sameSignature(a, b) }

// sameSignature tests parameter types, cv-qualification, and ref-qualifiers.
func sameSignature(a, b *Func) bool {
	if a == nil || b == nil {
		return a == b
	}
	if len(a.Params) != len(b.Params) ||
		a.Variadic != b.Variadic ||
		a.Quals != b.Quals ||
		a.RefQual != b.RefQual {
		return false
	}
	for i := range a.Params {
		if !typeEqual(a.Params[i].Type, b.Params[i].Type) {
			return false
		}
	}
	return true
}

func typeEqual(a, b Type) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(b)
}

// virtualName is the name a slot is known by. A destructor is "{dtor}",
// because every class's is spelled differently -- ~A, ~B -- and they all
// occupy the same slot.
func virtualName(meth *Method, r *Record) string {
	if meth.Name == "~"+r.Name {
		return "{dtor}"
	}
	return meth.Name
}

// OverridesVirtual reports whether a member function overrides a virtual function from a base.
func OverridesVirtual(r *Record, name string, fn *Func) bool {
	for _, b := range r.Bases {
		base, isRec := Unqualify(b.Type).(*Record)
		if !isRec {
			continue
		}
		for _, meth := range base.Methods {
			if virtualNameOf(meth.Name, base) != virtualNameOf(name, r) {
				continue
			}
			if !sameSignature(meth.Func, fn) {
				continue
			}
			if meth.Virtual || OverridesVirtual(base, meth.Name, meth.Func) {
				return true
			}
		}
		if OverridesVirtual(base, name, fn) {
			return true
		}
	}
	return false
}

func virtualNameOf(name string, r *Record) string {
	if name == "~"+r.Name {
		return "{dtor}"
	}
	return name
}

// ThisOffset returns the subobject offset expected for `this` when calling a virtual function.
func (m Model) ThisOffset(d *Record, name string, fn *Func) int64 {
	if m.ABI.IsItanium() {
		// Itanium folds nothing into the callee: a function expects its
		// own class, and a secondary table reaches it through a thunk.
		return 0
	}
	for _, table := range m.VTables(d) {
		for _, slot := range table.Slots {
			if slot.Definer == d.Name && slot.Name == virtualNameOf(name, d) && sameSignature(slot.Signature, fn) {
				return table.Offset
			}
		}
	}
	return 0
}

// ThunkAdjust is the distance a slot's thunk moves `this`, or zero where
// the slot holds the function itself.
//
// The table serves the subobject at its Offset and passes that address;
// the function expects the subobject its definer's ThisOffset names, which
// sits at the definer's offset in this class plus that. When the two agree
// there is nothing to adjust.
func (m Model) ThunkAdjust(r *Record, table VTable, slot VSlot) int64 {
	if slot.Pure {
		return 0
	}
	if m.ABI.IsItanium() {
		return slot.Adjust
	}
	definer := findBase(r, slot.Definer)
	if definer == nil {
		return 0
	}
	offD, ok := m.BaseOffset(r, definer)
	if !ok {
		return 0
	}
	name := slot.Name
	if name == "{dtor}" {
		name = "~" + definer.Name
	}
	expects := offD + m.ThisOffset(definer, name, slot.Signature)
	return table.Offset - expects
}

// BaseOffset is where a base subobject sits in a class, through any depth
// of non-virtual derivation.
func (m Model) BaseOffset(r, base *Record) (int64, bool) {
	if r == base {
		return 0, true
	}
	baseOffs := make([]int64, len(r.Bases))
	m.LayoutWithBases(r, make([]int64, len(r.Fields)), baseOffs)
	for i, b := range r.Bases {
		br, ok := Unqualify(b.Type).(*Record)
		if !ok || b.Virtual {
			continue
		}
		if off, found := m.BaseOffset(br, base); found {
			return baseOffs[i] + off, true
		}
	}
	return 0, false
}

// FindBase walks a class and its bases for the one with a given name --
// the definer a slot names, somewhere above the class whose table it is.
func FindBase(r *Record, name string) *Record { return findBase(r, name) }

// findBase walks a class and its bases for the one with a given name.
func findBase(r *Record, name string) *Record {
	if r == nil {
		return nil
	}
	if r.Name == name {
		return r
	}
	for _, b := range r.Bases {
		if br, ok := Unqualify(b.Type).(*Record); ok {
			if found := findBase(br, name); found != nil {
				return found
			}
		}
	}
	return nil
}
