package parser

import (
	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
)

// parseDeclarator parses a declarator (concrete or abstract).
func (p *parser) parseDeclarator() ast.Declarator {
	return p.parseDeclaratorInternal(false)
}

// parseAbstractDeclarator parses an abstract declarator (used in type-ids).
func (p *parser) parseAbstractDeclarator() ast.Declarator {
	return p.parseDeclaratorInternal(true)
}

func (p *parser) parseDeclaratorInternal(abstract bool) ast.Declarator {
	// A Microsoft calling convention or pointer modifier may sit anywhere
	// a declarator begins -- `void (__cdecl *f)(int)`, `int __cdecl g()`
	// -- and says nothing on this target (see msDiscarded).
	p.skipMSDiscarded()

	// Prefix ptr-operators: *, &, &&, C::*
	if p.isPtrOp() {
		return p.parsePointerDeclarator(abstract)
	}

	return p.parseDirectDeclarator(abstract)
}

func (p *parser) isPtrOp() bool {
	switch p.peek() {
	case token.MUL, token.AND, token.LAND:
		return true
	case token.SCOPE, token.IDENT:
		// Check pointer-to-member: C::*
		if p.peek() == token.SCOPE || p.peekAt(1) == token.SCOPE {
			depth := 0
			for i := 0; ; i++ {
				k := p.peekAt(i)
				if k == token.SCOPE && p.peekAt(i+1) == token.MUL {
					return true
				}
				if k == token.EOF || k == token.SEMI || k == token.LPAREN || k == token.LBRACE {
					break
				}
				if depth > 10 {
					break
				}
				depth++
			}
		}
	}
	return false
}

func (p *parser) parsePointerDeclarator(abstract bool) ast.Declarator {
	start := p.pos()
	var class ast.Name

	if p.peek() == token.IDENT || p.peek() == token.SCOPE {
		// Pointer to member: C::*
		class = p.parseName()
	}

	opTok := p.pos()
	kind := p.peek()
	p.next() // consume *, &, &&

	attrs := p.parseAttrGroups()
	var attrList []*ast.Attr
	for _, ag := range attrs {
		attrList = append(attrList, ag.Attrs...)
	}

	var quals []ast.DeclSpec
	for {
		k := p.peek()
		if k == token.CONST || k == token.VOLATILE || k == token.RESTRICT {
			tok := p.next()
			quals = append(quals, &ast.BasicSpec{
				Span: ast.Span{Lo: tok, Hi: tok + 1},
				Kind: k,
			})
		} else if k == token.UNALIGNED || (k == token.IDENT && msDiscarded[p.text(p.cur)]) {
			// `char *__ptr64 p`, `T *__unaligned q`: modifiers on the
			// pointer, read and dropped.
			p.next()
		} else {
			break
		}
	}

	inner := p.parseDeclaratorInternal(abstract)
	hi := opTok + 1
	if inner != nil && inner.End().IsValid() {
		hi = inner.End()
	}

	return &ast.PointerDeclarator{
		Span:  ast.Span{Lo: start, Hi: hi},
		OpPos: opTok,
		Kind:  kind,
		Class: class,
		Attrs: attrList,
		Quals: quals,
		Inner: inner,
	}
}

