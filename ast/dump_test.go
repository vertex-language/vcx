package ast

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vertex-language/vcx/token"
)

type dummyUnit struct{}

func (dummyUnit) Len() int                { return 10 }
func (dummyUnit) Text(t Tok) string       { return "foo" }
func (dummyUnit) Kind(t Tok) token.Kind   { return token.IDENT }
func (dummyUnit) Position(t Tok) Position { return Position{Filename: "test.cpp", Line: 1, Column: 1} }
func (dummyUnit) Between(a, b Tok) string { return " " }

func TestDump(t *testing.T) {
	u := dummyUnit{}
	file := &File{
		Span: Span{Lo: 0, Hi: 5},
		Decls: []Decl{
			&SimpleDecl{
				Span: Span{Lo: 0, Hi: 4},
				Specs: &DeclSpecs{
					Span: Span{Lo: 0, Hi: 1},
					List: []DeclSpec{
						&BasicSpec{
							Span: Span{Lo: 0, Hi: 1},
							Kind: token.INT,
						},
					},
				},
				Inits: []*InitDeclarator{
					{
						Span: Span{Lo: 1, Hi: 4},
						Decl: &NameDeclarator{
							Span: Span{Lo: 1, Hi: 2},
							Name: &Ident{Span: Span{Lo: 1, Hi: 2}},
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := Fdump(&buf, u, file); err != nil {
		t.Fatalf("Fdump failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "File 1:1") {
		t.Errorf("expected File 1:1 in dump, got:\n%s", out)
	}
	if !strings.Contains(out, "BasicSpec 1:1 int") {
		t.Errorf("expected BasicSpec int in dump, got:\n%s", out)
	}
	if !strings.Contains(out, "Ident 1:1 foo") {
		t.Errorf("expected Ident foo in dump, got:\n%s", out)
	}

	str := Dump(u, file)
	if str != out {
		t.Errorf("Dump and Fdump output mismatch")
	}
}
