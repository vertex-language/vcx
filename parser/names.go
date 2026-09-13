package parser

import (
	"strings"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
)

// nameKind classifies names seen in scope to resolve parsing ambiguities
// between declarations and expressions.
type nameKind uint8

const (
	nameUnknown nameKind = iota
	nameType
	nameValue
	nameConcept
	nameTemplate // a class, variable or function template
)

// pushNames opens a scope: a block, a class body, a function's parameters,
// a template's parameters, a namespace.
func (p *parser) pushNames() {
	p.names = append(p.names, map[string]nameKind{})
}

func (p *parser) popNames() {
	if len(p.names) > 1 {
		p.names = p.names[:len(p.names)-1]
	}
}

// pushNamespaceNames opens the scope of the namespace at path under the
// current one -- the scope it had when it was last open, if it was.
func (p *parser) pushNamespaceNames(path []string) {
	for _, n := range path {
		p.nsPath = append(p.nsPath, n)
		key := strings.Join(p.nsPath, "::")
		scope, seen := p.namespaces[key]
		if !seen {
			scope = map[string]nameKind{}
			if p.namespaces == nil {
				p.namespaces = map[string]map[string]nameKind{}
			}
			p.namespaces[key] = scope
		}
		p.names = append(p.names, scope)
	}
	if len(path) == 0 {
		p.pushNames() // an unnamed namespace: its own each time
	}
}

func (p *parser) popNamespaceNames(path []string) {
	if len(path) == 0 {
		p.popNames()
		return
	}
	for range path {
		p.popNames()
		p.nsPath = p.nsPath[:len(p.nsPath)-1]
	}
}

func (p *parser) note(name string, k nameKind) {
	if name == "" || len(p.names) == 0 {
		return
	}
	p.names[len(p.names)-1][name] = k
}

// noteType records that name denotes a type from here on in this scope.
func (p *parser) noteType(name string) { p.note(name, nameType) }

// noteValue records that name denotes a function or object.
func (p *parser) noteValue(name string) { p.note(name, nameValue) }

// noteConcept records that name denotes a concept.
func (p *parser) noteConcept(name string) { p.note(name, nameConcept) }

// namesAType reports whether a name node is one the source declared as a
// type -- a class, a typedef, a template type parameter.
func (p *parser) namesAType(n ast.Name) bool {
	id, isIdent := n.(*ast.Ident)
	return isIdent && p.kindOf(id.Text(p.u)) == nameType
}

// namesAValue reports whether a name was declared as an object or non-template function.
func (p *parser) namesAValue(n ast.Name) bool {
	id, isIdent := n.(*ast.Ident)
	return isIdent && p.kindOf(id.Text(p.u)) == nameValue
}

// namesATemplate reports whether a name is one the source declared with
// a template-head: a `<` after it opens arguments.
func (p *parser) namesATemplate(n ast.Name) bool {
	id, isIdent := n.(*ast.Ident)
	if !isIdent {
		return false
	}
	k := p.kindOf(id.Text(p.u))
	return k == nameTemplate || k == nameType && p.declaredTemplate[id.Text(p.u)]
}

// noteTemplate records that name was declared by a template-declaration,
// as kind k in the scope the template-head was written in.
func (p *parser) noteTemplate(name string, k nameKind) {
	if p.declaredTemplate == nil {
		p.declaredTemplate = map[string]bool{}
	}
	p.declaredTemplate[name] = true
	if name != "" && p.templateOuter >= 0 && p.templateOuter < len(p.names) {
		p.names[p.templateOuter][name] = k
	}
}

// kindOf is what the innermost scope that mentions name says of it.
func (p *parser) kindOf(name string) nameKind {
	for i := len(p.names) - 1; i >= 0; i-- {
		if k, seen := p.names[i][name]; seen {
			return k
		}
	}
	return nameUnknown
}

// identMayBeType reports whether an identifier could begin a parameter-declaration.
func (p *parser) identMayBeType(tok ast.Tok) bool {
	return p.kindOf(p.text(tok)) != nameValue
}

// noteDeclarator records the name a declarator declares, as a type after
// `typedef` and as a value otherwise. Under a template-head the name is
// a template's, and a `<` after it opens arguments.
func (p *parser) noteDeclarator(specs *ast.DeclSpecs, d ast.Declarator) {
	if d == nil {
		return
	}
	// Whatever the declarator names -- an operator, a conversion -- it
	// is the template-head's declaration; the parameters noted after it
	// are not.
	declaringTemplate := p.declaringTemplate
	p.declaringTemplate = false
	id, isIdent := d.DeclName().(*ast.Ident)
	if !isIdent {
		return
	}
	if declaringTemplate {
		p.noteTemplate(id.Text(p.u), nameTemplate)
		return
	}
	if specs != nil && hasTypedef(specs) {
		p.noteType(id.Text(p.u))
		return
	}
	p.noteValue(id.Text(p.u))
}

