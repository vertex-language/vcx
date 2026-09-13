package parser

import (
	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
)

// isTypeStart reports whether kind can begin a type or declaration specifier.
func (p *parser) isTypeStart(k token.Kind) bool {
	switch k {
	case token.VOID, token.BOOL, token.CHAR, token.CHAR8_T, token.CHAR16_T, token.CHAR32_T,
		token.WCHAR_T, token.INT, token.SHORT, token.LONG, token.SIGNED, token.UNSIGNED,
		token.FLOAT, token.DOUBLE, token.AUTO:
		return true

	case token.INT8, token.INT16, token.INT32, token.INT64, token.INT128:
		return true

	case token.CONST, token.VOLATILE, token.RESTRICT, token.UNALIGNED:
		return true

	case token.STATIC, token.EXTERN, token.THREAD_LOCAL, token.MUTABLE, token.REGISTER:
		return true

	case token.INLINE, token.VIRTUAL, token.EXPLICIT, token.FRIEND, token.TYPEDEF:
		return true

	case token.CONSTEXPR, token.CONSTEVAL, token.CONSTINIT:
		return true

	case token.CLASS, token.STRUCT, token.UNION, token.ENUM:
		return true

	case token.TYPENAME, token.DECLTYPE:
		return true

	case token.IDENT, token.SCOPE:
		return true

	case token.ALIGNAS, token.ATTRIBUTE, token.DECLSPEC:
		return true
	}
	return false
}

// startsTypeId is isTypeStart for a parenthesized operand that may be a
// type-id or an expression -- sizeof's, typeid's: an identifier the
// source declared as a value opens an expression, `sizeof(arr[0])`, and
// one it declared as a type, or has not met, a type-id.
func (p *parser) startsTypeId() bool {
	if p.peek() == token.IDENT && p.kindOf(p.text(p.cur)) == nameValue {
		return false
	}
	return p.isTypeStart(p.peek())
}

