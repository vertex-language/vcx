package types

// Record layout under the Itanium C++ ABI ("Non-POD Class Types").
//
// Every Itanium kind of types.CXXABI lays classes out this way (Linux, macOS, bare-metal ELF).
//
// Key record layout metrics:
//	sizeof  the complete object, rounded to its alignment
//	dsize   where meaningful bytes end (the rest is padding)
//	nvsize  the class as a base subobject, without its virtual bases
//	nvalign the alignment of that
//
// A derived class starts its own members at its base's dsize rather than
// its sizeof when the base is not a POD, allowing derived members into tail padding.

// isDynamic is the ABI's "dynamic class": one that needs a virtual table
// pointer, because it has virtual functions or virtual bases of its own or
// inherited. A class whose only virtual thing is a base still has a table
// under Itanium: the base's offset is read out of it.
func isDynamic(r *Record, visited map[*Record]bool) bool {
	if r == nil || visited[r] {
		return false
	}
	visited[r] = true
	for _, meth := range r.Methods {
		if meth.Virtual {
			return true
		}
	}
	for _, b := range r.Bases {
		if b.Virtual {
			return true
		}
		if br, ok := Unqualify(b.Type).(*Record); ok && isDynamic(br, visited) {
			return true
		}
	}
	return false
}

// IsDynamic is isDynamic for callers outside the package.
func IsDynamic(r *Record) bool { return isDynamic(r, make(map[*Record]bool)) }

// itaniumEmpty is the ABI's "empty class": no non-static data members other
// than zero-width bit-fields, no virtual functions, no virtual bases, and
// only empty non-virtual bases.
func itaniumEmpty(r *Record) bool {
	if r == nil || r.Tag == TagUnion {
		return r != nil && len(r.Fields) == 0
	}
	for _, f := range r.Fields {
		if !(f.BitField && f.Width == 0) {
			return false
		}
	}
	if isDynamic(r, make(map[*Record]bool)) {
		return false
	}
	for _, b := range r.Bases {
		br, ok := Unqualify(b.Type).(*Record)
		if !ok || !itaniumEmpty(br) {
			return false
		}
	}
	return true
}

// nearlyEmpty is a dynamic class whose only content is its vptr: the one
// kind of virtual base that can share its address with the class deriving
// from it, by being that class's primary base.
func (m Model) nearlyEmpty(r *Record) bool {
	if !isDynamic(r, make(map[*Record]bool)) {
		return false
	}
	info, ok := m.layoutInfo(r)
	return ok && info.nvsize == m.SizePtr
}

// primaryBase is the base a dynamic class shares its vptr and its address
// with: the first non-virtual dynamic base in declaration order;
// failing that, the first nearly empty virtual base that is not already
// some other base's primary; failing that, the first nearly empty virtual
// base at all. It returns the index into r.Bases for a direct base, or -1
// and the record for a virtual base found anywhere in the hierarchy.
func (m Model) primaryBase(r *Record) (idx int, vb *Record) {
	if !isDynamic(r, make(map[*Record]bool)) {
		return -1, nil
	}
	for i, b := range r.Bases {
		br, ok := Unqualify(b.Type).(*Record)
		if ok && !b.Virtual && isDynamic(br, make(map[*Record]bool)) {
			return i, nil
		}
	}
	vbases := itaniumVBases(r)
	if len(vbases) == 0 {
		return -1, nil
	}
	indirect := m.indirectPrimaries(r)
	var firstNearlyEmpty *Record
	for _, v := range vbases {
		if !m.nearlyEmpty(v) {
			continue
		}
		if !indirect[v] {
			return m.directIndex(r, v), v
		}
		if firstNearlyEmpty == nil {
			firstNearlyEmpty = v
		}
	}
	if firstNearlyEmpty != nil {
		return m.directIndex(r, firstNearlyEmpty), firstNearlyEmpty
	}
	return -1, nil
}

// directIndex is where v is named as a direct virtual base of r, or -1.
func (m Model) directIndex(r, v *Record) int {
	for i, b := range r.Bases {
		if br, ok := Unqualify(b.Type).(*Record); ok && b.Virtual && br == v {
			return i
		}
	}
	return -1
}

