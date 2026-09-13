package mangle

import (
	"testing"

	"github.com/vertex-language/vcx/types"
)

var (
	tInt    = types.Typ(types.Int)
	tVoid   = types.Typ(types.Void)
	tChar   = types.Typ(types.Char)
	tBool   = types.Typ(types.Bool)
	tLL     = types.Typ(types.LongLong)
	tDouble = types.Typ(types.Double)
)

func fn(ret types.Type, params ...types.Type) *types.Func {
	f := &types.Func{Ret: ret}
	for _, p := range params {
		f.Params = append(f.Params, types.Param{Type: p})
	}
	return f
}

func constOf(t types.Type) types.Type { return &types.Qualified{Q: types.QConst, T: t} }
func ptr(t types.Type) types.Type     { return &types.Pointer{Elem: t} }
func ref(t types.Type) types.Type     { return &types.LValueReference{Elem: t} }

var (
	widget = &types.Record{Tag: types.TagStruct, Name: "Widget"}
	nsRec  = &types.Record{Tag: types.TagClass, Name: "Widget", Scopes: []string{"N"}}
)

// The Itanium cases are examples from the ABI document and from what g++
// and clang++ are known to write; there is no oracle for them here.
func TestItanium(t *testing.T) {
	cases := []struct {
		f    Function
		want string
	}{
		{Function{Name: "f", Type: fn(tVoid)}, "_Z1fv"},
		{Function{Name: "f", Type: fn(tVoid, tInt)}, "_Z1fi"},
		{Function{Name: "main", Type: fn(tInt)}, "main"},
		{Function{Name: "f", Type: fn(tVoid, tInt), ExternC: true}, "f"},
		{Function{Scopes: Scopes("N"), Name: "f", Type: fn(tVoid)}, "_ZN1N1fEv"},
		{Function{Scopes: Scopes("std"), Name: "f", Type: fn(tVoid)}, "_ZSt1fv"},
		{Function{Scopes: Scopes("Widget"), Name: "foo", Type: fn(tVoid, tInt), Member: true}, "_ZN6Widget3fooEi"},
		{Function{Scopes: Scopes("Widget"), Name: "foo", Type: &types.Func{Ret: tVoid, Quals: types.QConst}, Member: true}, "_ZNK6Widget3fooEv"},
		{Function{Scopes: Scopes("Widget"), Name: "Widget", Type: fn(tVoid), Kind: Ctor, Member: true}, "_ZN6WidgetC1Ev"},
		{Function{Scopes: Scopes("Widget"), Name: "~Widget", Type: fn(tVoid), Kind: Dtor, Member: true}, "_ZN6WidgetD1Ev"},
		{Function{Name: "operator+", Type: fn(tInt, tInt, tInt)}, "_Zplii"},
		{Function{Scopes: Scopes("Widget"), Name: "operator()", Type: fn(tInt), Member: true}, "_ZN6WidgetclEv"},
		// Two identical class arguments: the second is the first substitution.
		{Function{Name: "f", Type: fn(tVoid, widget, widget)}, "_Z1f6WidgetS_"},
		// `const int &` twice: `Ki` is S_, `RKi` is S0_, and the second
		// parameter is the latter.
		{Function{Name: "f", Type: fn(tVoid, ref(constOf(tInt)), ref(constOf(tInt)))}, "_Z1fRKiS0_"},
		// A nested class: the namespace prefix is a candidate, the class is
		// the next, and the second parameter refers to the class.
		{Function{Name: "f", Type: fn(tVoid, nsRec, nsRec)}, "_Z1fN1N6WidgetES0_"},
		// A function pointer: the function type is a candidate and so is
		// the pointer to it.
		{Function{Name: "f", Type: fn(tVoid, ptr(fn(tVoid, tInt)), ptr(fn(tVoid, tInt)))}, "_Z1fPFviES0_"},
		{Function{Name: "f", Type: fn(tVoid, ptr(constOf(tChar)))}, "_Z1fPKc"},
		{Function{Name: "f", Type: &types.Func{Ret: tVoid, Params: []types.Param{{Type: tInt}}, Variadic: true}}, "_Z1fiz"},
		// An array parameter is a pointer, and top-level
		// const on a parameter is not part of the type.
		{Function{Name: "f", Type: fn(tVoid, &types.Array{Elem: tInt, Len: 3})}, "_Z1fPi"},
		{Function{Name: "f", Type: fn(tVoid, constOf(tInt))}, "_Z1fi"},
	}
	for _, c := range cases {
		got, err := FunctionName(Itanium, &c.f)
		if err != nil {
			t.Errorf("%s: %v", c.want, err)
			continue
		}
		if got != c.want {
			t.Errorf("got %s, want %s", got, c.want)
		}
	}

	vars := []struct {
		v    Variable
		want string
	}{
		{Variable{Name: "g", Type: tInt}, "g"},
		{Variable{Scopes: Scopes("N"), Name: "g", Type: tInt}, "_ZN1N1gE"},
		{Variable{Scopes: Scopes("Widget"), Name: "count", Type: tInt, Static: true}, "_ZN6Widget5countE"},
	}
	for _, c := range vars {
		got, err := VariableName(Itanium, &c.v)
		if err != nil {
			t.Errorf("%s: %v", c.want, err)
			continue
		}
		if got != c.want {
			t.Errorf("got %s, want %s", got, c.want)
		}
	}

	if got, _ := VTableName(Itanium, widget, nil); got != "_ZTV6Widget" {
		t.Errorf("vtable: got %s, want _ZTV6Widget", got)
	}
	if got, _ := VTableName(Itanium, nsRec, nil); got != "_ZTVN1N6WidgetE" {
		t.Errorf("vtable: got %s, want _ZTVN1N6WidgetE", got)
	}
}

