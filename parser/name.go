package parser

import (
	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
)

// parseIdent parses a single identifier token.
func (p *parser) parseIdent() *ast.Ident {
	if p.peek() == token.IDENT {
		tok := p.next()
		return &ast.Ident{Span: ast.Span{Lo: tok, Hi: tok + 1}}
	}
	if p.peek() == token.EXPORT {
		tok := p.next()
		return &ast.Ident{Span: ast.Span{Lo: tok, Hi: tok + 1}}
	}
	p.error(p.cur, "expected identifier")
	return &ast.Ident{Span: ast.Span{Lo: p.cur, Hi: p.cur + 1}}
}

// parseName parses an id-expression / declarator-id name:
// plain ident, qualified name (::A::B), template-id (A<T>),
// operator name, destructor name, conversion name, etc.
func (p *parser) parseName() ast.Name {
	start := p.pos()
	global := ast.NoTok

	if p.peek() == token.SCOPE {
		global = p.next()
	}

	var parts []ast.Name
	var colons []ast.Tok

	p.qualifiedPart = global != ast.NoTok
	first := p.parseUnqualifiedName()
	p.qualifiedPart = false
	if first == nil {
		if global != ast.NoTok {
			return &ast.QualifiedName{
				Span:   ast.Span{Lo: start, Hi: p.pos()},
				Global: global,
				Name:   &ast.BadName{Span: ast.Span{Lo: p.pos(), Hi: p.pos() + 1}},
			}
		}
		return nil
	}

	parts = append(parts, first)

	for p.peek() == token.SCOPE {
		col := p.next()
		colons = append(colons, col)
		p.qualifiedPart = true
		nextPart := p.parseUnqualifiedName()
		p.qualifiedPart = false
		if nextPart == nil {
			nextPart = &ast.BadName{Span: ast.Span{Lo: p.pos(), Hi: p.pos() + 1}}
		}
		parts = append(parts, nextPart)
	}

	// If pack expansion ellipsis follows the name
	var result ast.Name
	if len(parts) == 1 && global == ast.NoTok {
		result = parts[0]
	} else {
		finalPart := parts[len(parts)-1]
		qualParts := parts[:len(parts)-1]
		result = &ast.QualifiedName{
			Span:   ast.Span{Lo: start, Hi: finalPart.End()},
			Global: global,
			Qual:   qualParts,
			Colons: colons,
			Name:   finalPart,
		}
	}

	if p.peek() == token.ELLIPSIS {
		ell := p.next()
		result = &ast.PackName{
			Span:     ast.Span{Lo: result.Pos(), Hi: ell + 1},
			Name:     result,
			Ellipsis: ell,
		}
	}

	return result
}

// parseUnqualifiedName parses a single component of a name.
func (p *parser) parseUnqualifiedName() ast.Name {
	tmplTok := ast.NoTok
	if p.peek() == token.TEMPLATE {
		tmplTok = p.next()
	}

	start := p.pos()
	if tmplTok != ast.NoTok {
		start = tmplTok
	}

	var base ast.Name

	switch p.peek() {
	case token.IDENT:
		base = p.parseIdent()

	case token.TILDE:
		tilde := p.next()
		if p.peek() == token.DECLTYPE {
			dt := p.parseDecltypeSpec()
			base = &ast.DestructorName{
				Span:  ast.Span{Lo: tilde, Hi: dt.End()},
				Tilde: tilde,
				Type:  dt,
			}
		} else {
			name := p.parseIdent()
			base = &ast.DestructorName{
				Span:  ast.Span{Lo: tilde, Hi: name.End()},
				Tilde: tilde,
				Name:  name,
			}
		}

	case token.OPERATOR:
		base = p.parseOperatorName()

	default:
		if tmplTok != ast.NoTok {
			p.error(p.cur, "expected name after template")
			return &ast.BadName{Span: ast.Span{Lo: start, Hi: p.pos() + 1}}
		}
		return nil
	}

	// Check if this component has template arguments: A<int, 2>
	//
	// A name the source declared as a template -- a class, a variable
	// template, a function template -- takes them whatever follows: the
	// balanced-bracket heuristic is for names it has not met, and gives
	// up on `_Maximum<(_First < _Second ? _Second : _First)>`.
	if p.peek() == token.LSS {
		// A component after `::` is looked up in the scope before it, not
		// here, so what the current scopes say of the spelling does not
		// decide it: inside a class with a member `is_signed`,
		// `std::is_signed<T>` still names the template.
		qualifiedTemplate := false
		if id, isIdent := base.(*ast.Ident); isIdent && p.qualifiedPart {
			qualifiedTemplate = p.declaredTemplate[id.Text(p.u)]
		}
		if tmplTok.IsValid() || qualifiedTemplate || p.namesATemplate(base) || (p.memberSelector || !p.namesAValue(base)) && p.isTemplateArgs() {
			base = p.parseTemplateName(tmplTok, base)
		}
	}

	return base
}