func (p *parser) parseDirectDeclarator(abstract bool) ast.Declarator {
	start := p.pos()
	var d ast.Declarator

	// Pack declarator: ... D
	if p.peek() == token.ELLIPSIS {
		ell := p.next()
		inner := p.parseDirectDeclarator(abstract)
		hi := ell + 1
		if inner != nil {
			hi = inner.End()
		}
		return &ast.PackDeclarator{
			Span:     ast.Span{Lo: start, Hi: hi},
			Ellipsis: ell,
			Inner:    inner,
		}
	}

	// Parenthesized declarator: ( D )
	// In new-expressions, a new-declarator has no parenthesized form;
	// parentheses enclose the initializer.
	if p.peek() == token.LPAREN && !p.inNewTypeId {
		// Need to disambiguate ( D ) from function parameter list ( params )
		if p.isParenDeclarator(abstract) {
			lp := p.next()
			inner := p.parseDeclaratorInternal(abstract)
			rp := p.expect(token.RPAREN)
			d = &ast.ParenDeclarator{
				Span:   ast.Span{Lo: lp, Hi: rp + 1},
				Lparen: lp,
				Inner:  inner,
				Rparen: rp,
			}
		}
	}

	if d == nil {
		if !abstract && (p.peek() == token.IDENT || p.peek() == token.SCOPE || p.peek() == token.TILDE || p.peek() == token.OPERATOR) {
			name := p.parseName()
			attrs := p.parseAttrGroups()
			var attrList []*ast.Attr
			for _, ag := range attrs {
				attrList = append(attrList, ag.Attrs...)
			}
			d = &ast.NameDeclarator{
				Span:  ast.Span{Lo: start, Hi: p.pos()},
				Name:  name,
				Attrs: attrList,
			}
		} else {
			// Abstract declarator: leaf with nil Name
			attrs := p.parseAttrGroups()
			var attrList []*ast.Attr
			for _, ag := range attrs {
				attrList = append(attrList, ag.Attrs...)
			}
			d = &ast.NameDeclarator{
				Span:  ast.Span{Lo: start, Hi: start},
				Name:  nil,
				Attrs: attrList,
			}
		}
	}

	// Postfix suffixes: [size], (params), : width
	for !p.atEOF() {
		if p.inConversionTypeId {
			// In conversion type-ids, only ptr-operators are consumed.
			break
		}
		if p.peek() == token.LBRACK {
			d = p.parseArraySuffix(d)
		} else if p.peek() == token.LPAREN {
			if p.inNewTypeId {
				// A new-declarator has no parameter list; '(' begins the initializer.
				break
			}
			if !abstract && !p.opensParamList() {
				// A parenthesized initializer, not a parameter list. The
				// `(` is left for parseInitDeclaratorSuffix, which already
				// knows how to read `T v(a, b)`.
				break
			}
			d = p.parseFuncSuffix(d)
		} else if p.peek() == token.COLON && !abstract && !p.inForRangeDecl && !p.hasFuncShape(d) {
			colon := p.next()
			width := p.parseExpr()
			d = &ast.BitfieldDeclarator{
				Span:  ast.Span{Lo: d.Pos(), Hi: width.End()},
				Inner: d,
				Colon: colon,
				Width: width,
			}
		} else {
			break
		}
	}

	return d
}

// opensParamList reports whether '(' begins a parameter-declaration-clause
// rather than a parenthesized initializer (resolving the most vexing parse).
func (p *parser) opensParamList() bool {
	switch k := p.peekAt(1); k {
	case token.RPAREN, token.ELLIPSIS:
		return true
	case token.IDENT:
		if msDiscarded[p.text(p.peekTok(1))] {
			return p.tryParamList()
		}
		// Treat an identifier as a potential type unless known otherwise.
		if !p.identMayBeType(p.peekTok(1)) || cannotFollowType(p.peekAt(2)) {
			return false
		}
		return p.tryParamList()
	case token.SCOPE, token.TYPENAME, token.DECLTYPE, token.UNALIGNED:
		return p.tryParamList()
	case token.LBRACK:
		// An attribute-specifier on the first parameter: `[[maybe_unused]]`.
		return p.peekAt(2) == token.LBRACK
	default:
		if !p.isTypeStart(k) {
			return false
		}
		return p.tryParamList()
	}
}

// tryParamList speculatively parses a parameter-declaration-clause to
// determine if '(' introduces parameters without errors.
func (p *parser) tryParamList() bool {
	cur, half, ndiags, errTok, resyncs := p.cur, p.halfGtr, len(p.diags), p.errTok, p.resyncs
	p.parseFuncSuffix(&ast.NameDeclarator{})
	ok := len(p.diags) == ndiags
	p.cur, p.halfGtr, p.diags, p.errTok, p.resyncs = cur, half, p.diags[:ndiags], errTok, resyncs
	return ok
}

