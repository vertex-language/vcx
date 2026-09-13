package parser

import (
	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
)

// parseAttrGroups parses zero or more attribute groups ([[...]], alignas(...), __attribute__, __declspec).
func (p *parser) parseAttrGroups() []*ast.AttrGroup {
	var groups []*ast.AttrGroup
	for {
		g := p.parseAttrGroup()
		if g == nil {
			break
		}
		groups = append(groups, g)
	}
	return groups
}

// parseAttrGroup parses a single attribute group.
func (p *parser) parseAttrGroup() *ast.AttrGroup {
	// Standard C++ attribute: [[ ... ]]
	if p.peek() == token.LBRACK && p.peekAt(1) == token.LBRACK {
		start := p.pos()
		p.next() // [
		p.next() // [

		group := &ast.AttrGroup{
			Span: ast.Span{Lo: start, Hi: start + 2},
		}

		// Check for [[using ns: ...]]
		if p.peek() == token.USING && p.peekAt(2) == token.COLON {
			p.next() // using
			group.Using = p.parseIdent()
			p.expect(token.COLON)
		}

		for !p.atEOF() && !(p.peek() == token.RBRACK && p.peekAt(1) == token.RBRACK) {
			attr := p.parseSingleAttr(0)
			if attr != nil {
				group.Attrs = append(group.Attrs, attr)
			}
			if p.peek() == token.COMMA {
				p.next()
			} else {
				break
			}
		}

		p.expect(token.RBRACK)
		rb2 := p.expect(token.RBRACK)
		group.Span.Hi = rb2 + 1
		return group
	}

	// alignas(...)
	if p.peek() == token.ALIGNAS {
		start := p.next()
		lp := p.expect(token.LPAREN)
		group := &ast.AttrGroup{
			Span: ast.Span{Lo: start, Hi: lp + 1},
		}
		if p.isTypeStart(p.peek()) {
			group.Align = p.parseTypeId()
		} else {
			group.AlignX = p.parseExpr()
		}
		rp := p.expect(token.RPAREN)
		group.Span.Hi = rp + 1
		return group
	}

	// Vendor extensions: __attribute__((...)) or __declspec(...)
	if p.peek() == token.ATTRIBUTE || p.peek() == token.DECLSPEC {
		vendor := p.peek()
		start := p.next()
		lp := p.expect(token.LPAREN)
		// For __attribute__, typically double parens: __attribute__((...))
		doubleParen := vendor == token.ATTRIBUTE && p.peek() == token.LPAREN
		if doubleParen {
			p.next()
		}

		group := &ast.AttrGroup{
			Span: ast.Span{Lo: start, Hi: lp + 1},
		}

		for !p.atEOF() && p.peek() != token.RPAREN {
			attr := p.parseSingleAttr(vendor)
			if attr != nil {
				group.Attrs = append(group.Attrs, attr)
			}
			if p.peek() == token.COMMA {
				p.next()
			} else {
				break
			}
		}

		if doubleParen {
			p.expect(token.RPAREN)
		}
		rp := p.expect(token.RPAREN)
		group.Span.Hi = rp + 1
		return group
	}

	return nil
}

// parseSingleAttr parses one attribute entry inside a group.
func (p *parser) parseSingleAttr(vendor token.Kind) *ast.Attr {
	if p.peek() != token.IDENT && !p.isKeyword(p.peek()) {
		return nil
	}

	// An attribute-token can be an identifier or keyword (e.g. [[likely]], [[msvc::constexpr]]).
	start := p.pos()
	first := p.parseAttrToken()
	var ns *ast.Ident
	name := first

	if p.peek() == token.SCOPE {
		p.next()
		ns = first
		name = p.parseAttrToken()
	}

	attr := &ast.Attr{
		Span:      ast.Span{Lo: start, Hi: name.End()},
		Namespace: ns,
		Name:      name,
		Vendor:    vendor,
		Lparen:    ast.NoTok,
		Rparen:    ast.NoTok,
		Ellipsis:  ast.NoTok,
	}

	// Argument clause: ( balanced-tokens )
	if p.peek() == token.LPAREN {
		attr.Lparen = p.next()
		argStart := p.pos()
		depth := 1
		for !p.atEOF() && depth > 0 {
			switch p.peek() {
			case token.LPAREN:
				depth++
			case token.RPAREN:
				depth--
				if depth == 0 {
					break
				}
			}
			if depth > 0 {
				p.next()
			}
		}
		attr.Args = ast.Span{Lo: argStart, Hi: p.pos()}
		attr.Rparen = p.expect(token.RPAREN)
		attr.Span.Hi = attr.Rparen + 1
	}

	// Pack expansion: attr...
	if p.peek() == token.ELLIPSIS {
		attr.Ellipsis = p.next()
		attr.Span.Hi = attr.Ellipsis + 1
	}

	return attr
}

func (p *parser) isKeyword(k token.Kind) bool {
	return k.IsKeyword()
}

// parseAttrToken reads an attribute's identifier, which may be spelled
// like a keyword.
func (p *parser) parseAttrToken() *ast.Ident {
	if p.peek() == token.IDENT || p.isKeyword(p.peek()) {
		tok := p.next()
		return &ast.Ident{Span: ast.Span{Lo: tok, Hi: tok + 1}}
	}
	return p.parseIdent()
}
