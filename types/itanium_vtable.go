package types

// Virtual tables under the Itanium C++ ABI.
//
// A class has one vtable group: its primary table (shared with its primary base chain),
// secondary tables for other dynamic base subobjects, and tables for virtual bases.
//
// Key Itanium vtable properties:
//   - A virtual destructor consists of two entries: complete-object and deleting.
//   - Overriding a secondary base's virtual adds an entry in the primary table and a thunk in the secondary table.
//   - Pointer adjustments are performed by thunks in secondary tables.

// vtablesItanium is the vtable group of a class, primary table first.
func (m Model) vtablesItanium(r *Record) []VTable {
	if r == nil || !isDynamic(r, make(map[*Record]bool)) {
		return nil
	}
	info, ok := m.layoutInfo(r)
	if !ok {
		return nil
	}
	g := &vtableGroup{m: m, complete: r, info: info, primaryVirtual: m.primaryVirtualBases(r)}
	g.primaryAndSecondary(subobject{rec: r, path: []*Record{r}})
	seen := map[*Record]bool{}
	g.virtualBases(r, seen)
	return g.tables
}

type vtableGroup struct {
	m        Model
	complete *Record
	info     recordInfo
	tables   []VTable

	// primaryVirtual is every virtual base that is some class's primary
	// base in this hierarchy, and so shares a table rather than having one.
	primaryVirtual map[*Record]bool
}

// subobject is one base subobject of the complete class: its class, where
// it sits, and the chain of classes from the complete object down to it,
// which is where its final overriders are looked for.
type subobject struct {
	rec     *Record
	offset  int64
	path    []*Record
	virtual bool
}

// primaryAndSecondary lays out a subobject's primary table and then the
// tables of its other bases.
func (g *vtableGroup) primaryAndSecondary(s subobject) {
	t := VTable{Offset: s.offset}
	for _, vb := range itaniumVBases(s.rec) {
		if at, ok := g.info.vbaseOffsets[vb]; ok {
			t.VBaseOffsets = append(t.VBaseOffsets, at-s.offset)
		}
	}
	if s.offset != 0 {
		t.Base = s.rec.Name
	}
	for _, slot := range g.m.itaniumSlots(s.rec) {
		g.finalOverrider(&slot, s)
		t.Slots = append(t.Slots, slot)
	}
	g.tables = append(g.tables, t)
	g.secondary(s)
}

// secondary lays out the tables of a subobject's non-virtual dynamic bases.
// The primary base shares the subobject's table, but its own bases may
// still need theirs.
func (g *vtableGroup) secondary(s subobject) {
	info, ok := g.m.layoutInfo(s.rec)
	if !ok {
		return
	}
	for i, b := range s.rec.Bases {
		br, ok := Unqualify(b.Type).(*Record)
		if !ok || b.Virtual || !isDynamic(br, make(map[*Record]bool)) {
			continue
		}
		sub := subobject{
			rec:    br,
			offset: s.offset + info.baseOffsets[i],
			path:   append(append([]*Record(nil), s.path...), br),
		}
		if i == info.primary && info.primaryVirtual == nil {
			g.secondary(sub)
			continue
		}
		g.primaryAndSecondary(sub)
	}
}

// virtualBases lays out the tables of every virtual base that is not a
// primary base somewhere, in inheritance graph order.
func (g *vtableGroup) virtualBases(r *Record, seen map[*Record]bool) {
	for _, b := range r.Bases {
		br, ok := Unqualify(b.Type).(*Record)
		if !ok {
			continue
		}
		if b.Virtual && isDynamic(br, make(map[*Record]bool)) && !g.primaryVirtual[br] && !seen[br] {
			seen[br] = true
			g.primaryAndSecondary(subobject{
				rec:     br,
				offset:  g.info.vbaseOffsets[br],
				path:    []*Record{g.complete, br},
				virtual: true,
			})
		}
		if len(itaniumVBases(br)) > 0 {
			g.virtualBases(br, seen)
		}
	}
}

// primaryVirtualBases is every virtual base that is the primary base of the
// class or of any class in its hierarchy.
func (m Model) primaryVirtualBases(r *Record) map[*Record]bool {
	out := map[*Record]bool{}
	var walk func(*Record)
	walk = func(c *Record) {
		if _, vb := m.primaryBase(c); vb != nil {
			out[vb] = true
		}
		for _, b := range c.Bases {
			if br, ok := Unqualify(b.Type).(*Record); ok {
				walk(br)
			}
		}
	}
	walk(r)
	return out
}