func (p *parser) isParenDeclarator(abstract bool) bool {
	// If followed by ')' and abstract, this is an empty parameter list `()`
	if p.peekAt(1) == token.RPAREN {
		return false
	}
	// If followed by a ptr-op, it is a parenthesized declarator (skipping calling conventions).
	i := 1
	for p.peekAt(i) == token.IDENT && msDiscarded[p.text(p.peekTok(i))] || p.peekAt(i) == token.UNALIGNED {
		i++
	}
	k := p.peekAt(i)
	if k == token.MUL || k == token.AND || k == token.LAND {
		return true
	}
	// Handle parenthesized function names like (max)(args...).
	if k == token.IDENT || k == token.SCOPE {
		j := i
		if p.peekAt(j) == token.SCOPE {
			j++
		}
		for p.peekAt(j) == token.IDENT && p.peekAt(j+1) == token.SCOPE {
			// Pointer to member declarator (e.g. C::*p).
			if p.peekAt(j+2) == token.MUL {
				return true
			}
			j += 2
		}
		if !abstract && p.peekAt(j) == token.IDENT && p.peekAt(j+1) == token.RPAREN && p.peekAt(j+2) == token.LPAREN && p.kindOf(p.text(p.peekTok(j))) != nameType {
			return true
		}
	}
	if !abstract && (k == token.IDENT || k == token.SCOPE) {
		// Could be `(x)` vs `(int x)`
		if p.isTypeStart(k) && p.peekAt(2) != token.RPAREN && p.peekAt(2) != token.LBRACK {
			return false
		}
	}
	return false
}

func (p *parser) parseArraySuffix(inner ast.Declarator) ast.Declarator {
	lb := p.next()
	var size ast.Expr
	if p.peek() != token.RBRACK {
		size = p.parseExpr()
	}
	attrs := p.parseAttrGroups()
	var attrList []*ast.Attr
	for _, ag := range attrs {
		attrList = append(attrList, ag.Attrs...)
	}
	rb := p.expect(token.RBRACK)

	return &ast.ArrayDeclarator{
		Span:   ast.Span{Lo: inner.Pos(), Hi: rb + 1},
		Inner:  inner,
		Lbrack: lb,
		Size:   size,
		Attrs:  attrList,
		Rbrack: rb,
	}
}

