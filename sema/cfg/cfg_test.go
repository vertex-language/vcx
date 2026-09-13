package cfg

import (
	"testing"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/types"
)

func TestCFGReturnAnalysis(t *testing.T) {
	// Function body: { return 1; }
	bodyWithReturn := &ast.CompoundStmt{
		Stmts: []ast.Stmt{
			&ast.ReturnStmt{
				X: &ast.BasicLit{},
			},
		},
	}
	cfg1 := Build(bodyWithReturn, nil)
	diags1 := CheckReturns(cfg1, "foo", types.Typ(types.Int))
	if len(diags1) != 0 {
		t.Errorf("expected no return errors, got: %v", diags1)
	}

	// Function body: { int x = 1; } -> missing return in non-void function
	bodyMissingReturn := &ast.CompoundStmt{
		Stmts: []ast.Stmt{
			&ast.ExprStmt{
				X: &ast.BasicLit{},
			},
		},
	}
	cfg2 := Build(bodyMissingReturn, nil)
	diags2 := CheckReturns(cfg2, "bar", types.Typ(types.Int))
	if len(diags2) == 0 {
		t.Errorf("expected missing return error for bar")
	}

	// Void function with missing return is valid in C++
	diagsVoid := CheckReturns(cfg2, "baz", types.Typ(types.Void))
	if len(diagsVoid) != 0 {
		t.Errorf("void function should not have return errors")
	}
}

func TestCFGUnreachableCode(t *testing.T) {
	// Function body: { return 1; int x = 2; }
	body := &ast.CompoundStmt{
		Stmts: []ast.Stmt{
			&ast.ReturnStmt{
				X: &ast.BasicLit{},
			},
			&ast.ExprStmt{
				X: &ast.BasicLit{},
			},
		},
	}
	cfg := Build(body, nil)
	unreachable := CheckUnreachable(cfg)
	if len(unreachable) == 0 {
		t.Errorf("expected unreachable statement detection after return")
	}
}
