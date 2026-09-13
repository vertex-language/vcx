package types

import "testing"

func TestApplyTransform(t *testing.T) {
	i := Typ(Int)
	ci := Qualify(i, QConst)
	arr := &Array{Elem: ci, Len: 3}
	fn := &Func{Ret: i}
	enum := &Enum{Name: "E", Underlying: Typ(UShort), Complete: true}
	for _, tc := range []struct {
		op       string
		in, want Type
	}{
		{"__remove_reference_t", AddLValueReference(ci), ci},
		{"__remove_cvref", AddRValueReference(ci), i},
		{"__remove_const", Qualify(i, QConst|QVolatile), Qualify(i, QVolatile)},
		{"__decay", AddLValueReference(arr), &Pointer{Elem: ci}},
		{"__decay", fn, &Pointer{Elem: fn}},
		{"__add_pointer", AddLValueReference(i), &Pointer{Elem: i}},
		{"__add_lvalue_reference", Typ(Void), Typ(Void)},
		{"__remove_all_extents", &Array{Elem: arr, Len: 2}, ci},
		{"__underlying_type", enum, Typ(UShort)},
		{"__make_unsigned", ci, Qualify(Typ(UInt), QConst)},
		{"__make_signed", enum, Typ(Short)},
		{"__make_signed", Typ(Char32), Typ(Int)},
	} {
		got, ok := ApplyTransform(tc.op, tc.in, false)
		if !ok || got == nil || !got.Equal(tc.want) {
			t.Errorf("%s(%s) = %v, want %s", tc.op, tc.in, got, tc.want)
		}
	}
	if got, _ := ApplyTransform("__decay", &TemplateParam{Name: "T"}, true); got.Kind() != TransformKind {
		t.Errorf("a dependent operand gave %v, want a Transform", got)
	}
}
