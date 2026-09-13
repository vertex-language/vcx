package types

// Virtual base tables (vbtables) and layout for virtual base subobjects under the MSVC ABI.

// VBTable is one virtual base table.
type VBTable struct {
	// Base names the base subobject whose vbptr this table belongs to, and
	// is empty for one the class introduced itself.
	Base string

	// Offset is where that vbptr sits in the complete object.
	Offset int64

	// Entries[0] is the offset from the vbptr back to the start of its
	// owning subobject, negated. Entries[i] for i > 0 is the offset from the
	// vbptr to VirtualBases(r)[i-1].
	Entries []int64
}

// VirtualBases returns all unique virtual bases in depth-first left-to-right order.
func VirtualBases(r *Record) []*Record {
	var out []*Record
	seen := map[*Record]bool{}

	var walk func(*Record, map[*Record]bool)
	walk = func(cur *Record, visiting map[*Record]bool) {
		if cur == nil || visiting[cur] {
			return
		}
		visiting[cur] = true
		for _, b := range cur.Bases {
			br, ok := Unqualify(b.Type).(*Record)
			if !ok {
				continue
			}
			walk(br, visiting)
			if b.Virtual && !seen[br] {
				seen[br] = true
				out = append(out, br)
			}
		}
	}
	walk(r, map[*Record]bool{})
	return out
}

// hasOwnVBPtr reports whether a class introduces a vbptr not inherited from a base.
func hasOwnVBPtr(r *Record) bool {
	if len(VirtualBases(r)) == 0 {
		return false
	}
	for _, b := range r.Bases {
		if b.Virtual {
			continue
		}
		if br, ok := Unqualify(b.Type).(*Record); ok && len(VirtualBases(br)) > 0 {
			return false // the base's pointer serves
		}
	}
	return true
}

// vbSite is one virtual base table pointer in an object: which class put it
// there, where it sits in the complete object, and where it sits inside its
// own class.
type vbSite struct {
	owner  *Record
	at     int64 // offset in the complete object
	within int64 // offset inside owner
}

// vbptrSites collects every vbptr in an object's hierarchy in layout order.
func (m Model) vbptrSites(r *Record, at int64, out *[]vbSite) {
	if r == nil {
		return
	}
	info, ok := m.layoutInfo(r)
	if !ok {
		return
	}
	if info.vbptr >= 0 {
		*out = append(*out, vbSite{owner: r, at: at + info.vbptr, within: info.vbptr})
		return // a class with its own pointer has no base that brought one
	}
	for _, i := range baseOrder(r) {
		b := r.Bases[i]
		if b.Virtual {
			continue
		}
		if br, isRec := Unqualify(b.Type).(*Record); isRec {
			m.vbptrSites(br, at+info.baseOffsets[i], out)
		}
	}
}

// VBTables computes every virtual base table for a class in layout order.
func (m Model) VBTables(r *Record) []VBTable {
	if r == nil || m.ABI.IsItanium() {
		// Itanium keeps the same offsets in the virtual table itself; see
		// VTable.VBaseOffsets.
		return nil
	}
	vbases := VirtualBases(r)
	if len(vbases) == 0 {
		return nil
	}
	info, ok := m.layoutInfo(r)
	if !ok {
		return nil
	}

	var sites []vbSite
	m.vbptrSites(r, 0, &sites)

	tables := make([]VBTable, 0, len(sites))
	for _, site := range sites {
		// The first entry walks back from the pointer to the top of the
		// subobject that owns it, which is how code holding only the pointer
		// finds the object it belongs to.
		t := VBTable{Offset: site.at, Entries: []int64{-site.within}}
		if site.owner != r {
			t.Base = site.owner.Name
		}
		for _, vb := range vbases {
			t.Entries = append(t.Entries, info.vbaseOffsets[vb]-site.at)
		}
		tables = append(tables, t)
	}
	return tables
}
