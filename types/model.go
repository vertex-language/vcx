package types

// Target data models, sizing, alignment, and record layout.

// Model defines target-specific primitive sizes, alignments, and layout rules.
type Model struct {
	CharSigned      bool
	WCharKind       Kind
	SizeShort       int64
	SizeInt         int64
	SizeLong        int64
	SizeLongLong    int64
	SizePtr         int64
	SizeFloat       int64
	SizeDouble      int64
	SizeLongDouble  int64
	AlignLongDouble int64
	MSBitfields     bool

	// ABI is the C++ ABI: layout, tables, structors, member pointers. The
	// sizes above are the data model and say nothing about it.
	ABI CXXABI

	// VaList is what a va_list is under the target's calling convention.
	VaList VaListKind
}

// VaListKind is the shape of __builtin_va_list, which the calling
// convention decides and every <stdarg.h> is built on.
type VaListKind uint8

const (
	// VaListPointer is a plain char *: Windows, i386, and Apple's arm64,
	// which passes every variadic argument on the stack.
	VaListPointer VaListKind = iota

	// VaListX86_64 is SysV x86-64's `__va_list_tag[1]`: offsets into the
	// register save area and two pointers.
	VaListX86_64

	// VaListAArch64 is AAPCS64's `std::__va_list`: three pointers and two
	// offsets.
	VaListAArch64
)

// LP64 returns the target model for x86-64/aarch64 Linux and macOS.
func LP64() Model {
	return Model{
		CharSigned:      true,
		WCharKind:       Int,
		SizeShort:       2,
		SizeInt:         4,
		SizeLong:        8,
		SizeLongLong:    8,
		SizePtr:         8,
		SizeFloat:       4,
		SizeDouble:      8,
		SizeLongDouble:  16,
		AlignLongDouble: 16,
		MSBitfields:     false,
		ABI:             ItaniumGeneric,
		VaList:          VaListX86_64,
	}
}

// LLP64 returns the target model for x86-64 Windows.
func LLP64() Model {
	return Model{
		CharSigned:      true,
		WCharKind:       UShort,
		SizeShort:       2,
		SizeInt:         4,
		SizeLong:        4,
		SizeLongLong:    8,
		SizePtr:         8,
		SizeFloat:       4,
		SizeDouble:      8,
		SizeLongDouble:  8,
		AlignLongDouble: 8,
		MSBitfields:     true,
		ABI:             Microsoft,
	}
}

// ILP32 returns the target model for 32-bit x86 ELF.
func ILP32() Model {
	return Model{
		CharSigned:      true,
		WCharKind:       Int,
		SizeShort:       2,
		SizeInt:         4,
		SizeLong:        4,
		SizeLongLong:    8,
		SizePtr:         4,
		SizeFloat:       4,
		SizeDouble:      8,
		SizeLongDouble:  12,
		AlignLongDouble: 4,
		MSBitfields:     false,
		ABI:             ItaniumGeneric,
	}
}

// ModelForTarget returns the model appropriate for target configuration.
func ModelForTarget(arch, os string) Model {
	if os == "windows" {
		return LLP64()
	}
	if arch == "i386" || arch == "386" {
		return ILP32()
	}
	m := LP64()
	if arch == "arm64" || arch == "aarch64" {
		m.ABI = ItaniumAArch64
		m.VaList = VaListAArch64
		switch os {
		case "linux":
			// AAPCS64 on Linux: plain char and wchar_t are both unsigned.
			m.CharSigned = false
			m.WCharKind = UInt
		case "macos", "darwin":
			// Apple's arm64: long double is double, and the ABI is
			// Apple's variant of ARM's.
			m.SizeLongDouble, m.AlignLongDouble = 8, 8
			m.ABI = ItaniumAppleARM64
			m.VaList = VaListPointer
		}
	}
	return m
}