func (p *parser) parseFuncSuffix(inner ast.Declarator) ast.Declarator {
	lp := p.next()
	var params []*ast.ParamDecl
	var vararg ast.Tok = ast.NoTok

	for !p.atEOF() && p.peek() != token.RPAREN {
		if p.peek() == token.ELLIPSIS {
			vararg = p.next()
			break
		}
		param := p.parseParamDecl()
		if param != nil {
			params = append(params, param)
		}
		if p.peek() == token.COMMA {
			p.next()
		} else {
			break
		}
	}

	rp := p.expect(token.RPAREN)

	fd := &ast.FuncDeclarator{
		Span:     ast.Span{Lo: inner.Pos(), Hi: rp + 1},
		Inner:    inner,
		Lparen:   lp,
		Params:   params,
		Vararg:   vararg,
		Rparen:   rp,
		RefQual:  ast.NoTok,
		Override: ast.NoTok,
		Final:    ast.NoTok,
	}

	// cv-qualifiers
	for {
		k := p.peek()
		if k == token.CONST || k == token.VOLATILE {
			tok := p.next()
			fd.Quals = append(fd.Quals, &ast.BasicSpec{
				Span: ast.Span{Lo: tok, Hi: tok + 1},
				Kind: k,
			})
			fd.Span.Hi = tok + 1
		} else {
			break
		}
	}

	// ref-qualifiers: & or &&
	if p.peek() == token.AND || p.peek() == token.LAND {
		fd.RefKind = p.peek()
		fd.RefQual = p.next()
		fd.Span.Hi = fd.RefQual + 1
	}

	// noexcept specifier
	if p.peek() == token.NOEXCEPT || p.peek() == token.THROW {
		fd.Noexcept = p.parseNoexceptSpec()
		fd.Span.Hi = fd.Noexcept.End()
	}

	// attributes
	attrs := p.parseAttrGroups()
	for _, ag := range attrs {
		fd.Attrs = append(fd.Attrs, ag.Attrs...)
	}

	// Trailing return type: -> TypeId
	if p.peek() == token.ARROW {
		arr := p.next()
		retType := p.parseTypeId()
		fd.Trailing = &ast.TrailingReturn{
			Span:  ast.Span{Lo: arr, Hi: retType.End()},
			Arrow: arr,
			Type:  retType,
		}
		fd.Span.Hi = retType.End()
	}

	// virt-specifiers: override, final
	for p.peek() == token.IDENT {
		txt := p.text(p.cur)
		if txt == "override" && fd.Override == ast.NoTok {
			fd.Override = p.next()
			fd.Span.Hi = fd.Override + 1
		} else if txt == "final" && fd.Final == ast.NoTok {
			fd.Final = p.next()
			fd.Span.Hi = fd.Final + 1
		} else {
			break
		}
	}

	// trailing requires clause
	if p.peek() == token.REQUIRES {
		fd.Requires = p.parseRequiresClause()
		fd.Span.Hi = fd.Requires.End()
	}

	return fd
}

// parseNoexceptSpec parses noexcept, noexcept(expr), or throw()
func (p *parser) parseNoexceptSpec() *ast.NoexceptSpec {
	start := p.pos()
	isThrow := p.peek() == token.THROW
	kw := p.next()

	spec := &ast.NoexceptSpec{
		Span:    ast.Span{Lo: start, Hi: kw + 1},
		Keyword: kw,
		Throw:   isThrow,
		Lparen:  ast.NoTok,
		Rparen:  ast.NoTok,
	}

	if p.peek() == token.LPAREN {
		spec.Lparen = p.next()
		if p.peek() != token.RPAREN {
			spec.Cond = p.parseExpr()
		}
		spec.Rparen = p.expect(token.RPAREN)
		spec.Span.Hi = spec.Rparen + 1
	}

	return spec
}

// parseParamDecl parses one function/template parameter: attrs specs declarator = default
func (p *parser) parseParamDecl() *ast.ParamDecl {
	start := p.pos()
	attrs := p.parseAttrGroups()
	var attrList []*ast.Attr
	for _, ag := range attrs {
		attrList = append(attrList, ag.Attrs...)
	}

	specs := p.parseDeclSpecs()
	decl := p.parseDeclarator()

	param := &ast.ParamDecl{
		Span:   ast.Span{Lo: start, Hi: p.pos()},
		Attrs:  attrList,
		Specs:  specs,
		Decl:   decl,
		Assign: ast.NoTok,
	}

	// The default is an initializer-clause (an assignment-expression or braced-init-list).
	if p.peek() == token.ASSIGN {
		param.Assign = p.next()
		if p.peek() == token.LBRACE {
			param.Default = p.parseInitList()
		} else {
			param.Default = p.parseAssignmentExpr()
		}
		param.Span.Hi = param.Default.End()
	}

	return param
}

// parseTypeId parses a type-id (e.g. for sizeof, cast, template arg).
func (p *parser) parseTypeId() *ast.TypeId {
	start := p.pos()
	specs := p.parseDeclSpecs()
	decl := p.parseAbstractDeclarator()

	hi := start
	if specs != nil && specs.End().IsValid() {
		hi = specs.End()
	}
	if decl != nil && decl.End().IsValid() {
		hi = decl.End()
	}

	return &ast.TypeId{
		Span:  ast.Span{Lo: start, Hi: hi},
		Specs: specs,
		Decl:  decl,
	}
}
