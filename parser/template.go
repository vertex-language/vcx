package parser

import (
	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
)

// parseTemplateDecl parses template < ... > decl, explicit specializations, or explicit instantiations.
func (p *parser) parseTemplateDecl() ast.Decl {
	start := p.pos()
	externTok := ast.NoTok

	if p.peek() == token.EXTERN && p.peekAt(1) == token.TEMPLATE {
		externTok = p.next()
	}

	tmplTok := p.expect(token.TEMPLATE)

	// Check explicit instantiation: `extern_opt template decl` (no '<' immediately after template)
	if p.peek() != token.LSS {
		decl := p.parseDecl()
		hi := p.pos()
		if decl != nil {
			hi = decl.End()
		}
		return &ast.ExplicitInstDecl{
			Span:    ast.Span{Lo: start, Hi: hi},
			Extern:  externTok,
			Keyword: tmplTok,
			Decl:    decl,
		}
	}

	// Template parameters: template < ... >
	less := p.expect(token.LSS)

	// Explicit specialization: template <> decl
	if p.peek() == token.GTR {
		gtr := p.next()
		decl := p.parseDecl()
		hi := gtr + 1
		if decl != nil {
			hi = decl.End()
		}
		return &ast.ExplicitSpecDecl{
			Span:    ast.Span{Lo: start, Hi: hi},
			Keyword: tmplTok,
			Less:    less,
			Greater: gtr,
			Decl:    decl,
		}
	}

	// The parameters' names are in scope for the declaration and end
	// with it. The declared name, though, belongs to the scope the
	// template-head was written in -- the outermost one when heads nest
	// (`template <class T> template <class U>`) -- and that is where
	// noteTemplate puts it.
	if p.templateHeads == 0 {
		p.templateOuter = len(p.names) - 1
	}
	p.templateHeads++
	defer func() { p.templateHeads-- }()
	p.pushNames()
	defer p.popNames()
	params := p.parseTemplateParamsContent(tmplTok, less)

	var req *ast.RequiresClause
	if p.peek() == token.REQUIRES {
		req = p.parseRequiresClause()
	}

	// Check if this is a concept declaration: concept C = ...;
	if p.peek() == token.CONCEPT {
		conc := p.parseConceptDecl()
		return &ast.TemplateDecl{
			Span:     ast.Span{Lo: start, Hi: conc.End()},
			Params:   params,
			Requires: req,
			Decl:     conc,
		}
	}

	// The next declarator's name is a template's.
	p.declaringTemplate = true
	decl := p.parseDecl()
	p.declaringTemplate = false
	hi := p.pos()
	if decl != nil {
		hi = decl.End()
	}

	return &ast.TemplateDecl{
		Span:     ast.Span{Lo: start, Hi: hi},
		Params:   params,
		Requires: req,
		Decl:     decl,
	}
}

func (p *parser) parseTemplateParamsContent(tmplTok, less ast.Tok) *ast.TemplateParams {
	tp := &ast.TemplateParams{
		Span:    ast.Span{Lo: tmplTok, Hi: less + 1},
		Keyword: tmplTok,
		Less:    less,
	}

	for !p.atEOF() && p.peek() != token.GTR && p.peek() != token.SHR {
		param := p.parseSingleTemplateParam()
		if param != nil {
			tp.Params = append(tp.Params, param)
		}
		if p.peek() == token.COMMA {
			p.next()
		} else {
			break
		}
	}

	gtr := p.pos()
	if p.peek() == token.GTR || p.peek() == token.SHR {
		p.consumeGreater()
	}
	tp.Greater = gtr
	tp.Span.Hi = gtr + 1
	return tp
}