// indirectPrimaries is every class that is the primary base of some base of
// r, at any depth. Such a class is laid out with the base it is primary to
// and does not get a place of its own among r's virtual bases.
func (m Model) indirectPrimaries(r *Record) map[*Record]bool {
	out := map[*Record]bool{}
	var walk func(*Record)
	walk = func(c *Record) {
		for _, b := range c.Bases {
			br, ok := Unqualify(b.Type).(*Record)
			if !ok {
				continue
			}
			idx, vb := m.primaryBase(br)
			if vb != nil {
				out[vb] = true
			} else if idx >= 0 {
				if pr, ok := Unqualify(br.Bases[idx].Type).(*Record); ok {
					out[pr] = true
				}
			}
			walk(br)
		}
	}
	walk(r)
	return out
}

// itaniumVBases is every virtual base of r in the ABI's inheritance graph
// order: depth-first, left to right, each at its first appearance, a base
// before the bases it in turn derives from.
func itaniumVBases(r *Record) []*Record {
	var out []*Record
	seen := map[*Record]bool{}
	var walk func(*Record)
	walk = func(c *Record) {
		for _, b := range c.Bases {
			br, ok := Unqualify(b.Type).(*Record)
			if !ok {
				continue
			}
			if b.Virtual && !seen[br] {
				seen[br] = true
				out = append(out, br)
			}
			walk(br)
		}
	}
	walk(r)
	return out
}

// podForLayout decides whether a class's tail padding is reused.
// TR1 rules apply on generic Itanium; C++11 trivial + standard-layout rules on Apple arm64.
// A POD base's padding is never reused because copying via memcpy could overwrite derived members.
func (m Model) podForLayout(r *Record) bool {
	if r.Tag == TagUnion {
		return m.podFields(r)
	}
	if m.ABI.TailPaddingPOD11() {
		return layoutTrivial(r) && layoutStandard(r)
	}
	// TR1: an aggregate -- no user-declared constructors, no private or
	// protected data, no bases, no virtual functions -- with no
	// user-declared copy assignment or destructor and only POD members.
	if len(r.Bases) > 0 || isDynamic(r, make(map[*Record]bool)) {
		return false
	}
	for _, meth := range r.Methods {
		if meth.Static || meth.Friend {
			continue
		}
		if meth.Name == r.Name || meth.Name == "~"+r.Name || (meth.Name == "operator=" && !meth.Template && isCopyAssign(r, meth)) {
			return false
		}
	}
	for _, f := range r.Fields {
		if f.Access != AccessPublic {
			return false
		}
	}
	return m.podFields(r)
}

func (m Model) podFields(r *Record) bool {
	for _, f := range r.Fields {
		t := Unqualify(f.Type)
		for {
			a, ok := t.(*Array)
			if !ok {
				break
			}
			t = Unqualify(a.Elem)
		}
		if fr, ok := t.(*Record); ok && !m.podForLayout(fr) {
			return false
		}
	}
	return true
}

func isCopyAssign(r *Record, meth *Method) bool {
	if meth.Func == nil || len(meth.Func.Params) != 1 {
		return false
	}
	p := Unqualify(RemoveReference(meth.Func.Params[0].Type))
	rec, ok := p.(*Record)
	return ok && rec == r
}

// layoutTrivial reports whether r has no user-provided special member,
// no virtual functions/bases, no default member initializer, and trivial bases/members.
func layoutTrivial(r *Record) bool {
	if isDynamic(r, make(map[*Record]bool)) {
		return false
	}
	hasCtor, hasDefault := false, false
	for _, meth := range r.Methods {
		if meth.Static || meth.Friend || meth.Template {
			continue
		}
		switch {
		case meth.Name == r.Name:
			hasCtor = true
			if meth.Func != nil && len(meth.Func.Params) == 0 {
				hasDefault = true
			}
			if !meth.Defaulted && !meth.Deleted && specialCtor(r, meth) {
				return false
			}
		case meth.Name == "~"+r.Name, meth.Name == "operator=" && isCopyAssign(r, meth):
			if !meth.Defaulted && !meth.Deleted {
				return false
			}
		}
	}
	if hasCtor && !hasDefault {
		return false // no default constructor at all is not a trivial one
	}
	for _, f := range r.Fields {
		if f.HasInit {
			return false
		}
		if fr := recordElem(f.Type); fr != nil && !layoutTrivial(fr) {
			return false
		}
	}
	for _, b := range r.Bases {
		if br, ok := Unqualify(b.Type).(*Record); ok && !layoutTrivial(br) {
			return false
		}
	}
	return true
}