// The Microsoft cases here are the handful measured before tests/mangle
// existed; the corpus is where the scheme is actually checked.
func TestMicrosoft(t *testing.T) {
	cases := []struct {
		f    Function
		want string
	}{
		{Function{Name: "f", Type: fn(tVoid)}, "?f@@YAXXZ"},
		{Function{Name: "f", Type: fn(tVoid, tInt)}, "?f@@YAXH@Z"},
		{Function{Name: "main", Type: fn(tInt)}, "main"},
		{Function{Name: "v", Type: &types.Func{Ret: tVoid, Params: []types.Param{{Type: tInt}}, Variadic: true}}, "?v@@YAXHZZ"},
		{Function{Scopes: Scopes("Widget"), Name: "foo", Type: fn(tVoid, tInt), Member: true}, "?foo@Widget@@QEAAXH@Z"},
		{Function{Scopes: Scopes("Widget"), Name: "Widget", Type: fn(tVoid), Kind: Ctor, Member: true}, "??0Widget@@QEAA@XZ"},
		{Function{Scopes: Scopes("Widget"), Name: "~Widget", Type: fn(tVoid), Kind: Dtor, Member: true}, "??1Widget@@QEAA@XZ"},
		// The copy constructor: the class is back-reference 0 by the time
		// its parameter is written.
		{Function{Scopes: Scopes("Widget"), Name: "Widget", Type: fn(tVoid, ref(constOf(widget))), Kind: Ctor, Member: true}, "??0Widget@@QEAA@AEBU0@@Z"},
		{Function{Name: "f", Type: fn(tVoid, tBool, tBool)}, "?f@@YAX_N0@Z"},
		{Function{Name: "f", Type: fn(tVoid, tLL, tDouble, tLL)}, "?f@@YAX_JN0@Z"},
		{Function{Name: "f", Type: fn(widget)}, "?f@@YA?AUWidget@@XZ"},
		{Function{Name: "f", Type: fn(tVoid, ptr(fn(tVoid, tInt)))}, "?f@@YAXP6AXH@Z@Z"},
	}
	for _, c := range cases {
		got, err := FunctionName(Microsoft, &c.f)
		if err != nil {
			t.Errorf("%s: %v", c.want, err)
			continue
		}
		if got != c.want {
			t.Errorf("got %s, want %s", got, c.want)
		}
	}
	if got, _ := VTableName(Microsoft, widget, nil); got != "??_7Widget@@6B@" {
		t.Errorf("vftable: got %s, want ??_7Widget@@6B@", got)
	}
}