func (p *parser) parseSingleTemplateParam() ast.Decl {
	start := p.pos()

	// Template template parameter: template <...> class/typename T
	if p.peek() == token.TEMPLATE {
		tmplTok := p.next()
		less := p.expect(token.LSS)
		nestedParams := p.parseTemplateParamsContent(tmplTok, less)

		kwTok := p.pos()
		kind := p.peek()
		if kind == token.CLASS || kind == token.TYPENAME {
			p.next()
		} else {
			p.error(p.cur, "expected 'class' or 'typename' in template template parameter")
		}

		ell := ast.NoTok
		if p.peek() == token.ELLIPSIS {
			ell = p.next()
		}

		var name *ast.Ident
		if p.peek() == token.IDENT {
			name = p.parseIdent()
		}

		assign := ast.NoTok
		var defName ast.Name
		if p.peek() == token.ASSIGN {
			assign = p.next()
			defName = p.parseName()
		}

		hi := kwTok + 1
		if name != nil {
			hi = name.End()
		}
		if defName != nil {
			hi = defName.End()
		}

		return &ast.TemplateTemplateParam{
			Span:     ast.Span{Lo: start, Hi: hi},
			Params:   nestedParams,
			Keyword:  kwTok,
			Kind:     kind,
			Ellipsis: ell,
			Name:     name,
			Assign:   assign,
			Default:  defName,
		}
	}

	// Type parameter: class/typename T = default, or constrained parameter Concept T = default
	if p.peek() == token.CLASS || p.peek() == token.TYPENAME {
		kwTok := p.pos()
		kind := p.peek()
		p.next()

		ell := ast.NoTok
		if p.peek() == token.ELLIPSIS {
			ell = p.next()
		}

		var name *ast.Ident
		if p.peek() == token.IDENT {
			name = p.parseIdent()
		}

		assign := ast.NoTok
		var defType *ast.TypeId
		if p.peek() == token.ASSIGN {
			assign = p.next()
			defType = p.parseTypeId()
		}

		hi := kwTok + 1
		if name != nil {
			hi = name.End()
		}
		if defType != nil {
			hi = defType.End()
		}

		if name != nil {
			p.noteType(name.Text(p.u))
		}
		return &ast.TypeParam{
			Span:     ast.Span{Lo: start, Hi: hi},
			Keyword:  kwTok,
			Kind:     kind,
			Ellipsis: ell,
			Name:     name,
			Assign:   assign,
			Default:  defType,
		}
	}

	// Non-type parameter (e.g. `int N = 0`) or concept-constrained parameter (`Concept T = int`)
	specs := p.parseDeclSpecs()

	// Check if this was a concept constraint: Concept T
	//
	// `T V` is the same shape when T is an earlier parameter of this list
	// -- `template <class T, T V>` is integral_constant -- and then it is
	// a non-type parameter of type T. The name table settles it: a name
	// declared as a type is not a concept, and a name declared as a
	// concept is not a type.
	if len(specs.List) == 1 {
		// `T... Vs` -- the ellipsis read with the type is the parameter's
		// pack expansion, and T is still the type it was declared as.
		typeName := specs.List[0]
		if named, ok := typeName.(*ast.NamedTypeSpec); ok {
			if pn, isPack := named.Name.(*ast.PackName); isPack && p.namesAType(pn.Name) {
				typeName = nil
			}
		}
		if named, ok := typeName.(*ast.NamedTypeSpec); ok && named.Typename == ast.NoTok && !p.namesAType(named.Name) {
			if p.peek() == token.IDENT || p.peek() == token.ELLIPSIS {
				ell := ast.NoTok
				if p.peek() == token.ELLIPSIS {
					ell = p.next()
				}
				var name *ast.Ident
				if p.peek() == token.IDENT {
					name = p.parseIdent()
				}
				assign := ast.NoTok
				var defType *ast.TypeId
				if p.peek() == token.ASSIGN {
					assign = p.next()
					defType = p.parseTypeId()
				}
				hi := named.End()
				if name != nil {
					hi = name.End()
				}
				if defType != nil {
					hi = defType.End()
				}
				return &ast.TypeParam{
					Span:       ast.Span{Lo: start, Hi: hi},
					Constraint: named.Name,
					Ellipsis:   ell,
					Name:       name,
					Assign:     assign,
					Default:    defType,
				}
			}
		}
	}

	// Otherwise, it's a non-type parameter: `specs decl = default`
	decl := p.parseDeclarator()
	p.noteDeclarator(nil, decl)
	assign := ast.NoTok
	var defExpr ast.Expr
	if p.peek() == token.ASSIGN {
		assign = p.next()
		// The default is a constant-expression where top-level '>' ends the list.
		p.inTemplateArgs++
		defExpr = p.parseAssignmentExpr()
		p.inTemplateArgs--
	}

	hi := start
	if decl != nil && decl.End().IsValid() {
		hi = decl.End()
	}
	if defExpr != nil {
		hi = defExpr.End()
	}

	return &ast.ParamDecl{
		Span:    ast.Span{Lo: start, Hi: hi},
		Specs:   specs,
		Decl:    decl,
		Assign:  assign,
		Default: defExpr,
	}
}

