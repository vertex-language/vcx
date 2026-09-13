package ast

import (
	goast "go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	vtok "github.com/vertex-language/vcx/token"
)

// ---- structural invariants ----

// Every exported struct type in this package is a Node, and a Node is a
// stored extent. The check reads the package's own source rather than a
// hand-kept list, so a node added without a Span is caught the day it is
// added and nobody has to remember to update a table.
func TestEveryNodeEmbedsSpan(t *testing.T) {
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		f, err := goparser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		goast.Inspect(f, func(node goast.Node) bool {
			ts, ok := node.(*goast.TypeSpec)
			if !ok || !ts.Name.IsExported() {
				return true
			}
			st, ok := ts.Type.(*goast.StructType)
			if !ok {
				return true
			}
			// The two value types in the package: an extent, and a
			// resolved location. Neither is a node, and both are named
			// here so that adding a third is a deliberate act.
			// PackAt is a third: a fact beside the tree rather than in
			// it, keyed by position.
			if ts.Name.Name == "Span" || ts.Name.Name == "Position" || ts.Name.Name == "PackAt" {
				return true
			}
			n++
			if len(st.Fields.List) == 0 {
				t.Errorf("%s has no fields; every node embeds Span", ts.Name.Name)
				return true
			}
			first := st.Fields.List[0]
			id, ok := first.Type.(*goast.Ident)
			if len(first.Names) != 0 || !ok || id.Name != "Span" {
				t.Errorf("%s does not embed Span as its first field", ts.Name.Name)
			}
			return true
		})
	}
	if n < 80 {
		t.Errorf("only found %d node types; the scan is not seeing the package", n)
	}
	t.Logf("%d node types", n)
}

// A Name is an Expr. The hierarchy is nested rather than parallel because an
// id-expression is a primary expression, and a parser that had to convert
// between the two would be converting at every use.
func TestNamesAreExpressions(t *testing.T) {
	names := []Name{
		&Ident{}, &BadName{}, &QualifiedName{}, &TemplateName{},
		&OperatorName{}, &ConversionName{}, &LiteralOperatorName{},
		&DestructorName{}, &PackName{},
	}
	for _, n := range names {
		var _ Expr = n
	}
}

// The hierarchies are closed: a node belongs to exactly the ones its marker
// methods say, and the marker methods are what the parser's return types rely
// on.
func TestHierarchyMembership(t *testing.T) {
	var _ Expr = (*LambdaExpr)(nil)
	var _ Expr = (*FoldExpr)(nil)
	var _ Expr = (*RequiresExpr)(nil)
	var _ Stmt = (*RangeForStmt)(nil)
	var _ Stmt = (*TryStmt)(nil)
	var _ Decl = (*TemplateDecl)(nil)
	var _ Decl = (*ConceptDecl)(nil)
	var _ Decl = (*ModuleDecl)(nil)
	var _ Decl = (*StructuredBinding)(nil)
	var _ Declarator = (*PointerDeclarator)(nil)
	var _ Declarator = (*FuncDeclarator)(nil)
	var _ DeclSpec = (*ClassSpec)(nil)
	var _ DeclSpec = (*DecltypeSpec)(nil)
	var _ DeclSpec = (*ConstrainedAutoSpec)(nil)
}

func TestSpan(t *testing.T) {
	n := &BasicLit{Span: Span{Lo: 3, Hi: 7}}
	if n.Pos() != 3 || n.End() != 7 {
		t.Errorf("span = [%d,%d)", n.Pos(), n.End())
	}
}

// ---- DeclName ----

// A declarator's name is found through the chain, and an abstract declarator
// has none — which is what makes a type-id and a named declaration the same
// production.
func TestDeclName(t *testing.T) {
	name := &Ident{Span: Span{Lo: 10, Hi: 11}}
	// int (*p)[4]
	d := Declarator(&ArrayDeclarator{
		Inner: &ParenDeclarator{
			Inner: &PointerDeclarator{
				Kind:  vtok.MUL,
				Inner: &NameDeclarator{Name: name},
			},
		},
	})
	if got := d.DeclName(); got != Name(name) {
		t.Errorf("DeclName = %v, want the identifier", got)
	}

	abstract := Declarator(&PointerDeclarator{
		Kind:  vtok.MUL,
		Inner: &NameDeclarator{},
	})
	if got := abstract.DeclName(); got != nil {
		t.Errorf("an abstract declarator should have no name, got %v", got)
	}

	if got := (&BadDeclarator{}).DeclName(); got != nil {
		t.Errorf("BadDeclarator.DeclName = %v", got)
	}
	// A chain ending in nothing must not panic.
	if got := (&PointerDeclarator{}).DeclName(); got != nil {
		t.Errorf("nil inner should give no name, got %v", got)
	}
}

// ---- walking ----