// The structor variants and a thunk, against what clang++ writes for
//
//	struct A { virtual void f(); };
//	struct B { virtual void g(); };
//	struct C : A, B { C(); ~C(); void g(); };
//
// with the members defined, on the machine this was written on: C2 and D2
// are what a derived class's structors call, and the thunk in B-in-C's
// table subtracts eight. D0, a virtual destructor's second slot, is the
// same spelling with the last digit changed.
func TestItaniumStructorVariants(t *testing.T) {
	c := &types.Record{Name: "C", Complete: true}
	void := &types.Func{Ret: types.Typ(types.Void)}
	cases := []struct {
		kind Kind
		name string
		want string
	}{
		{Ctor, "C", "_ZN1CC1Ev"},
		{CtorBase, "C", "_ZN1CC2Ev"},
		{Dtor, "~C", "_ZN1CD1Ev"},
		{DtorBase, "~C", "_ZN1CD2Ev"},
		{DtorDeleting, "~C", "_ZN1CD0Ev"},
	}
	for _, tc := range cases {
		got, err := FunctionName(Itanium, &Function{Scopes: recordScopes(c), Name: tc.name, Type: void, Kind: tc.kind, Member: true})
		if err != nil || got != tc.want {
			t.Errorf("kind %d: got %q, %v; want %q", tc.kind, got, err, tc.want)
		}
	}
	got, err := ThunkName(Itanium, &Function{Scopes: recordScopes(c), Name: "g", Type: void, Member: true}, 8)
	if want := "_ZThn8_N1C1gEv"; err != nil || got != want {
		t.Errorf("thunk: got %q, %v; want %q", got, err, want)
	}
	if _, err := FunctionName(Microsoft, &Function{Scopes: recordScopes(c), Name: "~C", Type: void, Kind: DtorDeleting, Member: true}); err == nil {
		t.Error("the Microsoft scheme spelled a D0")
	}
}

// A class directly in std is St and its name, not a nested name: the
// aligned allocation functions are the case that found it, since libc++
// exports `__ZnamSt11align_val_t` and nothing else.
func TestItaniumStdClass(t *testing.T) {
	alignVal := &types.Enum{Name: "align_val_t", Scopes: []string{"std"}, Scoped: true, Underlying: types.Typ(types.ULong), Complete: true}
	got, err := FunctionName(Itanium, &Function{
		Name: "operator new[]",
		Type: &types.Func{Ret: &types.Pointer{Elem: tVoid}, Params: []types.Param{{Type: types.Typ(types.ULong)}, {Type: alignVal}}},
	})
	if want := "_ZnamSt11align_val_t"; err != nil || got != want {
		t.Errorf("got %q, %v; want %q", got, err, want)
	}
}

func TestItaniumTypeInfoNames(t *testing.T) {
	base := &types.Record{Name: "Base", Complete: true}
	inner := &types.Record{Name: "Inner", Scopes: []string{"ns"}, Complete: true}
	for _, tc := range []struct {
		rec        *types.Record
		ti, ts, nm string
	}{
		{base, "_ZTI4Base", "_ZTS4Base", "4Base"},
		{inner, "_ZTIN2ns5InnerE", "_ZTSN2ns5InnerE", "N2ns5InnerE"},
	} {
		ti, _ := TypeInfoName(Itanium, tc.rec)
		ts, _ := TypeNameName(Itanium, tc.rec)
		nm, _ := TypeName(Itanium, tc.rec)
		if ti != tc.ti || ts != tc.ts || nm != tc.nm {
			t.Errorf("%s: got %q %q %q", tc.rec.Name, ti, ts, nm)
		}
	}
}