// parseOperatorName parses operator+, operator[], operator new, conversion functions, etc.
func (p *parser) parseOperatorName() ast.Name {
	start := p.pos()
	opTok := p.expect(token.OPERATOR)

	// Literal operator: operator "" _suffix
	if p.peek() == token.STRING_LIT {
		strTok := p.next()
		var suffix *ast.Ident
		if p.peek() == token.IDENT {
			suffix = p.parseIdent()
		}
		hi := strTok + 1
		if suffix != nil {
			hi = suffix.End()
		}
		return &ast.LiteralOperatorName{
			Span:     ast.Span{Lo: start, Hi: hi},
			Operator: opTok,
			String:   strTok,
			Suffix:   suffix,
		}
	}

	// Conversion function: operator TypeId
	if p.isTypeStart(p.peek()) {
		p.inConversionTypeId = true
		typeId := p.parseTypeId()
		p.inConversionTypeId = false
		return &ast.ConversionName{
			Span:     ast.Span{Lo: start, Hi: typeId.End()},
			Operator: opTok,
			Type:     typeId,
		}
	}

	// Punctuator or keyword operators
	k := p.peek()
	opPos := p.pos()

	switch k {
	case token.NEW, token.DELETE:
		p.next()
		isArray := false
		lb, rb := ast.NoTok, ast.NoTok
		if p.peek() == token.LBRACK && p.peekAt(1) == token.RBRACK {
			lb = p.next()
			rb = p.next()
			isArray = true
		}
		hi := opPos + 1
		if isArray {
			hi = rb + 1
		}
		return &ast.OperatorName{
			Span:     ast.Span{Lo: start, Hi: hi},
			Operator: opTok,
			OpPos:    opPos,
			Op:       k,
			Array:    isArray,
			Lbrack:   lb,
			Rbrack:   rb,
		}

	case token.LBRACK:
		lb := p.next()
		rb := p.expect(token.RBRACK)
		return &ast.OperatorName{
			Span:     ast.Span{Lo: start, Hi: rb + 1},
			Operator: opTok,
			OpPos:    opPos,
			Op:       token.LBRACK,
			Lbrack:   lb,
			Rbrack:   rb,
		}

	case token.LPAREN:
		lp := p.next()
		rp := p.expect(token.RPAREN)
		return &ast.OperatorName{
			Span:     ast.Span{Lo: start, Hi: rp + 1},
			Operator: opTok,
			OpPos:    opPos,
			Op:       token.LPAREN,
			Lparen:   lp,
			Rparen:   rp,
		}

	default:
		// Standard overloaded operators (+, -, *, /, <<, >>, <=>, etc.)
		p.next()
		return &ast.OperatorName{
			Span:     ast.Span{Lo: start, Hi: opPos + 1},
			Operator: opTok,
			OpPos:    opPos,
			Op:       k,
		}
	}
}

// isTemplateArgs determines if '<' introduces a template argument list.
func (p *parser) isTemplateArgs() bool {
	return p.isTemplateArgsAt(0)
}