// specialCtor is a default, copy or move constructor.
func specialCtor(r *Record, meth *Method) bool {
	if meth.Func == nil {
		return false
	}
	switch len(meth.Func.Params) {
	case 0:
		return true
	case 1:
		p := Unqualify(RemoveReference(meth.Func.Params[0].Type))
		rec, ok := p.(*Record)
		return ok && rec == r
	}
	return false
}

// layoutStandard reports whether r has standard layout: no virtual functions/bases,
// uniform access control for non-static data members in a single class, and standard bases/members.
func layoutStandard(r *Record) bool {
	if isDynamic(r, make(map[*Record]bool)) {
		return false
	}
	for i, f := range r.Fields {
		if i > 0 && f.Access != r.Fields[0].Access {
			return false
		}
		if IsReference(f.Type) {
			return false
		}
		if fr := recordElem(f.Type); fr != nil && !layoutStandard(fr) {
			return false
		}
	}
	withData := 0
	if len(r.Fields) > 0 {
		withData++
	}
	for _, b := range r.Bases {
		br, ok := Unqualify(b.Type).(*Record)
		if !ok {
			continue
		}
		if !layoutStandard(br) {
			return false
		}
		if hasDataInHierarchy(br) {
			withData++
		}
	}
	return withData <= 1
}

func hasDataInHierarchy(r *Record) bool {
	if len(r.Fields) > 0 {
		return true
	}
	for _, b := range r.Bases {
		if br, ok := Unqualify(b.Type).(*Record); ok && hasDataInHierarchy(br) {
			return true
		}
	}
	return false
}

// recordElem is the class a member's type is, through any arrays.
func recordElem(t Type) *Record {
	t = Unqualify(t)
	for {
		a, ok := t.(*Array)
		if !ok {
			break
		}
		t = Unqualify(a.Elem)
	}
	r, _ := t.(*Record)
	return r
}

// emptyMap tracks placed empty subobjects by type. Distinct subobjects
// of the same type must have different addresses, so overlapping empty subobjects
// of matching type are offset by alignment.
type emptyMap map[int64][]*Record

func (e emptyMap) conflicts(m Model, r *Record, at int64, asBase bool) bool {
	hit := false
	m.eachEmpty(r, at, asBase, func(c *Record, off int64) {
		for _, have := range e[off] {
			if have == c {
				hit = true
			}
		}
	})
	return hit
}

func (e emptyMap) add(m Model, r *Record, at int64, asBase bool) {
	m.eachEmpty(r, at, asBase, func(c *Record, off int64) {
		e[off] = append(e[off], c)
	})
}

// eachEmpty visits every empty class subobject of r placed at `at`: r
// itself if it is empty, its bases, and its members of class type. A base
// subobject has no virtual bases of its own inside it; a complete object,
// which a member is, does.
func (m Model) eachEmpty(r *Record, at int64, asBase bool, visit func(*Record, int64)) {
	if itaniumEmpty(r) {
		visit(r, at)
	}
	info, ok := m.layoutInfo(r)
	if !ok {
		return
	}
	for i, b := range r.Bases {
		br, ok := Unqualify(b.Type).(*Record)
		if !ok || b.Virtual {
			continue
		}
		m.eachEmpty(br, at+info.baseOffsets[i], true, visit)
	}
	if !asBase {
		for _, vb := range itaniumVBases(r) {
			if off, ok := info.vbaseOffsets[vb]; ok {
				m.eachEmpty(vb, at+off, true, visit)
			}
		}
	}
	for i, f := range r.Fields {
		t := Unqualify(f.Type)
		n := int64(1)
		for {
			a, ok := t.(*Array)
			if !ok {
				break
			}
			n *= a.Len
			t = Unqualify(a.Elem)
		}
		fr, ok := t.(*Record)
		if !ok {
			continue
		}
		sz, _ := m.Sizeof(fr)
		for k := int64(0); k < n; k++ {
			m.eachEmpty(fr, at+info.fieldOffsets[i]+k*sz, false, visit)
		}
	}
}