// Sizeof returns the size of t in bytes, and whether the size is known.
func (m Model) Sizeof(t Type) (int64, bool) {
	if t == nil {
		return 0, false
	}
	switch u := Unqualify(t).(type) {
	case *Basic:
		return m.basicSize(u.K)
	case *Pointer:
		return m.SizePtr, true
	case *LValueReference, *RValueReference:
		return m.SizePtr, true
	case *MemberPointer:
		return m.memberPointerSize(u)
	case *Enum:
		if u.Underlying != nil {
			return m.Sizeof(u.Underlying)
		}
		return m.SizeInt, u.Complete
	case *Array:
		if u.Incomplete {
			return 0, false
		}
		elemSz, ok := m.Sizeof(u.Elem)
		if !ok {
			return 0, false
		}
		return elemSz * u.Len, true
	case *Record:
		if !u.Complete {
			return 0, false
		}
		size, _, ok := m.Layout(u, nil)
		return size, ok
	}
	return 0, false
}

// memberPointerSize returns the size of a member pointer for the target ABI.
// Under Itanium: data member pointer is ptrdiff_t, member function pointer is a pair.
// Under MSVC: size depends on inheritance model (single, multiple, virtual, or incomplete).
func (m Model) memberPointerSize(mp *MemberPointer) (int64, bool) {
	isFunc := IsFunc(mp.Elem)

	if m.ABI.IsItanium() {
		if isFunc {
			return 2 * m.SizePtr, true
		}
		return m.SizePtr, true
	}

	rec, _ := Unqualify(mp.Class).(*Record)
	switch {
	case rec == nil || !rec.Complete:
		if isFunc {
			return 2 * m.SizePtr, true
		}
		return 8, true
	case hasVirtualBase(rec, make(map[*Record]bool)):
		if isFunc {
			return 2 * m.SizePtr, true
		}
		return 8, true
	case len(rec.Bases) > 1:
		if isFunc {
			return 2 * m.SizePtr, true
		}
		return 4, true
	default:
		if isFunc {
			return m.SizePtr, true
		}
		return 4, true
	}
}

// hasVirtualBase reports whether a class has a virtual base anywhere in its hierarchy.
func hasVirtualBase(r *Record, visited map[*Record]bool) bool {
	if r == nil || visited[r] {
		return false
	}
	visited[r] = true
	for _, b := range r.Bases {
		if b.Virtual {
			return true
		}
		if br, ok := Unqualify(b.Type).(*Record); ok && hasVirtualBase(br, visited) {
			return true
		}
	}
	return false
}

// baseOrder is the order the bases are laid out in, as indices into the
// declaration order.
//
// A polymorphic base comes first, so that the virtual table pointer it
// carries lands at offset zero and a pointer to the derived class is already
// a pointer to it -- no adjustment on the common path. Both ABIs do this,
// under different names: Itanium calls it choosing a primary base, and
// Microsoft does not name it at all.
//
// It is measured rather than assumed: cl lays `struct D : Plain, PolyA {
// int z; }` out with PolyA at 0 and Plain at 16, which is not the order they
// were written in.
func baseOrder(r *Record) []int {
	order := make([]int, 0, len(r.Bases))
	for i, b := range r.Bases {
		if br, ok := Unqualify(b.Type).(*Record); ok && !b.Virtual &&
			isPolymorphic(br, make(map[*Record]bool)) {
			order = append(order, i)
		}
	}
	if len(order) == len(r.Bases) {
		return order // already in declaration order
	}
	for i := range r.Bases {
		if !containsInt(order, i) {
			order = append(order, i)
		}
	}
	return order
}

func containsInt(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// isPolymorphic reports whether a class has a virtual table pointer, its own
// or one inherited from a base.
func isPolymorphic(r *Record, visited map[*Record]bool) bool {
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
		if br, ok := Unqualify(b.Type).(*Record); ok && isPolymorphic(br, visited) {
			return true
		}
	}
	return false
}

// IsEmptyRecord is isEmptyRecord for callers outside the package: the
// trait is_empty asks it.
func IsEmptyRecord(r *Record) bool { return isEmptyRecord(r, make(map[*Record]bool)) }

