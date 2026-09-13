package sema

import (
	"strings"
	"testing"

	"github.com/vertex-language/vcx/parser"
	"github.com/vertex-language/vcx/preprocessor"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

func parseAndAnalyze(src string) (*Result, []Diagnostic) {
	f := token.NewFile("test.cpp", []byte(src))
	pp := preprocessor.New(preprocessor.Config{Std: token.Cxx23})
	toks, _ := pp.Run(f)
	unit := parser.NewUnit(toks)
	tree, _ := parser.Parse(unit, parser.DefaultMode)
	return Analyze(tree, types.LP64())
}

func TestOverloadResolution(t *testing.T) {
	// void f(int);
	// void f(double);
	fInt := &FuncSymbol{
		SymName: "f",
		FuncType: &types.Func{
			Ret:    types.Typ(types.Void),
			Params: []types.Param{{Name: "a", Type: types.Typ(types.Int)}},
		},
	}
	fDouble := &FuncSymbol{
		SymName: "f",
		FuncType: &types.Func{
			Ret:    types.Typ(types.Void),
			Params: []types.Param{{Name: "a", Type: types.Typ(types.Double)}},
		},
	}
	candidates := []*FuncSymbol{fInt, fDouble}

	// Call f(42) -> exact match for int
	res1, err1 := ResolveOverload(candidates, []Argument{{Type: types.Typ(types.Int), IsLValue: false}})
	if err1 != nil || res1 != fInt {
		t.Errorf("ResolveOverload(42) = %v (err=%v), want fInt", res1, err1)
	}

	// Call f(3.14) -> exact match for double
	res2, err2 := ResolveOverload(candidates, []Argument{{Type: types.Typ(types.Double), IsLValue: false}})
	if err2 != nil || res2 != fDouble {
		t.Errorf("ResolveOverload(3.14) = %v (err=%v), want fDouble", res2, err2)
	}

	// Call f((short)5) -> promotion to int is better than conversion to double
	res3, err3 := ResolveOverload(candidates, []Argument{{Type: types.Typ(types.Short), IsLValue: false}})
	if err3 != nil || res3 != fInt {
		t.Errorf("ResolveOverload(short) = %v (err=%v), want fInt via promotion", res3, err3)
	}
}

func TestSpecialMemberSynthesis(t *testing.T) {
	rec := &types.Record{
		Tag:      types.TagClass,
		Name:     "Widget",
		Complete: true,
	}
	SynthesizeSpecialMembers(rec)

	hasDefaultCtor := false
	hasCopyCtor := false
	hasMoveCtor := false
	hasCopyAssign := false
	hasMoveAssign := false
	hasDtor := false

	for _, m := range rec.Methods {
		switch m.Name {
		case "Widget":
			if len(m.Func.Params) == 0 {
				hasDefaultCtor = true
			} else if len(m.Func.Params) == 1 {
				if types.IsRValueReference(m.Func.Params[0].Type) {
					hasMoveCtor = true
				} else {
					hasCopyCtor = true
				}
			}
		case "~Widget":
			hasDtor = true
		case "operator=":
			if len(m.Func.Params) == 1 {
				if types.IsRValueReference(m.Func.Params[0].Type) {
					hasMoveAssign = true
				} else {
					hasCopyAssign = true
				}
			}
		}
	}

	if !hasDefaultCtor || !hasCopyCtor || !hasMoveCtor || !hasCopyAssign || !hasMoveAssign || !hasDtor {
		t.Errorf("SynthesizeSpecialMembers failed: default=%v, copy=%v, move=%v, copyAssign=%v, moveAssign=%v, dtor=%v",
			hasDefaultCtor, hasCopyCtor, hasMoveCtor, hasCopyAssign, hasMoveAssign, hasDtor)
	}
}

func TestAccessControl(t *testing.T) {
	baseRec := &types.Record{
		Tag:  types.TagClass,
		Name: "Base",
	}
	derivedRec := &types.Record{
		Tag:   types.TagClass,
		Name:  "Derived",
		Bases: []types.BaseSpec{{Type: baseRec, Access: types.AccessPublic}},
	}

	globalScope := NewScope(nil, GlobalScope, nil)
	baseScope := NewScope(globalScope, ClassScope, baseRec)
	derivedScope := NewScope(globalScope, ClassScope, derivedRec)

	// Public access
	if !CheckAccess(baseRec, types.AccessPublic, globalScope) {
		t.Errorf("public member should be accessible globally")
	}

	// Protected access
	if CheckAccess(baseRec, types.AccessProtected, globalScope) {
		t.Errorf("protected member should not be accessible globally")
	}
	if !CheckAccess(baseRec, types.AccessProtected, derivedScope) {
		t.Errorf("protected member of Base should be accessible from Derived")
	}

	// Private access
	if CheckAccess(baseRec, types.AccessPrivate, derivedScope) {
		t.Errorf("private member of Base should not be accessible from Derived")
	}
	if !CheckAccess(baseRec, types.AccessPrivate, baseScope) {
		t.Errorf("private member of Base should be accessible from Base")
	}
}

func TestAnalyzeValidCode(t *testing.T) {
	src := `
namespace math {
    int add(int a, int b) {
        return a + b;
    }
}

class Counter {
public:
    int value;
    int get() {
        return value;
    }
};

int main() {
    int x = math::add(1, 2);
    Counter c;
    c.value = x;
    return c.get();
}
`
	_, diags := parseAndAnalyze(src)
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Errorf("unexpected error in valid code: %s", d)
		}
	}
}

func TestAnalyzeTypeErrors(t *testing.T) {
	// 1. Assign string/pointer to int
	src1 := `
void test() {
    int x = "hello";
}
`
	_, diags1 := parseAndAnalyze(src1)
	if len(diags1) == 0 {
		t.Errorf("expected type mismatch error for int x = \"hello\"")
	}

	// 2. Modify const variable
	src2 := `
void test() {
    const int x = 10;
    x = 20;
}
`
	_, diags2 := parseAndAnalyze(src2)
	hasConstErr := false
	for _, d := range diags2 {
		if strings.Contains(d.Message, "const") {
			hasConstErr = true
			break
		}
	}
	if !hasConstErr {
		t.Errorf("expected error assigning to const variable")
	}

	// 3. Control reaches end of non-void function
	src3 := `
int test() {
    int a = 1;
}
`
	_, diags3 := parseAndAnalyze(src3)
	hasRetErr := false
	for _, d := range diags3 {
		if strings.Contains(d.Message, "does not return a value") {
			hasRetErr = true
			break
		}
	}
	if !hasRetErr {
		t.Errorf("expected missing return error")
	}

	// 4. Static assertion failure
	src4 := `
static_assert(0, "assertion failed!");
`
	_, diags4 := parseAndAnalyze(src4)
	hasAssertErr := false
	for _, d := range diags4 {
		if strings.Contains(d.Message, "assertion") {
			hasAssertErr = true
			break
		}
	}
	if !hasAssertErr {
		t.Errorf("expected static assertion failure error")
	}
}
