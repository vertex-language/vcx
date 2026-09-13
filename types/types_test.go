package types

import (
	"testing"
)

func TestBasicSizes(t *testing.T) {
	lp64 := LP64()
	llp64 := LLP64()
	ilp32 := ILP32()

	tests := []struct {
		typ   Type
		lp64  int64
		llp64 int64
		ilp32 int64
	}{
		{Typ(Bool), 1, 1, 1},
		{Typ(Char), 1, 1, 1},
		{Typ(Short), 2, 2, 2},
		{Typ(Int), 4, 4, 4},
		{Typ(Long), 8, 4, 4},
		{Typ(LongLong), 8, 8, 8},
		{Typ(Float), 4, 4, 4},
		{Typ(Double), 8, 8, 8},
		{Typ(LongDouble), 16, 8, 12},
		{&Pointer{Elem: Typ(Int)}, 8, 8, 4},
		{&LValueReference{Elem: Typ(Int)}, 8, 8, 4},
	}

	for _, tt := range tests {
		sz1, ok1 := lp64.Sizeof(tt.typ)
		if !ok1 || sz1 != tt.lp64 {
			t.Errorf("LP64 Sizeof(%s) = %d (ok=%v), want %d", tt.typ, sz1, ok1, tt.lp64)
		}
		sz2, ok2 := llp64.Sizeof(tt.typ)
		if !ok2 || sz2 != tt.llp64 {
			t.Errorf("LLP64 Sizeof(%s) = %d (ok=%v), want %d", tt.typ, sz2, ok2, tt.llp64)
		}
		sz3, ok3 := ilp32.Sizeof(tt.typ)
		if !ok3 || sz3 != tt.ilp32 {
			t.Errorf("ILP32 Sizeof(%s) = %d (ok=%v), want %d", tt.typ, sz3, ok3, tt.ilp32)
		}
	}
}

func TestRecordLayout(t *testing.T) {
	m := LP64()

	// 1. Empty struct
	empty := &Record{Tag: TagStruct, Complete: true}
	sz, al, ok := m.Layout(empty, nil)
	if !ok || sz != 1 || al != 1 {
		t.Fatalf("Empty struct layout = (%d, %d), want (1, 1)", sz, al)
	}

	// 2. Struct with fields: char c; int i; double d;
	// char at 0, 3 bytes pad, int at 4, double at 8 -> size 16, align 8
	fields := []Field{
		{Name: "c", Type: Typ(Char)},
		{Name: "i", Type: Typ(Int)},
		{Name: "d", Type: Typ(Double)},
	}
	s1 := &Record{Tag: TagStruct, Name: "S1", Fields: fields, Complete: true}
	offs := make([]int64, len(fields))
	sz, al, ok = m.Layout(s1, offs)
	if !ok || sz != 16 || al != 8 {
		t.Fatalf("S1 layout = (%d, %d), want (16, 8)", sz, al)
	}
	if offs[0] != 0 || offs[1] != 4 || offs[2] != 8 {
		t.Fatalf("S1 field offsets = %v, want [0, 4, 8]", offs)
	}

	// Offsetof query
	off, found := m.Offsetof(s1, "i")
	if !found || off != 4 {
		t.Errorf("Offsetof(S1, i) = %d (found=%v), want 4", off, found)
	}

	// 3. Inheritance with Empty Base Optimization
	derivedWithEmptyBase := &Record{
		Tag:      TagClass,
		Name:     "Derived1",
		Bases:    []BaseSpec{{Type: empty, Access: AccessPublic}},
		Fields:   []Field{{Name: "x", Type: Typ(Int)}},
		Complete: true,
	}
	sz, al, ok = m.Layout(derivedWithEmptyBase, nil)
	if !ok || sz != 4 || al != 4 {
		t.Errorf("Derived1 layout = (%d, %d), want (4, 4) with EBO", sz, al)
	}

	// 4. Polymorphic class has vptr
	poly := &Record{
		Tag:  TagClass,
		Name: "Poly",
		Methods: []*Method{
			{Name: "vfunc", Virtual: true, Func: &Func{Ret: Typ(Void)}},
		},
		Fields:   []Field{{Name: "a", Type: Typ(Int)}},
		Complete: true,
	}
	sz, al, ok = m.Layout(poly, nil)
	// vptr (8 bytes) at 0, a (4 bytes) at 8 -> size 16, align 8
	if !ok || sz != 16 || al != 8 {
		t.Errorf("Poly layout = (%d, %d), want (16, 8)", sz, al)
	}
}