func TestWalkFindsChildren(t *testing.T) {
	// f(a, b + 1)
	tree := &CallExpr{
		Fun: &Ident{Span: Span{Lo: 1, Hi: 2}},
		Args: []Expr{
			&Ident{Span: Span{Lo: 3, Hi: 4}},
			&BinaryExpr{
				X:  &Ident{Span: Span{Lo: 5, Hi: 6}},
				Op: vtok.ADD,
				Y:  &BasicLit{Span: Span{Lo: 7, Hi: 8}, Kind: vtok.INT_LIT},
			},
		},
	}
	var kinds []string
	Inspect(tree, func(n Node) bool {
		kinds = append(kinds, strings.TrimPrefix(reflect.TypeOf(n).String(), "*ast."))
		return true
	})
	want := []string{"CallExpr", "Ident", "Ident", "BinaryExpr", "Ident", "BasicLit"}
	if strings.Join(kinds, " ") != strings.Join(want, " ") {
		t.Errorf("walk order = %v, want %v", kinds, want)
	}
}

func TestWalkSkipsNilsAndExtents(t *testing.T) {
	// A node with every optional field empty must still walk cleanly, and
	// StringLit's segment spans are extents rather than children.
	lit := &StringLit{
		Span: Span{Lo: 1, Hi: 9},
		Segs: []Span{{Lo: 1, Hi: 5}, {Lo: 5, Hi: 9}},
	}
	count := 0
	Inspect(lit, func(Node) bool { count++; return true })
	if count != 1 {
		t.Errorf("StringLit walked %d nodes, want 1", count)
	}

	// An if with no init, no else, and a nil condition.
	s := &IfStmt{Then: &EmptyStmt{Span: Span{Lo: 1, Hi: 2}}}
	count = 0
	Inspect(s, func(Node) bool { count++; return true })
	if count != 2 {
		t.Errorf("IfStmt walked %d nodes, want 2", count)
	}
}

func TestInspectCanPrune(t *testing.T) {
	tree := &ParenExpr{
		X: &BinaryExpr{
			X: &Ident{Span: Span{Lo: 1, Hi: 2}},
			Y: &Ident{Span: Span{Lo: 3, Hi: 4}},
		},
	}
	count := 0
	Inspect(tree, func(n Node) bool {
		count++
		_, isBinary := n.(*BinaryExpr)
		return !isBinary // do not descend into the binary expression
	})
	if count != 2 {
		t.Errorf("visited %d nodes, want 2 (the paren and the binary)", count)
	}
}

// Walk reaches through the interface-typed fields that make up most of the
// C++ tree — a declarator chain, a name, a type-id.
func TestWalkReachesTypeSyntax(t *testing.T) {
	// static_cast<const T*>(x)
	tree := &NamedCastExpr{
		Kind: vtok.STATIC_CAST,
		Type: &TypeId{
			Specs: &DeclSpecs{List: []DeclSpec{
				&BasicSpec{Span: Span{Lo: 1, Hi: 6}, Kind: vtok.CONST},
				&NamedTypeSpec{Name: &Ident{Span: Span{Lo: 7, Hi: 8}}},
			}},
			Decl: &PointerDeclarator{Kind: vtok.MUL, Inner: &NameDeclarator{}},
		},
		X: &Ident{Span: Span{Lo: 11, Hi: 12}},
	}
	var seen []string
	Inspect(tree, func(n Node) bool {
		seen = append(seen, strings.TrimPrefix(reflect.TypeOf(n).String(), "*ast."))
		return true
	})
	want := "NamedCastExpr TypeId DeclSpecs BasicSpec NamedTypeSpec Ident " +
		"PointerDeclarator NameDeclarator Ident"
	if got := strings.Join(seen, " "); got != want {
		t.Errorf("walk = %q\nwant %q", got, want)
	}
}

// ---- File ----

func TestFileRelease(t *testing.T) {
	released := 0
	f := &File{}
	f.SetReleaser(releaserFunc(func() { released++ }))
	f.Release()
	f.Release() // safe twice
	if released != 1 {
		t.Errorf("released %d times, want 1", released)
	}
	// Safe with no releaser.
	(&File{}).Release()
}

type releaserFunc func()

func (r releaserFunc) Release() { r() }

// testUnit is the smallest thing that satisfies Unit: a token stream is a
// slice of spellings. That the interface can be met this cheaply is the point
// of declaring it here rather than importing one — ast asks for what the tree
// needs and learns nothing else about phase 4.
type testUnit []string

func (u testUnit) Len() int                { return len(u) }
func (u testUnit) Text(t Tok) string       { return u[t] }
func (u testUnit) Kind(Tok) vtok.Kind      { return vtok.IDENT }
func (u testUnit) Position(Tok) Position   { return Position{Filename: "a.cpp", Line: 1} }
func (u testUnit) Between(a, b Tok) string { return " " }

func TestIdentText(t *testing.T) {
	u := testUnit{"int", "value", "=", "1", ";"}
	id := &Ident{Span: Span{Lo: 1, Hi: 2}}
	if got := id.Text(u); got != "value" {
		t.Errorf("Text = %q", got)
	}
	var nilID *Ident
	if got := nilID.Text(u); got != "" {
		t.Errorf("nil Ident.Text = %q, want empty", got)
	}
	// An index that was never written resolves to nothing rather than
	// panicking, because half the fields in this package are optional.
	absent := &Ident{Span: Span{Lo: NoTok, Hi: NoTok}}
	if got := absent.Text(u); got != "" {
		t.Errorf("absent Ident.Text = %q, want empty", got)
	}
}

func TestTokValidity(t *testing.T) {
	if NoTok.IsValid() {
		t.Error("NoTok should not be valid")
	}
	if !Tok(0).IsValid() {
		t.Error("index 0 is a real token")
	}
}