// isTemplateArgsAt reports whether the `<` at offset n opens a balanced template-argument-list.
func (p *parser) isTemplateArgsAt(n int) bool {
	if p.peekAt(n) != token.LSS {
		return false
	}
	depth := 1
	for i := n + 1; ; i++ {
		k := p.peekAt(i)
		if k == token.EOF || k == token.SEMI || k == token.LBRACE || k == token.RBRACE {
			return false
		}
		if k == token.LSS {
			depth++
		} else if k == token.GTR {
			depth--
			if depth == 0 {
				return true
			}
		} else if k == token.SHR {
			// >> treated as two '>'
			depth -= 2
			if depth <= 0 {
				return true
			}
		}
	}
}

// parseTemplateName parses Name<Args...>
func (p *parser) parseTemplateName(tmplTok ast.Tok, name ast.Name) *ast.TemplateName {
	p.inTemplateArgs++
	defer func() {
		p.inTemplateArgs--
	}()

	start := name.Pos()
	if tmplTok != ast.NoTok {
		start = tmplTok
	}
	less := p.expect(token.LSS)
	var args []ast.Node
	var comma ast.Tok = ast.NoTok

	for !p.atEOF() && p.peek() != token.GTR && p.peek() != token.SHR {
		arg := p.parseTemplateArg()
		if arg != nil {
			args = append(args, arg)
		}
		if p.peek() == token.COMMA {
			comma = p.next()
		} else {
			break
		}
	}

	greater := p.pos()
	if p.peek() == token.GTR || p.peek() == token.SHR {
		p.consumeGreater()
	}

	return &ast.TemplateName{
		Span:     ast.Span{Lo: start, Hi: greater + 1},
		Template: tmplTok,
		Name:     name,
		Less:     less,
		Args:     args,
		Comma:    comma,
		Greater:  greater,
	}
}

// parseTemplateArg parses either a type-id or a non-type expression.
func (p *parser) parseTemplateArg() ast.Node {
	// An identifier the source declared as a value -- a non-type
	// template parameter, a constant, a variable template -- opens an
	// expression: `Fact<N - 1>` is arithmetic on N, not a type followed
	// by junk. One it declared as a type, or one it has not met, is tried
	// as a type-id first, and backed out of when what follows is not what
	// ends an argument.
	if p.peek() == token.IDENT {
		if k := p.kindOf(p.text(p.cur)); k == nameValue || k == nameTemplate {
			return p.parseTemplateArgExpr()
		}
		// `bool_constant<__is_constructible(T, A)>` -- a type trait is an
		// expression, however much its spelling looks like a function type.
		if typeTraits[p.text(p.cur)] && p.peekAt(1) == token.LPAREN {
			return p.parseTemplateArgExpr()
		}
	}
	if p.isTypeStart(p.peek()) {
		cur, half, ndiags, errTok, resyncs := p.cur, p.halfGtr, len(p.diags), p.errTok, p.resyncs
		id := p.parseTypeId()
		switch p.peek() {
		case token.COMMA, token.GTR, token.SHR, token.RPAREN, token.EOF:
			if len(p.diags) == ndiags && !namesMemberTemplateValue(id) {
				return id
			}
		}
		p.cur, p.halfGtr, p.diags, p.errTok, p.resyncs = cur, half, p.diags[:ndiags], errTok, resyncs
	}
	return p.parseTemplateArgExpr()
}

// namesMemberTemplateValue reports whether a type-id is spelled `X::template f<A>` without typename.
func namesMemberTemplateValue(id *ast.TypeId) bool {
	if id == nil || id.Specs == nil {
		return false
	}
	for _, spec := range id.Specs.List {
		nt, ok := spec.(*ast.NamedTypeSpec)
		if !ok || nt.Typename != ast.NoTok {
			continue
		}
		qn, ok := nt.Name.(*ast.QualifiedName)
		if !ok {
			continue
		}
		if tn, ok := qn.Name.(*ast.TemplateName); ok && tn.Template != ast.NoTok {
			return true
		}
	}
	return false
}

// parseTemplateArgExpr parses a non-type argument expression where top-level `>` ends the list.
func (p *parser) parseTemplateArgExpr() ast.Expr {
	p.inTemplateArgs++
	defer func() { p.inTemplateArgs-- }()
	return p.parseAssignmentExpr()
}