func TestQualifiersAndCollapsing(t *testing.T) {
	intTyp := Typ(Int)
	constInt := Qualify(intTyp, QConst)

	if !IsConst(constInt) {
		t.Errorf("const int must be IsConst")
	}
	if !Unqualify(constInt).Equal(intTyp) {
		t.Errorf("Unqualify(const int) must equal int")
	}

	// Reference collapsing
	// & + & -> &
	ref1 := AddLValueReference(intTyp) // int&
	ref2 := AddLValueReference(ref1)   // int& & -> int&
	if !IsLValueReference(ref2) {
		t.Errorf("AddLValueReference(int&) must be lvalue ref")
	}

	// & + && -> &
	refR := AddRValueReference(intTyp) // int&&
	refLfromR := AddLValueReference(refR)
	if !IsLValueReference(refLfromR) {
		t.Errorf("AddLValueReference(int&&) must collapse to lvalue ref")
	}

	// && + && -> &&
	refRfromR := AddRValueReference(refR)
	if !IsRValueReference(refRfromR) {
		t.Errorf("AddRValueReference(int&&) must collapse to rvalue ref")
	}

	// Decay
	arr := &Array{Elem: Typ(Char), Len: 10}
	decayed := Decay(arr)
	if !decayed.Equal(&Pointer{Elem: Typ(Char)}) {
		t.Errorf("Decay(char[10]) = %s, want char*", decayed)
	}
}

func TestSubtypingAndConversions(t *testing.T) {
	base := &Record{Tag: TagClass, Name: "Base", Complete: true}
	derived := &Record{
		Tag:      TagClass,
		Name:     "Derived",
		Bases:    []BaseSpec{{Type: base, Access: AccessPublic}},
		Complete: true,
	}

	if !IsBaseOf(base, derived) {
		t.Errorf("IsBaseOf(Base, Derived) should be true")
	}
	if IsBaseOf(derived, base) {
		t.Errorf("IsBaseOf(Derived, Base) should be false")
	}

	// Pointer conversion: Derived* -> Base*
	derivedPtr := &Pointer{Elem: derived}
	basePtr := &Pointer{Elem: base}
	if !IsConvertible(derivedPtr, basePtr) {
		t.Errorf("Derived* should be convertible to Base*")
	}

	// Const qualification conversion: int* -> const int*
	intPtr := &Pointer{Elem: Typ(Int)}
	constIntPtr := &Pointer{Elem: Qualify(Typ(Int), QConst)}
	if !IsConvertible(intPtr, constIntPtr) {
		t.Errorf("int* should be convertible to const int*")
	}
	if IsConvertible(constIntPtr, intPtr) {
		t.Errorf("const int* should NOT be convertible to int*")
	}

	// Nullptr conversion
	nullptrTyp := Typ(NullptrKind)
	if !IsConvertible(nullptrTyp, intPtr) {
		t.Errorf("nullptr should be convertible to int*")
	}
	if !IsConvertible(nullptrTyp, Typ(Bool)) {
		t.Errorf("nullptr should be convertible to bool")
	}

	// CommonType arithmetic conversions
	ct1 := CommonType(Typ(Int), Typ(Double))
	if !ct1.Equal(Typ(Double)) {
		t.Errorf("CommonType(int, double) = %s, want double", ct1)
	}
	ct2 := CommonType(Typ(Short), Typ(Char))
	if !ct2.Equal(Typ(Int)) {
		t.Errorf("CommonType(short, char) = %s, want int", ct2)
	}
	ct3 := CommonType(Typ(Int), Typ(UInt))
	if !ct3.Equal(Typ(UInt)) {
		t.Errorf("CommonType(int, unsigned int) = %s, want unsigned int", ct3)
	}
}