// parseDeclSpecs parses a sequence of declaration specifiers.
func (p *parser) parseDeclSpecs() *ast.DeclSpecs {
	start := p.pos()
	specs := &ast.DeclSpecs{
		Span: ast.Span{Lo: start, Hi: start},
	}

	for !p.atEOF() {
		k := p.peek()

		// Basic keywords
		switch k {
		case token.VOID, token.BOOL, token.CHAR, token.CHAR8_T, token.CHAR16_T, token.CHAR32_T,
			token.WCHAR_T, token.INT, token.SHORT, token.LONG, token.SIGNED, token.UNSIGNED,
			token.FLOAT, token.DOUBLE, token.AUTO,
			token.INT8, token.INT16, token.INT32, token.INT64, token.INT128,
			token.CONST, token.VOLATILE, token.RESTRICT,
			token.STATIC, token.EXTERN, token.THREAD_LOCAL, token.MUTABLE, token.REGISTER,
			token.INLINE, token.VIRTUAL, token.FRIEND, token.TYPEDEF,
			token.CONSTEXPR, token.CONSTEVAL, token.CONSTINIT:
			tok := p.next()
			specs.List = append(specs.List, &ast.BasicSpec{
				Span: ast.Span{Lo: tok, Hi: tok + 1},
				Kind: k,
			})
			continue

		case token.EXPLICIT:
			tok := p.next()
			spec := &ast.ExplicitSpec{
				Span:    ast.Span{Lo: tok, Hi: tok + 1},
				Keyword: tok,
				Lparen:  ast.NoTok,
				Rparen:  ast.NoTok,
			}
			if p.peek() == token.LPAREN {
				spec.Lparen = p.next()
				spec.Cond = p.parseExpr()
				spec.Rparen = p.expect(token.RPAREN)
				spec.Span.Hi = spec.Rparen + 1
			}
			specs.List = append(specs.List, spec)
			continue

		case token.TYPENAME:
			tnTok := p.next()
			name := p.parseName()
			if name == nil {
				name = &ast.BadName{Span: ast.Span{Lo: p.pos(), Hi: p.pos() + 1}}
			}
			specs.List = append(specs.List, &ast.NamedTypeSpec{
				Span:     ast.Span{Lo: tnTok, Hi: name.End()},
				Typename: tnTok,
				Name:     name,
			})
			continue

		case token.DECLTYPE:
			dt := p.parseDecltypeSpec()
			specs.List = append(specs.List, dt)
			continue

		case token.CLASS, token.STRUCT, token.UNION:
			spec := p.parseClassOrElaboratedSpec()
			specs.List = append(specs.List, spec)
			continue

		case token.ENUM:
			spec := p.parseEnumSpec()
			specs.List = append(specs.List, spec)
			continue

		case token.ALIGNAS, token.DECLSPEC, token.ATTRIBUTE:
			// An attribute may appear anywhere among decl-specifiers; alignas is kept.
			if g := p.parseAttrGroup(); g != nil && (g.Align != nil || g.AlignX != nil) {
				specs.Aligns = append(specs.Aligns, g)
			}
			continue
		}

		// Skip MSVC calling conventions and pointer modifiers.
		if k == token.IDENT && msDiscarded[p.text(p.cur)] {
			p.next()
			continue
		}
		if k == token.UNALIGNED {
			p.next()
			continue
		}

		// A type-transformation builtin, `__decay(T)`: a type specifier
		// whose operand is a type-id.
		if k == token.IDENT && ast.TypeTransforms[p.text(p.cur)] && p.peekAt(1) == token.LPAREN && !p.hasTypeSpec(specs) {
			name := p.next()
			lp := p.expect(token.LPAREN)
			arg := p.parseTypeId()
			rp := p.expect(token.RPAREN)
			specs.List = append(specs.List, &ast.TypeTransformSpec{
				Span: ast.Span{Lo: name, Hi: rp + 1}, Name: name, Lparen: lp, Arg: arg, Rparen: rp,
			})
			continue
		}

		// Check if this is an identifier naming a type or concept
		if k == token.IDENT || k == token.SCOPE {
			// In C++, if we already have a type in specs.List (like int, or class X),
			// an identifier is the declarator name, not a type specifier!
			if p.hasTypeSpec(specs) {
				break
			}

			before := p.cur
			name := p.parseName()
			if name == nil {
				break
			}

			// `V::~V`, `V::operator int` and `V::operator+` are not types
			// under any reading: a qualified name ending in a destructor,
			// conversion or operator name is the declarator-id of an
			// out-of-line member definition, and the type-specifier-seq
			// before it was empty or has already been read. It is given
			// back to the declarator.
			if qn, isQualified := name.(*ast.QualifiedName); isQualified {
				switch qn.Name.(type) {
				case *ast.DestructorName, *ast.ConversionName, *ast.OperatorName:
					p.cur = before
					p.halfGtr = false
					name = nil
				}
			}
			if name == nil {
				break
			}

			// Check if this is a concept-constrained auto: Concept auto x
			if p.peek() == token.AUTO || (p.peek() == token.DECLTYPE && p.peekAt(2) == token.AUTO) {
				autoTok := p.pos()
				autoKind := p.peek()
				p.next()
				if autoKind == token.DECLTYPE {
					p.expect(token.LPAREN)
					p.expect(token.AUTO)
					p.expect(token.RPAREN)
				}
				specs.List = append(specs.List, &ast.ConstrainedAutoSpec{
					Span:    ast.Span{Lo: name.Pos(), Hi: p.pos()},
					Concept: name,
					Auto:    autoTok,
					Kind:    autoKind,
				})
				continue
			}

			specs.List = append(specs.List, &ast.NamedTypeSpec{
				Span:     ast.Span{Lo: name.Pos(), Hi: name.End()},
				Typename: ast.NoTok,
				Name:     name,
			})
			continue
		}

		break
	}

	if len(specs.List) > 0 {
		specs.Span.Hi = specs.List[len(specs.List)-1].End()
	}
	return specs
}

func (p *parser) hasTypeSpec(specs *ast.DeclSpecs) bool {
	for _, s := range specs.List {
		switch s := s.(type) {
		case *ast.BasicSpec:
			switch s.Kind {
			case token.VOID, token.BOOL, token.CHAR, token.CHAR8_T, token.CHAR16_T, token.CHAR32_T,
				token.WCHAR_T, token.INT, token.SHORT, token.LONG, token.SIGNED, token.UNSIGNED,
				token.FLOAT, token.DOUBLE, token.AUTO,
				token.INT8, token.INT16, token.INT32, token.INT64, token.INT128:
				return true
			}
		case *ast.NamedTypeSpec, *ast.ClassSpec, *ast.EnumSpec, *ast.ElaboratedSpec,
			*ast.DecltypeSpec, *ast.ConstrainedAutoSpec, *ast.TypeTransformSpec:
			return true
		}
	}
	return false
}