// itaniumSlots is a class's primary table as the class itself sees it: the
// primary base's entries with this class's overrides in them, then the
// virtual functions this class introduces in declaration order -- where
// "introduces" means overrides nothing in the primary chain, so an override
// of a secondary base's function is new here.
func (m Model) itaniumSlots(r *Record) []VSlot {
	var slots []VSlot
	idx, pv := m.primaryBase(r)
	var primary *Record
	switch {
	case pv != nil:
		primary = pv
	case idx >= 0:
		primary, _ = Unqualify(r.Bases[idx].Type).(*Record)
	}
	if primary != nil {
		slots = append(slots, m.itaniumSlots(primary)...)
		for i := range slots {
			overrideSlot(r, &slots[i])
		}
	}

	sawDtor := false
	for _, meth := range r.Methods {
		if meth.Static || meth.Friend || meth.Template {
			continue
		}
		name := virtualName(meth, r)
		if name == "" {
			continue
		}
		if !meth.Virtual && !OverridesVirtual(r, meth.Name, meth.Func) {
			continue
		}
		if name == "{dtor}" {
			sawDtor = true
		}
		taken := false
		for _, s := range slots {
			if overrides(meth, s, r) {
				taken = true
				break
			}
		}
		if taken {
			continue
		}
		slot := VSlot{
			Name:       name,
			Introduced: r.Name,
			Definer:    r.Name,
			Pure:       meth.PureVirtual,
			Signature:  meth.Func,
		}
		slots = append(slots, slot)
		if name == "{dtor}" {
			slot.Deleting = true
			slots = append(slots, slot)
		}
	}

	// A destructor nobody declared still overrides a base's virtual one,
	// and if that base is not in the primary chain the override is new
	// here too -- after everything declared, where the implicit
	// declaration lands.
	if !sawDtor && hasVirtualDtor(r) {
		have := false
		for _, s := range slots {
			if s.Name == "{dtor}" {
				have = true
			}
		}
		if !have {
			slot := VSlot{Name: "{dtor}", Introduced: r.Name, Definer: r.Name}
			slots = append(slots, slot)
			slot.Deleting = true
			slots = append(slots, slot)
		}
	}
	return slots
}

// overrideSlot puts r's overrider in a slot, if r has one. Every class has
// a destructor, declared or not, so a destructor's slot always ends up
// holding the class's own.
func overrideSlot(r *Record, s *VSlot) {
	if s.Name == "{dtor}" {
		s.Definer, s.Pure = r.Name, false
		return
	}
	for _, meth := range r.Methods {
		if overrides(meth, *s, r) {
			s.Definer, s.Pure = r.Name, meth.PureVirtual
			return
		}
	}
}

// finalOverrider substitutes, into a slot of a subobject's table, the most
// derived definition the complete object has for it, and records how far a
// call through this table has to move `this` to reach it.
func (g *vtableGroup) finalOverrider(slot *VSlot, s subobject) {
	candidates := s.path
	if s.virtual {
		// A virtual base is reached along every path that leads to it, and
		// its overrider may be on any of them.
		candidates = g.derivedFrom(s.rec)
	}
	for k := len(candidates) - 2; k >= 0; k-- {
		overrideSlot(candidates[k], slot)
	}
	definer := findBase(g.complete, slot.Definer)
	if slot.Definer == g.complete.Name {
		definer = g.complete
	}
	if definer != nil {
		if off, ok := g.m.itaniumBaseOffset(g.complete, definer, g.info); ok {
			slot.Adjust = s.offset - off
		}
	}
}

// derivedFrom is every class in the complete object's hierarchy that has vb
// as a base, most derived last-but-one before vb itself, in an order where
// each class comes before its own bases.
func (g *vtableGroup) derivedFrom(vb *Record) []*Record {
	var out []*Record
	seen := map[*Record]bool{}
	var walk func(*Record) bool
	walk = func(c *Record) bool {
		if c == vb {
			return true
		}
		has := false
		for _, b := range c.Bases {
			if br, ok := Unqualify(b.Type).(*Record); ok && walk(br) {
				has = true
			}
		}
		if has && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
		return has
	}
	walk(g.complete)
	// out is bases before the classes derived from them; overriders are
	// applied least derived first, so reverse into most derived first and
	// let finalOverrider walk it back.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return append(out, vb)
}

// itaniumBaseOffset is where a base class's subobject sits in r, through
// virtual bases as well as non-virtual ones.
func (m Model) itaniumBaseOffset(r, base *Record, info recordInfo) (int64, bool) {
	if r == base {
		return 0, true
	}
	if off, ok := info.vbaseOffsets[base]; ok {
		return off, true
	}
	for i, b := range r.Bases {
		br, ok := Unqualify(b.Type).(*Record)
		if !ok {
			continue
		}
		bi, ok := m.layoutInfo(br)
		if !ok {
			continue
		}
		at := info.baseOffsets[i]
		if b.Virtual {
			at = info.vbaseOffsets[br]
		}
		if br == base {
			return at, true
		}
		if off, found := m.itaniumBaseOffset(br, base, bi); found && !b.Virtual {
			return at + off, true
		}
	}
	return 0, false
}

// HasVirtualDtor is hasVirtualDtor for callers outside the package.
func HasVirtualDtor(r *Record) bool { return hasVirtualDtor(r) }

// hasVirtualDtor reports whether any class in r's hierarchy declares its
// destructor virtual.
func hasVirtualDtor(r *Record) bool {
	for _, meth := range r.Methods {
		if meth.Name == "~"+r.Name && meth.Virtual {
			return true
		}
	}
	for _, b := range r.Bases {
		if br, ok := Unqualify(b.Type).(*Record); ok && hasVirtualDtor(br) {
			return true
		}
	}
	return false
}