// IsPolymorphic is isPolymorphic for callers outside the package.
func IsPolymorphic(r *Record) bool { return isPolymorphic(r, make(map[*Record]bool)) }

// IsAbstract reports whether a class has a pure virtual function with
// no final overrider: one of its own, or an inherited one not overridden.
func IsAbstract(r *Record) bool {
	if r == nil {
		return false
	}
	for _, table := range LLP64().VTables(r) {
		for _, slot := range table.Slots {
			if slot.Pure {
				return true
			}
		}
	}
	return false
}

// isEmptyRecord reports whether a class occupies no storage of its own:
// no members, no virtual functions, and recursively all bases are empty.
func isEmptyRecord(r *Record, visited map[*Record]bool) bool {
	if r == nil || visited[r] {
		return false
	}
	visited[r] = true

	if len(r.Fields) > 0 {
		return false
	}
	for _, meth := range r.Methods {
		if meth.Virtual {
			return false
		}
	}
	for _, b := range r.Bases {
		br, ok := Unqualify(b.Type).(*Record)
		if !ok || !isEmptyRecord(br, visited) {
			return false
		}
	}
	return true
}

func (m Model) basicSize(k Kind) (int64, bool) {
	switch k {
	case Void:
		return 0, false
	case Bool, Char, SChar, UChar, Char8:
		return 1, true
	case Char16, Short, UShort:
		return m.SizeShort, true
	case Char32:
		return 4, true
	case WChar:
		return m.basicSize(m.WCharKind)
	case Int, UInt:
		return m.SizeInt, true
	case Long, ULong:
		return m.SizeLong, true
	case LongLong, ULongLong:
		return m.SizeLongLong, true
	case Int128, UInt128:
		return 16, true
	case Float:
		return m.SizeFloat, true
	case Double:
		return m.SizeDouble, true
	case LongDouble:
		return m.SizeLongDouble, true
	case NullptrKind:
		return m.SizePtr, true
	}
	return 0, false
}

// Alignof returns the alignment of t in bytes, and whether it is known.
func (m Model) Alignof(t Type) (int64, bool) {
	if t == nil {
		return 0, false
	}
	switch u := Unqualify(t).(type) {
	case *Basic:
		if u.K == LongDouble {
			return m.AlignLongDouble, true
		}
		return m.basicSize(u.K)
	case *Pointer, *LValueReference, *RValueReference:
		return m.SizePtr, true
	case *MemberPointer:
		// A member pointer aligns to whatever it is made of, not to a
		// pointer: Microsoft's four-byte data member pointer aligns to four,
		// and putting it at eight would leave a hole cl does not leave.
		sz, ok := m.memberPointerSize(u)
		if !ok {
			return m.SizePtr, true
		}
		if sz > m.SizePtr {
			return m.SizePtr, true
		}
		return sz, true
	case *Enum:
		if u.Underlying != nil {
			return m.Alignof(u.Underlying)
		}
		return m.SizeInt, u.Complete
	case *Array:
		return m.Alignof(u.Elem)
	case *Record:
		if !u.Complete {
			return 0, false
		}
		_, align, ok := m.Layout(u, nil)
		return align, ok
	}
	return 0, false
}

// Offsetof returns the byte offset of a named member in a complete record.
func (m Model) Offsetof(t Type, name string) (int64, bool) {
	r, ok := Unqualify(t).(*Record)
	if !ok || !r.Complete {
		return 0, false
	}
	offs := make([]int64, len(r.Fields))
	if _, _, ok := m.Layout(r, offs); !ok {
		return 0, false
	}
	for i, f := range r.Fields {
		if f.Name == name {
			if f.BitField {
				return 0, false
			}
			return offs[i], true
		}
		if f.Name == "" {
			if inner, isRec := Unqualify(f.Type).(*Record); isRec {
				if off, ok := m.Offsetof(inner, name); ok {
					return offs[i] + off, true
				}
			}
		}
	}
	return 0, false
}

func roundUp(n, to int64) int64 {
	if to <= 0 {
		return n
	}
	return (n + to - 1) / to * to
}