// parseDecltypeSpec parses decltype(expr) or decltype(auto).
func (p *parser) parseDecltypeSpec() *ast.DecltypeSpec {
	start := p.pos()
	kw := p.expect(token.DECLTYPE)
	lp := p.expect(token.LPAREN)

	dt := &ast.DecltypeSpec{
		Span:    ast.Span{Lo: start, Hi: lp + 1},
		Keyword: kw,
		Lparen:  lp,
		Auto:    ast.NoTok,
	}

	if p.peek() == token.AUTO {
		dt.Auto = p.next()
	} else {
		dt.X = p.parseExpr()
	}

	dt.Rparen = p.expect(token.RPAREN)
	dt.Span.Hi = dt.Rparen + 1
	return dt
}

// parseClassOrElaboratedSpec parses a class definition or elaborated-type-specifier.
func (p *parser) parseClassOrElaboratedSpec() ast.DeclSpec {
	start := p.pos()
	k := p.peek()
	kw := p.next()

	attrs := p.parseAttrGroups()
	var name ast.Name
	if p.peek() == token.IDENT || p.peek() == token.SCOPE {
		name = p.parseName()
	}

	// If no bases and no '{', this is an elaborated-type-specifier (e.g. `class C;` or `struct S x;`)
	if p.peek() != token.COLON && p.peek() != token.LBRACE && !(p.peek() == token.IDENT && p.text(p.cur) == "final") {
		var attrList []*ast.Attr
		for _, ag := range attrs {
			attrList = append(attrList, ag.Attrs...)
		}
		hi := kw + 1
		if name != nil {
			hi = name.End()
		}
		if id, isIdent := name.(*ast.Ident); isIdent {
			p.noteType(id.Text(p.u))
			if p.declaringTemplate {
				p.noteTemplate(id.Text(p.u), nameType)
				p.declaringTemplate = false
			}
		}
		return &ast.ElaboratedSpec{
			Span:    ast.Span{Lo: start, Hi: hi},
			Keyword: kw,
			Kind:    k,
			Attrs:   attrList,
			Name:    name,
		}
	}

	// Class definition: class Name finalopt : basesopt { members }
	//
	// The name is a type from here on -- the injected-class-name in the
	// body, the class after it -- and a template's when a template-head
	// declared it.
	if id, isIdent := name.(*ast.Ident); isIdent {
		p.noteType(id.Text(p.u))
		if p.declaringTemplate {
			p.noteTemplate(id.Text(p.u), nameType)
			p.declaringTemplate = false
		}
	}
	cs := &ast.ClassSpec{
		Span:    ast.Span{Lo: start, Hi: p.pos()},
		Keyword: kw,
		Kind:    k,
		Name:    name,
		Final:   ast.NoTok,
		Colon:   ast.NoTok,
	}
	for _, ag := range attrs {
		cs.Attrs = append(cs.Attrs, ag.Attrs...)
		if ag.Align != nil || ag.AlignX != nil {
			cs.Aligns = append(cs.Aligns, ag)
		}
	}

	// Check for contextual 'final'
	if p.peek() == token.IDENT && p.text(p.cur) == "final" {
		cs.Final = p.next()
	}

	// Base specifier list: `: public virtual B, private C`
	if p.peek() == token.COLON {
		cs.Colon = p.next()
		for !p.atEOF() && p.peek() != token.LBRACE {
			base := p.parseBaseSpec()
			if base != nil {
				cs.Bases = append(cs.Bases, base)
			}
			if p.peek() == token.COMMA {
				p.next()
			} else {
				break
			}
		}
	}

	if p.peek() == token.LBRACE {
		cs.Lbrace = p.next()
		// Members' names belong to the class scope, not enclosing scope.
		p.pushNames()
		defer p.popNames()
		for !p.atEOF() && p.peek() != token.RBRACE {
			prev := p.pos()
			d := p.parseMemberDecl()
			if d != nil {
				cs.Members = append(cs.Members, d)
			}
			if p.pos() == prev {
				p.advanceTo(token.SEMI, token.RBRACE)
				if p.peek() == token.SEMI {
					p.next()
				}
			}
		}
		cs.Rbrace = p.expect(token.RBRACE)
		cs.Span.Hi = cs.Rbrace + 1
	}

	return cs
}