// parseConceptDecl parses `concept C = constraint;`
func (p *parser) parseConceptDecl() *ast.ConceptDecl {
	start := p.pos()
	kw := p.expect(token.CONCEPT)
	name := p.parseIdent()
	p.noteConcept(name.Text(p.u))
	assign := p.expect(token.ASSIGN)
	val := p.parseExpr()
	semi := p.expect(token.SEMI)

	return &ast.ConceptDecl{
		Span:    ast.Span{Lo: start, Hi: semi + 1},
		Keyword: kw,
		Name:    name,
		Assign:  assign,
		Value:   val,
		Semi:    semi,
	}
}

// parseRequiresClause parses `requires constraint-logical-or-expr`
func (p *parser) parseRequiresClause() *ast.RequiresClause {
	start := p.pos()
	kw := p.expect(token.REQUIRES)
	expr := p.parseConstraintExpr()
	return &ast.RequiresClause{
		Span:    ast.Span{Lo: start, Hi: expr.End()},
		Keyword: kw,
		X:       expr,
	}
}

// parseConstraintExpr parses a constraint-logical-or-expression (primary expressions joined by && and ||).
func (p *parser) parseConstraintExpr() ast.Expr {
	lhs := p.parseConstraintAnd()
	for p.peek() == token.LOR {
		op := p.next()
		rhs := p.parseConstraintAnd()
		lhs = &ast.BinaryExpr{Span: ast.Span{Lo: lhs.Pos(), Hi: rhs.End()}, X: lhs, OpPos: op, Op: token.LOR, Y: rhs}
	}
	return lhs
}

func (p *parser) parseConstraintAnd() ast.Expr {
	lhs := p.parseConstraintPrimary()
	for p.peek() == token.LAND {
		op := p.next()
		rhs := p.parseConstraintPrimary()
		lhs = &ast.BinaryExpr{Span: ast.Span{Lo: lhs.Pos(), Hi: rhs.End()}, X: lhs, OpPos: op, Op: token.LAND, Y: rhs}
	}
	return lhs
}

func (p *parser) parseConstraintPrimary() ast.Expr {
	switch p.peek() {
	case token.LPAREN:
		// A parenthesized constraint holds any constant expression, a
		// fold over a pack included.
		return p.parseParenOrFoldExpr()
	case token.REQUIRES:
		return p.parseRequiresExpr()
	case token.NOT:
		op := p.next()
		x := p.parseConstraintPrimary()
		return &ast.UnaryExpr{Span: ast.Span{Lo: op, Hi: x.End()}, OpPos: op, Op: token.NOT, X: x}
	case token.TRUE, token.FALSE, token.INT_LIT:
		tok := p.next()
		return &ast.BasicLit{Span: ast.Span{Lo: tok, Hi: tok + 1}, Kind: p.u.Kind(tok)}
	}
	if name := p.parseName(); name != nil {
		if x, isExpr := name.(ast.Expr); isExpr {
			return x
		}
	}
	p.error(p.cur, "expected a constraint")
	tok := p.next()
	return &ast.BadExpr{Span: ast.Span{Lo: tok, Hi: tok + 1}}
}
