package ast

import (
	"fmt"
	"io"
	"reflect"
	"strings"
)

// Fdump prints the tree rooted at n to w, one node per line, indented by depth.
// Identifiers and literals appear with their resolved text, and every node with
// its source position.
func Fdump(w io.Writer, u Unit, n Node) error {
	d := &dumper{w: w, u: u}
	d.dump(n, 0)
	return d.err
}

// Dump returns a string rendering of the tree rooted at n.
func Dump(u Unit, n Node) string {
	var b strings.Builder
	_ = Fdump(&b, u, n)
	return b.String()
}

type dumper struct {
	w   io.Writer
	u   Unit
	err error
}

func (d *dumper) printf(format string, args ...any) {
	if d.err == nil {
		_, d.err = fmt.Fprintf(d.w, format, args...)
	}
}

func (d *dumper) dump(n Node, depth int) {
	if d.err != nil || n == nil || isNil(n) {
		return
	}
	for i := 0; i < depth; i++ {
		d.printf("  ")
	}

	posStr := ""
	if d.u != nil && n.Pos().IsValid() {
		p := d.u.Position(n.Pos())
		posStr = fmt.Sprintf(" %d:%d", p.Line, p.Column)
	}

	d.printf("%s%s", nodeName(n), posStr)

	switch n := n.(type) {
	case *Ident:
		if d.u != nil {
			d.printf(" %s", n.Text(d.u))
		}
	case *BasicLit:
		if d.u != nil {
			d.printf(" %s %s", n.Kind, d.u.Text(n.Lo))
		} else {
			d.printf(" %s", n.Kind)
		}
	case *StringLit:
		if d.u != nil {
			for _, seg := range n.Segs {
				d.printf(" %s", d.u.Text(seg.Lo))
			}
		}
	case *BasicSpec:
		d.printf(" %s", n.Kind)
	case *ClassSpec:
		d.printf(" %s", n.Kind)
	case *EnumSpec:
		d.printf(" %s", n.Kind)
	case *BinaryExpr:
		d.printf(" %s", n.Op)
	case *AssignExpr:
		d.printf(" %s", n.Op)
	case *UnaryExpr:
		d.printf(" %s", n.Op)
	case *IncDecExpr:
		d.printf(" %s", n.Op)
	case *MemberExpr:
		d.printf(" %s", n.Op)
	case *NamedCastExpr:
		d.printf(" %s", n.Kind)
	case *OperatorName:
		d.printf(" %s", n.Op)
	case *TypeParam:
		if n.Kind != 0 {
			d.printf(" %s", n.Kind)
		}
	case *TemplateTemplateParam:
		d.printf(" %s", n.Kind)
	case *AccessDecl:
		d.printf(" %s", n.Kind)
	}
	d.printf("\n")

	for _, c := range children(n) {
		d.dump(c, depth+1)
	}
}

func nodeName(n Node) string {
	t := reflect.TypeOf(n)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}