// layoutItanium performs class layout according to the Itanium C++ ABI.
func (m Model) layoutItanium(r *Record) (recordInfo, bool) {
	info := recordInfo{vbptr: -1, vbaseOffsets: map[*Record]int64{}}
	info.fieldOffsets = make([]int64, len(r.Fields))
	info.baseOffsets = make([]int64, len(r.Bases))
	info.bitOffsets = make([]int64, len(r.Fields))
	info.bitUnits = make([]int64, len(r.Fields))

	align := int64(1)
	if r.Align > align {
		// A class-head alignas is part of the class's alignment from the
		// start, so it is the base-subobject alignment too.
		align = r.Align
	}
	var dsize, size int64 // bytes
	empties := emptyMap{}

	grow := func(end int64) {
		if end > size {
			size = end
		}
	}

	if r.Tag == TagUnion {
		for i, f := range r.Fields {
			fs, ok1 := m.Sizeof(f.Type)
			fa, ok2 := m.Alignof(f.Type)
			if !ok1 || !ok2 {
				return info, false
			}
			if f.Align > fa {
				fa = f.Align
			}
			fa = r.MemberAlign(fa)
			if fa > align && (!f.BitField || f.Name != "") {
				align = fa
			}
			if f.BitField {
				fs = (f.Width + 7) / 8
				info.bitUnits[i] = fs
			}
			info.fieldOffsets[i] = 0
			grow(fs)
		}
		size = roundUp(size, align)
		if size == 0 {
			size = 1
		}
		info.size, info.align, info.nvsize, info.nvalign, info.dsize = size, align, size, align, size
		return info, true
	}

	// II.1: the primary base, or the vptr.
	primaryIdx, primaryVirtual := m.primaryBase(r)
	info.primary = primaryIdx
	info.primaryVirtual = primaryVirtual
	if isDynamic(r, make(map[*Record]bool)) && primaryIdx < 0 {
		dsize, size = m.SizePtr, m.SizePtr
		if m.SizePtr > align {
			align = m.SizePtr
		}
	}

	placeBase := func(br *Record, first bool) (int64, bool) {
		bi, ok := m.layoutInfo(br)
		if !ok {
			return 0, false
		}
		if bi.nvalign > align {
			align = bi.nvalign
		}
		if itaniumEmpty(br) {
			off := int64(0)
			if !first {
				for empties.conflicts(m, br, off, true) {
					if off == 0 {
						off = dsize
					} else {
						off += bi.nvalign
					}
					if off < dsize {
						off = dsize
					}
				}
			}
			empties.add(m, br, off, true)
			grow(off + bi.size)
			return off, true
		}
		off := roundUp(dsize, bi.nvalign)
		if first {
			off = 0
		}
		for empties.conflicts(m, br, off, true) {
			off += bi.nvalign
		}
		empties.add(m, br, off, true)
		dsize = off + bi.nvsize
		grow(dsize)
		return off, true
	}

	if primaryIdx >= 0 {
		br := Unqualify(r.Bases[primaryIdx].Type).(*Record)
		off, ok := placeBase(br, true)
		if !ok {
			return info, false
		}
		info.baseOffsets[primaryIdx] = off
		if primaryVirtual != nil {
			info.vbaseOffsets[primaryVirtual] = off
		}
	}

	// II.2: the other non-virtual bases, in declaration order.
	for i, b := range r.Bases {
		if i == primaryIdx || b.Virtual {
			continue
		}
		br, ok := Unqualify(b.Type).(*Record)
		if !ok || !br.Complete {
			continue
		}
		off, ok := placeBase(br, false)
		if !ok {
			return info, false
		}
		info.baseOffsets[i] = off
	}

	// II.3: the members. dsize is tracked in bits while a run of bit-fields
	// is open, since the next one may start in the middle of a byte.
	bits := dsize * 8
	for i, f := range r.Fields {
		fs, ok1 := m.Sizeof(f.Type)
		fa, ok2 := m.Alignof(f.Type)
		if !ok1 || !ok2 {
			return info, false
		}
		if f.Align > fa {
			fa = f.Align
		}
		fa = r.MemberAlign(fa)

		if f.BitField {
			unitBits := fs * 8
			alignBits := fa * 8
			o := bits
			if f.Width == 0 {
				o = roundUp(o, alignBits)
			} else if f.Width <= unitBits && o%alignBits+f.Width > unitBits {
				o = roundUp(o, alignBits)
			}
			if f.Name != "" && fa > align {
				align = fa
			}
			unit := o / alignBits * alignBits / 8
			if unitBits > 0 && unit*8+unitBits < o+f.Width {
				unit = (o + f.Width - unitBits + 7) / 8
			}
			info.fieldOffsets[i] = unit
			info.bitOffsets[i] = o - unit*8
			info.bitUnits[i] = fs
			bits = o + f.Width
			grow((bits + 7) / 8)
			dsize = (bits + 7) / 8
			continue
		}

		if fa > align {
			align = fa
		}
		off := roundUp((bits+7)/8, fa)
		if fr := recordElem(f.Type); fr != nil {
			for empties.conflicts(m, fr, off, false) {
				off += fa
			}
			if at, ok := Unqualify(f.Type).(*Record); ok {
				empties.add(m, at, off, false)
			}
		}
		info.fieldOffsets[i] = off
		dsize = off + fs
		bits = dsize * 8
		grow(dsize)
	}

	// II.4: the non-virtual size is the class as a base.
	info.nvsize = size
	info.nvalign = align
	if info.nvsize == 0 {
		info.nvsize = 1
	}

	// II.5: the virtual bases, each once, the indirect primaries with the
	// base that made them primary.
	if vbases := itaniumVBases(r); len(vbases) > 0 {
		indirect := m.indirectPrimaries(r)
		for _, vb := range vbases {
			if vb == primaryVirtual {
				continue
			}
			if indirect[vb] {
				continue
			}
			off, ok := placeBase(vb, false)
			if !ok {
				return info, false
			}
			info.vbaseOffsets[vb] = off
		}
		// An indirect primary shares its address with the base whose
		// primary it is, wherever that base ended up.
		for _, vb := range vbases {
			if _, placed := info.vbaseOffsets[vb]; placed || !indirect[vb] {
				continue
			}
			if at, ok := m.primaryHolderOffset(r, vb, info); ok {
				info.vbaseOffsets[vb] = at
			}
		}
		for i, b := range r.Bases {
			if br, ok := Unqualify(b.Type).(*Record); ok && b.Virtual {
				info.baseOffsets[i] = info.vbaseOffsets[br]
			}
		}
	}

	info.dsize = dsize
	size = roundUp(size, align)
	if size == 0 {
		size = 1
	}
	info.size, info.align = size, align

	if m.podForLayout(r) {
		info.dsize = size
		info.nvsize = roundUp(info.nvsize, align)
	}
	return info, true
}

// primaryHolderOffset is where, in r, the class whose primary base vb is
// was placed -- which is where vb is too.
func (m Model) primaryHolderOffset(r, vb *Record, info recordInfo) (int64, bool) {
	var found int64
	ok := false
	var walk func(c *Record, at int64, ci recordInfo)
	walk = func(c *Record, at int64, ci recordInfo) {
		if ok {
			return
		}
		for i, b := range c.Bases {
			br, isRec := Unqualify(b.Type).(*Record)
			if !isRec {
				continue
			}
			bat := at + ci.baseOffsets[i]
			if b.Virtual {
				off, placed := info.vbaseOffsets[br]
				if !placed {
					continue
				}
				bat = off
			}
			if _, pv := m.primaryBase(br); pv == vb {
				found, ok = bat, true
				return
			}
			if bi, good := m.layoutInfo(br); good {
				walk(br, bat, bi)
			}
		}
	}
	walk(r, 0, info)
	return found, ok
}
