package constexpr

import (
	"testing"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/parser"
	"github.com/vertex-language/vcx/preprocessor"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

func parseExpr(src string) (ast.Expr, ast.Unit) {
	full := "auto _dummy = " + src + ";"
	f := token.NewFile("test.cpp", []byte(full))
	pp := preprocessor.New(preprocessor.Config{Std: token.Cxx23})
	toks, _ := pp.Run(f)
	unit := parser.NewUnit(toks)
	tree, _ := parser.Parse(unit, parser.DefaultMode)

	if len(tree.Decls) > 0 {
		if sd, ok := tree.Decls[0].(*ast.SimpleDecl); ok && len(sd.Inits) > 0 {
			return sd.Inits[0].Value, unit
		}
	}
	return nil, unit
}

func parseFunc(src string) (*ast.FuncDecl, ast.Unit) {
	f := token.NewFile("test.cpp", []byte(src))
	pp := preprocessor.New(preprocessor.Config{Std: token.Cxx23})
	toks, _ := pp.Run(f)
	unit := parser.NewUnit(toks)
	tree, _ := parser.Parse(unit, parser.DefaultMode)
	if len(tree.Decls) > 0 {
		if fd, ok := tree.Decls[0].(*ast.FuncDecl); ok {
			return fd, unit
		}
	}
	return nil, unit
}

func TestArithmeticAndBitwise(t *testing.T) {
	tests := []struct {
		expr string
		want int64
	}{
		{"1 + 2", 3},
		{"10 - 4", 6},
		{"3 * 7", 21},
		{"20 / 4", 5},
		{"23 % 5", 3},
		{"2 + 3 * 4", 14},
		{"(2 + 3) * 4", 20},
		{"1 << 4", 16},
		{"32 >> 2", 8},
		{"0xFF & 0x0F", 15},
		{"0x10 | 0x01", 17},
		{"0xAA ^ 0xFF", 85},
		{"~0 & 0xFF", 255},
		{"- -42", 42},
		{"true ? 100 : 200", 100},
		{"false ? 100 : 200", 200},
		{"1 < 2 ? 10 : 20", 10},
		{"5 >= 5 ? 1 : 0", 1},
		{"3 == 3", 1},
		{"3 != 4", 1},
		{"(true && false) ? 1 : 0", 0},
		{"(true || false) ? 1 : 0", 1},
		{"!false ? 42 : 0", 42},
		{"1'000 + 2'000", 3000},
		{"0x10 + 0b101", 21},
	}

	for _, tt := range tests {
		exprNode, unit := parseExpr(tt.expr)
		if exprNode == nil {
			t.Fatalf("failed to parse expression: %s", tt.expr)
		}
		ctx := NewContext(unit, types.LP64())
		val, err := ctx.EvalInt(exprNode)
		if err != nil {
			t.Errorf("EvalInt(%q) failed: %v", tt.expr, err)
			continue
		}
		if val != tt.want {
			t.Errorf("EvalInt(%q) = %d, want %d", tt.expr, val, tt.want)
		}
	}
}

func TestUndefinedBehaviorDetection(t *testing.T) {
	tests := []struct {
		expr string
		name string
	}{
		{"10 / 0", "division by zero"},
		{"10 % 0", "modulo by zero"},
		{"1 << 100", "shift count too large"},
		{"1 << -1", "negative shift count"},
	}

	for _, tt := range tests {
		exprNode, unit := parseExpr(tt.expr)
		if exprNode == nil {
			t.Fatalf("failed to parse expression: %s", tt.expr)
		}
		ctx := NewContext(unit, types.LP64())
		_, err := ctx.Eval(exprNode)
		if err == nil {
			t.Errorf("expected error for %s (%q), got success", tt.name, tt.expr)
		}
	}
}

func TestShortCircuitEvaluation(t *testing.T) {
	// false && (1/0): must not evaluate 1/0
	expr1, u1 := parseExpr("false && (10 / 0 == 0)")
	ctx1 := NewContext(u1, types.LP64())
	v1, err1 := ctx1.EvalBool(expr1)
	if err1 != nil || v1 != false {
		t.Errorf("false && (1/0) failed or triggered UB: v=%v, err=%v", v1, err1)
	}

	// true || (1/0): must not evaluate 1/0
	expr2, u2 := parseExpr("true || (10 / 0 == 0)")
	ctx2 := NewContext(u2, types.LP64())
	v2, err2 := ctx2.EvalBool(expr2)
	if err2 != nil || v2 != true {
		t.Errorf("true || (1/0) failed or triggered UB: v=%v, err=%v", v2, err2)
	}

	// false ? (1/0) : 42
	expr3, u3 := parseExpr("false ? (10 / 0) : 42")
	ctx3 := NewContext(u3, types.LP64())
	v3, err3 := ctx3.EvalInt(expr3)
	if err3 != nil || v3 != 42 {
		t.Errorf("false ? (1/0) : 42 failed or triggered UB: v=%v, err=%v", v3, err3)
	}
}

func TestConstexprFunctionAndLoops(t *testing.T) {
	src := `
constexpr int factorial(int n) {
    int res = 1;
    for (int i = 2; i <= n; i = i + 1) {
        res = res * i;
    }
    return res;
}
`
	fnDecl, unit := parseFunc(src)
	if fnDecl == nil {
		t.Fatalf("failed to parse function:\n%s", src)
	}

	ctx := NewContext(unit, types.LP64())
	body, ok := fnDecl.Body.(*ast.CompoundStmt)
	if !ok {
		t.Fatalf("function body is not compound stmt: %T", fnDecl.Body)
	}

	ctx.RegisterFunc(&FuncInfo{
		Unit:       unit,
		Name:       "factorial",
		ParamNames: []string{"n"},
		ParamTypes: []types.Type{types.Typ(types.Int)},
		Body:       body,
		RetType:    types.Typ(types.Int),
	})

	callExpr, u2 := parseExpr("factorial(5)")
	ctx.Unit = u2
	val, err := ctx.EvalInt(callExpr)
	if err != nil {
		t.Fatalf("failed to eval factorial(5): %v", err)
	}
	if val != 120 {
		t.Errorf("factorial(5) = %d, want 120", val)
	}
}

func TestRecursiveConstexpr(t *testing.T) {
	src := `
constexpr int fib(int n) {
    if (n <= 1) {
        return n;
    }
    return fib(n - 1) + fib(n - 2);
}
`
	fnDecl, unit := parseFunc(src)
	if fnDecl == nil {
		t.Fatalf("failed to parse recursive function")
	}

	ctx := NewContext(unit, types.LP64())
	body, _ := fnDecl.Body.(*ast.CompoundStmt)
	ctx.RegisterFunc(&FuncInfo{
		Unit:       unit,
		Name:       "fib",
		ParamNames: []string{"n"},
		ParamTypes: []types.Type{types.Typ(types.Int)},
		Body:       body,
		RetType:    types.Typ(types.Int),
	})

	callExpr, u2 := parseExpr("fib(7)")
	ctx.Unit = u2
	val, err := ctx.EvalInt(callExpr)
	if err != nil {
		t.Fatalf("failed to eval fib(7): %v", err)
	}
	if val != 13 {
		t.Errorf("fib(7) = %d, want 13", val)
	}
}

func TestPointersAndArrayLValues(t *testing.T) {
	ctx := NewContext(nil, types.LP64())

	// Allocate array: int arr[3] = {10, 20, 30};
	arrVal := NewArray([]Value{
		NewInt(10, types.Typ(types.Int), ctx.Model),
		NewInt(20, types.Typ(types.Int), ctx.Model),
		NewInt(30, types.Typ(types.Int), ctx.Model),
	}, types.Typ(types.Int))

	obj := ctx.DeclareVar("arr", arrVal.Type(), arrVal, false)

	// Test read subobject arr[1]
	elem, err := obj.Read([]PathStep{{Kind: PathIndex, Index: 1}}, 0)
	if err != nil {
		t.Fatalf("Read arr[1] failed: %v", err)
	}
	if iv, ok := elem.(IntValue); !ok || iv.Int64() != 20 {
		t.Errorf("arr[1] = %v, want 20", elem)
	}

	// Test write subobject arr[1] = 99
	err = obj.Write([]PathStep{{Kind: PathIndex, Index: 1}}, 0, NewInt(99, types.Typ(types.Int), ctx.Model))
	if err != nil {
		t.Fatalf("Write arr[1] failed: %v", err)
	}

	elem2, _ := obj.Read([]PathStep{{Kind: PathIndex, Index: 1}}, 0)
	if iv, ok := elem2.(IntValue); !ok || iv.Int64() != 99 {
		t.Errorf("arr[1] after write = %v, want 99", elem2)
	}

	// Test pointer arithmetic: ptr = &arr[0]; ptr + 2 -> arr[2]
	ptrVal := PointerValue{Target: obj, Path: []PathStep{{Kind: PathIndex, Index: 0}}, Offset: 0}
	ptrPlus2 := PointerValue{Target: obj, Path: []PathStep{{Kind: PathIndex, Index: 2}}, Offset: 0}
	if ptrVal.Equal(ptrPlus2) {
		t.Errorf("&arr[0] should not equal &arr[2]")
	}
}