// VirtualBaseOffsets is where each virtual base of a class sits in the
// complete object. A class with none yields an empty map.
func (m Model) VirtualBaseOffsets(r *Record) map[*Record]int64 {
	info, ok := m.layoutInfo(r)
	if !ok {
		return nil
	}
	return info.vbaseOffsets
}

// VBPtrOffset is where a class's own virtual base table pointer sits, or -1
// if it has none of its own.
func (m Model) VBPtrOffset(r *Record) int64 {
	info, ok := m.layoutInfo(r)
	if !ok {
		return -1
	}
	return info.vbptr
}

// layoutNonVirtual is a class's size and alignment counting only what it
// carries directly -- its own members, its non-virtual bases, and its
// pointers. A virtual base placed inside a derived class occupies exactly
// this much, because the shared subobject it in turn derives from is placed
// by the *complete* object rather than by it.
func (m Model) layoutNonVirtual(r *Record) (size, align int64, ok bool) {
	info, ok := m.layoutInfo(r)
	if !ok {
		return 0, 0, false
	}
	if m.ABI.IsItanium() {
		return info.nvsize, info.nvalign, true
	}
	if len(info.vbaseOffsets) == 0 {
		return info.nvsize, info.align, true
	}
	// Trim the virtual bases back off: the first of them marks where this
	// class's own storage ended.
	first := info.size
	for _, off := range info.vbaseOffsets {
		if off < first {
			first = off
		}
	}
	return first, info.align, true
}

// Layout computes a record's size and alignment, optionally populating field offsets.
func (m Model) Layout(r *Record, fieldOffs []int64) (size, align int64, ok bool) {
	return m.LayoutWithBases(r, fieldOffs, nil)
}

// BitField returns a bit-field member's layout: the byte offset of its storage
// unit, the unit's width in bytes, and the bit offset within the unit.
func (m Model) BitField(r *Record, i int) (unitOff, unitBytes, bitOff int64, ok bool) {
	info, ok := m.layoutInfo(r)
	if !ok || i < 0 || i >= len(info.fieldOffsets) || !r.Fields[i].BitField {
		return 0, 0, 0, false
	}
	return info.fieldOffsets[i], info.bitUnits[i], info.bitOffsets[i], true
}

// LayoutWithBases computes a class's size, alignment, field offsets, and base offsets.
func (m Model) LayoutWithBases(r *Record, fieldOffs, baseOffs []int64) (size, align int64, ok bool) {
	info, ok := m.layoutInfo(r)
	if !ok {
		return 0, 0, false
	}
	copy(fieldOffs, info.fieldOffsets)
	copy(baseOffs, info.baseOffsets)
	return info.size, info.align, true
}

// recordInfo holds computed layout metrics and offsets for a record.
type recordInfo struct {
	size, align  int64
	fieldOffsets []int64

	// bitOffsets and bitUnits are, per field, a bit-field's position in
	// its storage unit and the unit's width in bytes; zero for a field
	// that is not one.
	bitOffsets []int64
	bitUnits   []int64

	// nvsize is the non-virtual part's size as a base subobject: rounded
	// to the alignment the members gave the class, not to an alignas on
	// the class-head, whose padding belongs to the complete object only
	// -- cl lays a derived class's members into it.
	nvsize int64

	// dsize and nvalign are Itanium's: where the class's meaningful bytes
	// end, which is where a derived class may start putting its own, and
	// the alignment of the class as a base. primary is the index of the
	// primary base in r.Bases, or -1; primaryVirtual is set when that base
	// is a virtual one, which may be an indirect virtual base with no index.
	dsize          int64
	nvalign        int64
	primary        int
	primaryVirtual *Record

	baseOffsets  []int64
	vbptr        int64 // where this class's own vbptr sits, or -1
	vbaseOffsets map[*Record]int64
}