func hasTypedef(specs *ast.DeclSpecs) bool {
	for _, s := range specs.List {
		if b, isBasic := s.(*ast.BasicSpec); isBasic && b.Kind == token.TYPEDEF {
			return true
		}
	}
	return false
}

// noteParams records a function's parameter names in the scope just
// opened for its body.
func (p *parser) noteParams(d ast.Declarator) {
	for d != nil {
		switch node := d.(type) {
		case *ast.FuncDeclarator:
			for _, prm := range node.Params {
				if prm != nil {
					p.noteDeclarator(nil, prm.Decl)
				}
			}
			return
		case *ast.PointerDeclarator:
			d = node.Inner
		case *ast.ParenDeclarator:
			d = node.Inner
		case *ast.ArrayDeclarator:
			d = node.Inner
		default:
			return
		}
	}
}

// msDiscarded are MSVC keywords/calling conventions ignored on 64-bit targets.
var msDiscarded = map[string]bool{
	"__cdecl": true, "__stdcall": true, "__fastcall": true, "__vectorcall": true,
	"__thiscall": true, "__clrcall": true,
	"__ptr32": true, "__ptr64": true, "__sptr": true, "__uptr": true, "__w64": true,
}

// skipMSDiscarded reads past any run of the discarded spellings.
func (p *parser) skipMSDiscarded() {
	for {
		switch {
		case p.peek() == token.UNALIGNED:
			p.next()
		case p.peek() == token.IDENT && msDiscarded[p.text(p.cur)]:
			p.next()
		default:
			return
		}
	}
}

// cannotFollowType reports a token no parameter-declaration can put after
// its type: an operator that only an expression has room for.
func cannotFollowType(k token.Kind) bool {
	switch k {
	case token.PERIOD, token.ARROW, token.INC, token.DEC,
		token.ADD, token.SUB, token.QUO, token.REM,
		token.EQL, token.NEQ, token.LEQ, token.GEQ, token.LOR,
		token.NOT, token.QUESTION, token.ASSIGN, token.PERIOD_STAR, token.ARROW_STAR,
		token.INT_LIT, token.FLOAT_LIT, token.STRING_LIT, token.CHAR_LIT:
		return true
	}
	return false
}

// typeTraits are compiler builtins whose operands are type-ids.
var typeTraits = map[string]bool{
	"__builtin_is_constant_evaluated": true, "__is_constant_evaluated": true, "__builtin_offsetof": true,
	"__is_same": true, "__is_same_as": true, "__is_base_of": true, "__is_class": true, "__is_union": true,
	"__is_enum": true, "__is_pod": true, "__is_empty": true, "__is_polymorphic": true, "__is_abstract": true,
	"__is_final": true, "__is_sealed": true, "__is_standard_layout": true, "__is_trivial": true,
	"__is_trivially_copyable": true, "__is_trivially_constructible": true, "__is_trivially_assignable": true,
	"__is_trivially_destructible": true, "__is_nothrow_constructible": true, "__is_nothrow_assignable": true,
	"__is_nothrow_destructible": true, "__is_constructible": true, "__is_assignable": true,
	"__is_assignable_no_precondition_check": true, "__is_destructible": true, "__is_convertible_to": true,
	"__is_convertible": true, "__is_nothrow_convertible": true, "__has_trivial_constructor": true,
	"__has_trivial_copy": true, "__has_trivial_assign": true, "__has_trivial_destructor": true,
	"__has_nothrow_constructor": true, "__has_nothrow_copy": true, "__has_nothrow_assign": true,
	"__has_virtual_destructor": true, "__has_unique_object_representations": true, "__is_literal_type": true,
	"__is_aggregate": true, "__is_member_pointer": true, "__is_member_function_pointer": true,
	"__is_member_object_pointer": true, "__is_const": true, "__is_volatile": true, "__is_reference": true,
	"__is_lvalue_reference": true, "__is_rvalue_reference": true, "__is_pointer": true, "__is_void": true,
	"__is_integral": true, "__is_floating_point": true, "__is_arithmetic": true, "__is_array": true,
	"__is_function": true, "__is_fundamental": true, "__is_object": true, "__is_scalar": true,
	"__is_compound": true, "__is_signed": true, "__is_unsigned": true, "__is_bounded_array": true,
	"__is_unbounded_array": true, "__is_scoped_enum": true, "__is_nullptr": true, "__is_layout_compatible": true,
	"__is_pointer_interconvertible_base_of": true, "__is_pointer_interconvertible_with_class": true,
	"__is_corresponding_member": true, "__underlying_type": true, "__array_rank": true, "__array_extent": true,
	"__reference_binds_to_temporary": true, "__reference_constructs_from_temporary": true,
	"__is_invocable": true, "__is_nothrow_invocable": true, "__is_trivially_relocatable": true,
}