// parseBaseSpec parses one base specifier: `public virtual Base<T>...`
func (p *parser) parseBaseSpec() *ast.BaseSpec {
	start := p.pos()
	attrs := p.parseAttrGroups()
	var attrList []*ast.Attr
	for _, ag := range attrs {
		attrList = append(attrList, ag.Attrs...)
	}

	base := &ast.BaseSpec{
		Span:     ast.Span{Lo: start, Hi: start},
		Attrs:    attrList,
		Virtual:  ast.NoTok,
		Access:   ast.NoTok,
		Ellipsis: ast.NoTok,
	}

	for {
		if p.peek() == token.VIRTUAL && base.Virtual == ast.NoTok {
			base.Virtual = p.next()
		} else if (p.peek() == token.PUBLIC || p.peek() == token.PROTECTED || p.peek() == token.PRIVATE) && base.Access == ast.NoTok {
			base.AccKind = p.peek()
			base.Access = p.next()
		} else {
			break
		}
	}

	base.Name = p.parseName()
	if base.Name == nil {
		p.error(p.cur, "expected base class name")
		return nil
	}

	if p.peek() == token.ELLIPSIS {
		base.Ellipsis = p.next()
		base.Span.Hi = base.Ellipsis + 1
	} else {
		base.Span.Hi = base.Name.End()
	}

	return base
}

// parseEnumSpec parses an enum definition or opaque declaration.
func (p *parser) parseEnumSpec() *ast.EnumSpec {
	start := p.pos()
	kw := p.expect(token.ENUM)
	scoped := ast.NoTok

	if p.peek() == token.CLASS || p.peek() == token.STRUCT {
		scoped = p.next()
	}

	attrs := p.parseAttrGroups()
	var attrList []*ast.Attr
	for _, ag := range attrs {
		attrList = append(attrList, ag.Attrs...)
	}

	var name ast.Name
	if p.peek() == token.IDENT || p.peek() == token.SCOPE {
		name = p.parseName()
		// The enum's name is a type from here on (see names.go).
		if id, isIdent := name.(*ast.Ident); isIdent {
			p.noteType(id.Text(p.u))
		}
	}

	es := &ast.EnumSpec{
		Span:    ast.Span{Lo: start, Hi: p.pos()},
		Keyword: kw,
		Kind:    token.ENUM,
		Scoped:  scoped,
		Attrs:   attrList,
		Name:    name,
		Colon:   ast.NoTok,
		Lbrace:  ast.NoTok,
		Rbrace:  ast.NoTok,
		Comma:   ast.NoTok,
	}

	// Underlying type: enum E : int { ... }
	if p.peek() == token.COLON {
		es.Colon = p.next()
		es.Base = p.parseDeclSpecs()
	}

	if p.peek() == token.LBRACE {
		es.Lbrace = p.next()
		for !p.atEOF() && p.peek() != token.RBRACE {
			en := p.parseEnumerator()
			if en != nil {
				es.Values = append(es.Values, en)
			}
			if p.peek() == token.COMMA {
				es.Comma = p.next()
			} else {
				break
			}
		}
		es.Rbrace = p.expect(token.RBRACE)
		es.Span.Hi = es.Rbrace + 1
	}

	return es
}

// parseEnumerator parses one NAME = value inside an enum.
func (p *parser) parseEnumerator() *ast.Enumerator {
	// Attribute-specifiers precede the identifier.
	start := p.pos()
	attrs := p.parseAttrGroups()
	var attrList []*ast.Attr
	for _, ag := range attrs {
		attrList = append(attrList, ag.Attrs...)
	}

	if p.peek() != token.IDENT {
		return nil
	}
	name := p.parseIdent()

	en := &ast.Enumerator{
		Span:   ast.Span{Lo: start, Hi: name.End()},
		Name:   name,
		Attrs:  attrList,
		Assign: ast.NoTok,
	}

	if p.peek() == token.ASSIGN {
		en.Assign = p.next()
		// The value is parsed as an assignment-expression so commas separate enumerators.
		en.Value = p.parseAssignmentExpr()
		en.Span.Hi = en.Value.End()
	}

	return en
}