func (m Model) layoutInfo(r *Record) (recordInfo, bool) {
	if r != nil && m.ABI.IsItanium() {
		return m.layoutItanium(r)
	}
	info := recordInfo{vbptr: -1, vbaseOffsets: map[*Record]int64{}, primary: -1}
	if r == nil {
		return info, false
	}
	info.fieldOffsets = make([]int64, len(r.Fields))
	info.baseOffsets = make([]int64, len(r.Bases))
	info.bitOffsets = make([]int64, len(r.Fields))
	info.bitUnits = make([]int64, len(r.Fields))
	fieldOffs, baseOffs := info.fieldOffsets, info.baseOffsets

	var align int64
	var size int64
	align = 1

	// Check if this class introduces a new vfptr (not inherited from a polymorphic base).
	hasVPtr := false
	for _, meth := range r.Methods {
		if meth.Virtual {
			hasVPtr = true
			break
		}
	}
	if hasVPtr {
		for _, b := range r.Bases {
			if br, ok := Unqualify(b.Type).(*Record); ok && isPolymorphic(br, make(map[*Record]bool)) {
				hasVPtr = false
				break
			}
		}
	}

	var curOffset int64
	if hasVPtr {
		curOffset = m.SizePtr
		if m.SizePtr > align {
			align = m.SizePtr
		}
	}

	// Lay out non-virtual base classes. First empty base is at offset 0; subsequent empty bases take 1 byte.
	emptySoFar := 0
	for _, i := range baseOrder(r) {
		b := r.Bases[i]
		bRec, isRec := Unqualify(b.Type).(*Record)
		if !isRec || !bRec.Complete {
			continue
		}
		// A base subobject contributes only what it carries directly. Its
		// own virtual bases are not inside it here: they were hoisted to
		// the end of *this* object, which is the whole point of sharing
		// them. Using the base's complete size would count them twice.
		bSize, bAlign, bOk := m.layoutNonVirtual(bRec)
		if !bOk {
			return info, false
		}
		if bAlign > align {
			align = bAlign
		}
		if b.Virtual {
			// A virtual base is not laid out here. It goes after everything
			// else, once, however many paths reach it -- see below.
			continue
		}
		if isEmptyRecord(bRec, make(map[*Record]bool)) && !hasVPtr {
			off := curOffset
			if emptySoFar > 0 {
				// A second empty base cannot share the first's address.
				off = curOffset + int64(emptySoFar)
				if off+1 > curOffset {
					curOffset = off + 1
				}
			}
			emptySoFar++
			if baseOffs != nil && i < len(baseOffs) {
				baseOffs[i] = off
			}
			continue
		}
		curOffset = roundUp(curOffset, bAlign)
		if baseOffs != nil && i < len(baseOffs) {
			baseOffs[i] = curOffset
		}
		curOffset += bSize
	}

	// Virtual base table pointer (vbptr) placement.
	if hasOwnVBPtr(r) {
		curOffset = roundUp(curOffset, m.SizePtr)
		info.vbptr = curOffset
		curOffset += m.SizePtr
		if m.SizePtr > align {
			align = m.SizePtr
		}
	}

	if r.Tag == TagUnion {
		// Union layout: all members at offset 0
		var maxFieldSize int64
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
			if fa > align {
				align = fa
			}
			if fieldOffs != nil && i < len(fieldOffs) {
				fieldOffs[i] = 0
			}
			if fs > maxFieldSize {
				maxFieldSize = fs
			}
		}
		if r.Align > align {
			align = r.Align
		}
		size = roundUp(maxFieldSize, align)
		if size == 0 {
			size = 1 // C++ objects have non-zero size
		}
		info.size, info.align = size, align
		return info, true
	}

	// Struct / Class bitfield layout.
	// Under MSBitfields, unit packing does not span different declared types, and zero-width closes the unit.
	var (
		unitOffset int64 = -1 // byte offset of the open storage unit
		unitSize   int64      // size of that unit's declared type
		unitUsed   int64      // bits spent in it
	)
	closeUnit := func() {
		if unitOffset >= 0 {
			curOffset = unitOffset + unitSize
			unitOffset = -1
		}
	}

	for i, f := range r.Fields {
		fs, ok1 := m.Sizeof(f.Type)
		fa, ok2 := m.Alignof(f.Type)
		if !ok1 || !ok2 {
			return info, false
		}
		if f.Align > fa {
			// alignas raises the member's alignment.
			fa = f.Align
		}
		fa = r.MemberAlign(fa)
		if fa > align && (!f.BitField || f.Name != "") {
			align = fa
		}

		if f.BitField {
			if f.Width == 0 {
				closeUnit()
				curOffset = roundUp(curOffset, fa)
				if fieldOffs != nil && i < len(fieldOffs) {
					fieldOffs[i] = curOffset
				}
				continue
			}

			needNew := unitOffset < 0 ||
				unitUsed+f.Width > unitSize*8 ||
				(m.MSBitfields && unitSize != fs)
			if needNew {
				closeUnit()
				curOffset = roundUp(curOffset, fa)
				unitOffset, unitSize, unitUsed = curOffset, fs, 0
			}
			if fieldOffs != nil && i < len(fieldOffs) {
				fieldOffs[i] = unitOffset
			}
			info.bitOffsets[i], info.bitUnits[i] = unitUsed, unitSize
			unitUsed += f.Width
			continue
		}

		closeUnit()
		curOffset = roundUp(curOffset, fa)
		if fieldOffs != nil && i < len(fieldOffs) {
			fieldOffs[i] = curOffset
		}
		curOffset += fs
	}
	closeUnit()

	info.nvsize = roundUp(curOffset, align)
	if info.nvsize == 0 {
		info.nvsize = 1
	}
	if r.Align > align {
		align = r.Align
	}

	// Lay out virtual bases after non-virtual part, aligned to class alignment.
	if vbases := VirtualBases(r); len(vbases) > 0 {
		curOffset = roundUp(curOffset, align)
		for _, vb := range vbases {
			vbSize, vbAlign, vbOK := m.layoutNonVirtual(vb)
			if !vbOK {
				continue
			}
			// Empty virtual base takes 0 bytes.
			if isEmptyRecord(vb, make(map[*Record]bool)) {
				vbSize = 0
			}
			if vbAlign > align {
				align = vbAlign
			}
			curOffset = roundUp(curOffset, vbAlign)
			info.vbaseOffsets[vb] = curOffset
			curOffset += vbSize

			// Report it where a declared virtual base was written, so a
			// caller reading base offsets sees one number per base.
			for i, b := range r.Bases {
				if !b.Virtual {
					continue
				}
				if br, isRec := Unqualify(b.Type).(*Record); isRec && br == vb {
					baseOffs[i] = info.vbaseOffsets[vb]
				}
			}
		}
	}

	size = roundUp(curOffset, align)
	if size == 0 {
		size = 1 // Empty struct/class has size 1 in C++
	}

	info.size, info.align = size, align
	return info, true
}

// BuiltinVaList is __builtin_va_list as a type of this model. Each call
// builds it afresh; a caller that needs one identity, as an analysis does,
// asks once.
func (m Model) BuiltinVaList() Type {
	voidPtr := &Pointer{Elem: Typ(Void)}
	switch m.VaList {
	case VaListX86_64:
		tag := &Record{Tag: TagStruct, Name: "__va_list_tag", Complete: true, Fields: []Field{
			{Name: "gp_offset", Type: Typ(UInt)},
			{Name: "fp_offset", Type: Typ(UInt)},
			{Name: "overflow_arg_area", Type: voidPtr},
			{Name: "reg_save_area", Type: voidPtr},
		}}
		return &Array{Elem: tag, Len: 1}
	case VaListAArch64:
		return &Record{Tag: TagStruct, Name: "__va_list", Scopes: []string{"std"}, Complete: true, Fields: []Field{
			{Name: "__stack", Type: voidPtr},
			{Name: "__gr_top", Type: voidPtr},
			{Name: "__vr_top", Type: voidPtr},
			{Name: "__gr_offs", Type: Typ(Int)},
			{Name: "__vr_offs", Type: Typ(Int)},
		}}
	}
	return &Pointer{Elem: Typ(Char)}
}
