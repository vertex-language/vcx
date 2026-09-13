package parser

import (
	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
)

// parseStmt parses a statement.
func (p *parser) parseStmt() ast.Stmt {
	p.skipPragmaLines()
	if p.peek() == token.RBRACE || p.atEOF() {
		return nil
	}

	// Attribute statement: [[...]] stmt
	if p.peek() == token.LBRACK && p.peekAt(1) == token.LBRACK {
		attrs := p.parseAttrGroups()
		stmt := p.parseStmt()
		return &ast.AttrStmt{
			Span:  ast.Span{Lo: attrs[0].Pos(), Hi: stmt.End()},
			Attrs: attrs,
			Stmt:  stmt,
		}
	}

	start := p.pos()

	switch p.peek() {
	case token.SEMI:
		tok := p.next()
		return &ast.EmptyStmt{
			Span: ast.Span{Lo: tok, Hi: tok + 1},
			Semi: tok,
		}

	case token.LBRACE:
		return p.parseCompoundStmt()

	case token.CASE, token.DEFAULT:
		return p.parseCaseOrDefaultStmt()

	case token.IF:
		return p.parseIfStmt()

	case token.SWITCH:
		return p.parseSwitchStmt()

	case token.WHILE:
		return p.parseWhileStmt()

	case token.DO:
		return p.parseDoStmt()

	case token.FOR:
		return p.parseForStmt()

	case token.BREAK:
		kw := p.next()
		semi := p.expect(token.SEMI)
		return &ast.BreakStmt{
			Span:    ast.Span{Lo: kw, Hi: semi + 1},
			Keyword: kw,
			Semi:    semi,
		}

	case token.CONTINUE:
		kw := p.next()
		semi := p.expect(token.SEMI)
		return &ast.ContinueStmt{
			Span:    ast.Span{Lo: kw, Hi: semi + 1},
			Keyword: kw,
			Semi:    semi,
		}

	case token.RETURN:
		kw := p.next()
		var x ast.Expr
		if p.peek() != token.SEMI {
			if p.peek() == token.LBRACE {
				x = p.parseInitList()
			} else {
				x = p.parseExpr()
			}
		}
		semi := p.expect(token.SEMI)
		return &ast.ReturnStmt{
			Span:    ast.Span{Lo: kw, Hi: semi + 1},
			Keyword: kw,
			X:       x,
			Semi:    semi,
		}

	case token.CO_RETURN:
		kw := p.next()
		var x ast.Expr
		if p.peek() != token.SEMI {
			x = p.parseExpr()
		}
		semi := p.expect(token.SEMI)
		return &ast.CoReturnStmt{
			Span:    ast.Span{Lo: kw, Hi: semi + 1},
			Keyword: kw,
			X:       x,
			Semi:    semi,
		}

	case token.GOTO:
		kw := p.next()
		lbl := p.parseIdent()
		semi := p.expect(token.SEMI)
		return &ast.GotoStmt{
			Span:    ast.Span{Lo: kw, Hi: semi + 1},
			Keyword: kw,
			Label:   lbl,
			Semi:    semi,
		}

	case token.TRY:
		return p.parseTryStmt()

	case token.ASM:
		return p.parseAsmStmt()
	}

	// Labeled statement: identifier : stmt
	if p.peek() == token.IDENT && p.peekAt(1) == token.COLON {
		name := p.parseIdent()
		colon := p.next()
		stmt := p.parseStmt()
		return &ast.LabeledStmt{
			Span:  ast.Span{Lo: name.Pos(), Hi: stmt.End()},
			Name:  name,
			Colon: colon,
			Stmt:  stmt,
		}
	}

	// Declaration statement vs expression statement: a statement that can
	// be a declaration is treated as one; otherwise fall back to expression statement.
	if p.isDeclStart() {
		cur, half, ndiags, errTok, resyncs := p.cur, p.halfGtr, len(p.diags), p.errTok, p.resyncs
		decl := p.parseDecl()
		if len(p.diags) == ndiags && declaresSomething(decl) {
			return &ast.DeclStmt{
				Span: ast.Span{Lo: decl.Pos(), Hi: decl.End()},
				Decl: decl,
			}
		}
		if !startsWithKeyword(p, cur) {
			p.cur, p.halfGtr, p.diags, p.errTok, p.resyncs = cur, half, p.diags[:ndiags], errTok, resyncs
		} else {
			// A keyword opened it -- `int`, `struct`, `using` -- so it
			// was never an expression; the diagnostics stand.
			return &ast.DeclStmt{
				Span: ast.Span{Lo: decl.Pos(), Hi: decl.End()},
				Decl: decl,
			}
		}
	}

	// Expression statement: expr ;
	x := p.parseExpr()
	semi := p.expect(token.SEMI)
	return &ast.ExprStmt{
		Span: ast.Span{Lo: start, Hi: semi + 1},
		X:    x,
		Semi: semi,
	}
}

// parseCompoundStmt parses `{ ... }`
func (p *parser) parseCompoundStmt() *ast.CompoundStmt {
	lb := p.expect(token.LBRACE)
	var stmts []ast.Stmt

	p.pushNames()
	defer p.popNames()
	for !p.atEOF() && p.peek() != token.RBRACE {
		prev := p.pos()
		stmt := p.parseStmt()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
		if p.pos() == prev {
			p.advanceTo(token.SEMI, token.RBRACE)
			if p.peek() == token.SEMI {
				p.next()
			}
		}
	}

	rb := p.expect(token.RBRACE)
	return &ast.CompoundStmt{
		Span:   ast.Span{Lo: lb, Hi: rb + 1},
		Lbrace: lb,
		Stmts:  stmts,
		Rbrace: rb,
	}
}

func (p *parser) parseCaseOrDefaultStmt() *ast.LabeledStmt {
	start := p.pos()
	k := p.peek()
	kw := p.next()

	var val, high ast.Expr
	ell := ast.NoTok

	if k == token.CASE {
		val = p.parseAssignmentExpr()
		// GNU extension: case lo ... hi:
		if p.peek() == token.ELLIPSIS {
			ell = p.next()
			high = p.parseAssignmentExpr()
		}
	}

	colon := p.expect(token.COLON)
	stmt := p.parseStmt()

	return &ast.LabeledStmt{
		Span:     ast.Span{Lo: start, Hi: stmt.End()},
		Keyword:  kw,
		Kind:     k,
		Value:    val,
		Ellipsis: ell,
		High:     high,
		Colon:    colon,
		Stmt:     stmt,
	}
}

func (p *parser) parseIfStmt() *ast.IfStmt {
	start := p.pos()
	kw := p.expect(token.IF)

	constexprTok := ast.NoTok
	constevalTok := ast.NoTok
	notTok := ast.NoTok

	if p.peek() == token.CONSTEXPR {
		constexprTok = p.next()
	} else if p.peek() == token.CONSTEVAL {
		constevalTok = p.next()
	} else if p.peek() == token.NOT && p.peekAt(1) == token.CONSTEVAL {
		notTok = p.next()
		constevalTok = p.next()
	}

	var initStmt ast.Stmt
	var cond ast.Node
	lp, rp := ast.NoTok, ast.NoTok

	// `if consteval` has no condition or parentheses, only compound statements.
	if constevalTok != ast.NoTok {
		thenStmt := ast.Stmt(p.parseCompoundStmt())

		elseTok := ast.NoTok
		var elseStmt ast.Stmt
		if p.peek() == token.ELSE {
			elseTok = p.next()
			elseStmt = p.parseStmt()
		}

		hi := thenStmt.End()
		if elseStmt != nil {
			hi = elseStmt.End()
		}
		return &ast.IfStmt{
			Span:      ast.Span{Lo: start, Hi: hi},
			Keyword:   kw,
			Consteval: constevalTok,
			Not:       notTok,
			Lparen:    ast.NoTok,
			Rparen:    ast.NoTok,
			Then:      thenStmt,
			ElsePos:   elseTok,
			Else:      elseStmt,
		}
	}

	lp = p.expect(token.LPAREN)
	initStmt, cond = p.parseConditionWithOptionalInit()
	rp = p.expect(token.RPAREN)
	thenStmt := p.parseStmt()

	elseTok := ast.NoTok
	var elseStmt ast.Stmt

	if p.peek() == token.ELSE {
		elseTok = p.next()
		elseStmt = p.parseStmt()
	}

	hi := thenStmt.End()
	if elseStmt != nil {
		hi = elseStmt.End()
	}

	return &ast.IfStmt{
		Span:      ast.Span{Lo: start, Hi: hi},
		Keyword:   kw,
		Constexpr: constexprTok,
		Consteval: constevalTok,
		Not:       notTok,
		Lparen:    lp,
		Init:      initStmt,
		Cond:      cond,
		Rparen:    rp,
		Then:      thenStmt,
		ElsePos:   elseTok,
		Else:      elseStmt,
	}
}

func (p *parser) parseConditionWithOptionalInit() (ast.Stmt, ast.Node) {
	// Look ahead to see if there's a ';' inside balanced parens -- and
	// balanced braces and brackets, since `T{}(a, b)` and a lambda can
	// both stand in an init-statement.
	hasSemi := false
	depth := 1
	for i := 0; ; i++ {
		k := p.peekAt(i)
		if k == token.EOF {
			break
		}
		switch k {
		case token.LPAREN, token.LBRACE, token.LBRACK:
			depth++
		case token.RPAREN, token.RBRACE, token.RBRACK:
			depth--
		}
		if depth == 0 {
			break
		}
		if k == token.SEMI && depth == 1 {
			hasSemi = true
			break
		}
	}

	var initStmt ast.Stmt
	if hasSemi {
		if p.isDeclStart() {
			decl := p.parseDecl()
			initStmt = &ast.DeclStmt{
				Span: ast.Span{Lo: decl.Pos(), Hi: decl.End()},
				Decl: decl,
			}
		} else {
			x := p.parseExpr()
			semi := p.expect(token.SEMI)
			initStmt = &ast.ExprStmt{
				Span: ast.Span{Lo: x.Pos(), Hi: semi + 1},
				X:    x,
				Semi: semi,
			}
		}
	}

	// Now parse condition: either `T var = expr` or `expr`.
	// A condition that can be a declaration is one; try declaration first.
	var cond ast.Node
	if p.isDeclStart() {
		cur, half, ndiags, errTok, resyncs := p.cur, p.halfGtr, len(p.diags), p.errTok, p.resyncs
		specs := p.parseDeclSpecs()
		decl := p.parseDeclarator()
		if len(p.diags) == ndiags && p.peek() == token.ASSIGN && decl != nil && decl.DeclName() != nil {
			assign := p.next()
			val := p.parseAssignmentExpr()
			cond = &ast.SimpleDecl{
				Span:  ast.Span{Lo: specs.Pos(), Hi: val.End()},
				Specs: specs,
				Inits: []*ast.InitDeclarator{
					{
						Span:   ast.Span{Lo: decl.Pos(), Hi: val.End()},
						Decl:   decl,
						Assign: assign,
						Value:  val,
					},
				},
			}
			return initStmt, cond
		}
		p.cur, p.halfGtr, p.diags, p.errTok, p.resyncs = cur, half, p.diags[:ndiags], errTok, resyncs
	}
	cond = p.parseExpr()

	return initStmt, cond
}

func (p *parser) parseSwitchStmt() *ast.SwitchStmt {
	start := p.pos()
	kw := p.expect(token.SWITCH)
	lp := p.expect(token.LPAREN)
	initStmt, cond := p.parseConditionWithOptionalInit()
	rp := p.expect(token.RPAREN)
	body := p.parseStmt()

	return &ast.SwitchStmt{
		Span:    ast.Span{Lo: start, Hi: body.End()},
		Keyword: kw,
		Lparen:  lp,
		Init:    initStmt,
		Cond:    cond,
		Rparen:  rp,
		Body:    body,
	}
}

func (p *parser) parseWhileStmt() *ast.WhileStmt {
	start := p.pos()
	kw := p.expect(token.WHILE)
	lp := p.expect(token.LPAREN)
	_, cond := p.parseConditionWithOptionalInit()
	rp := p.expect(token.RPAREN)
	body := p.parseStmt()

	return &ast.WhileStmt{
		Span:    ast.Span{Lo: start, Hi: body.End()},
		Keyword: kw,
		Lparen:  lp,
		Cond:    cond,
		Rparen:  rp,
		Body:    body,
	}
}

func (p *parser) parseDoStmt() *ast.DoStmt {
	start := p.pos()
	kw := p.expect(token.DO)
	body := p.parseStmt()
	whileTok := p.expect(token.WHILE)
	lp := p.expect(token.LPAREN)
	cond := p.parseExpr()
	rp := p.expect(token.RPAREN)
	semi := p.expect(token.SEMI)

	return &ast.DoStmt{
		Span:     ast.Span{Lo: start, Hi: semi + 1},
		Keyword:  kw,
		Body:     body,
		WhilePos: whileTok,
		Lparen:   lp,
		Cond:     cond,
		Rparen:   rp,
		Semi:     semi,
	}
}

func (p *parser) parseForStmt() ast.Stmt {
	start := p.pos()
	kw := p.expect(token.FOR)
	lp := p.expect(token.LPAREN)

	// Check if this is a range-for or standard for
	// Range-for: `for ( init-statement_opt decl : range )`
	if p.isRangeFor() {
		return p.parseRangeForStmt(start, kw, lp)
	}

	// Standard three-clause for: `for ( init cond ; post ) body`
	var initStmt ast.Stmt
	if p.peek() == token.SEMI {
		semi := p.next()
		initStmt = &ast.EmptyStmt{
			Span: ast.Span{Lo: semi, Hi: semi + 1},
			Semi: semi,
		}
	} else if p.isDeclStart() {
		decl := p.parseDecl()
		initStmt = &ast.DeclStmt{
			Span: ast.Span{Lo: decl.Pos(), Hi: decl.End()},
			Decl: decl,
		}
	} else {
		x := p.parseExpr()
		semi := p.expect(token.SEMI)
		initStmt = &ast.ExprStmt{
			Span: ast.Span{Lo: x.Pos(), Hi: semi + 1},
			X:    x,
			Semi: semi,
		}
	}

	var cond ast.Node
	if p.peek() != token.SEMI {
		cond = p.parseExpr()
	}
	semi2 := p.expect(token.SEMI)

	var post ast.Expr
	if p.peek() != token.RPAREN {
		post = p.parseExpr()
	}
	rp := p.expect(token.RPAREN)
	body := p.parseStmt()

	return &ast.ForStmt{
		Span:    ast.Span{Lo: start, Hi: body.End()},
		Keyword: kw,
		Lparen:  lp,
		Init:    initStmt,
		Cond:    cond,
		Semi:    semi2,
		Post:    post,
		Rparen:  rp,
		Body:    body,
	}
}

// isRangeFor reports whether the `for (` opens a range-based loop (looking for a top-level colon).
func (p *parser) isRangeFor() bool {
	depth := 1
	semis := 0
	questions := 0
	for i := 0; ; i++ {
		switch k := p.peekAt(i); k {
		case token.EOF, token.LBRACE, token.RBRACE:
			return false

		case token.SEMI:
			if depth == 1 {
				semis++
				if semis > 1 {
					return false
				}
			}

		case token.LPAREN, token.LBRACK:
			depth++

		case token.RPAREN, token.RBRACK:
			depth--
			if depth == 0 {
				return false
			}

		case token.QUESTION:
			if depth == 1 {
				questions++
			}

		case token.COLON:
			if depth != 1 || p.peekAt(i+1) == token.COLON {
				continue
			}
			if questions > 0 {
				questions--
				continue
			}
			return true
		}
	}
}

func (p *parser) parseRangeForStmt(start, kw, lp ast.Tok) *ast.RangeForStmt {
	// In C++20, range-for can have an optional init-statement: `for (init; decl : range)`
	var initStmt ast.Stmt
	// Check if there's a ';' before ':'
	hasInit := false
	depth := 1
	for i := 0; ; i++ {
		k := p.peekAt(i)
		if k == token.SEMI && depth == 1 {
			hasInit = true
			break
		}
		if k == token.COLON && depth == 1 {
			break
		}
		if k == token.LPAREN {
			depth++
		} else if k == token.RPAREN {
			depth--
			if depth == 0 {
				break
			}
		}
	}

	if hasInit {
		if p.isDeclStart() {
			d := p.parseDecl()
			initStmt = &ast.DeclStmt{Span: ast.Span{Lo: d.Pos(), Hi: d.End()}, Decl: d}
		} else {
			x := p.parseExpr()
			semi := p.expect(token.SEMI)
			initStmt = &ast.ExprStmt{Span: ast.Span{Lo: x.Pos(), Hi: semi + 1}, X: x, Semi: semi}
		}
	}

	p.inForRangeDecl = true
	decl := p.parseDeclNoSemi()
	p.inForRangeDecl = false

	colon := p.expect(token.COLON)
	rng := p.parseExpr()
	rp := p.expect(token.RPAREN)
	body := p.parseStmt()

	return &ast.RangeForStmt{
		Span:    ast.Span{Lo: start, Hi: body.End()},
		Keyword: kw,
		Lparen:  lp,
		Init:    initStmt,
		Decl:    decl,
		Colon:   colon,
		Range:   rng,
		Rparen:  rp,
		Body:    body,
	}
}

func (p *parser) parseTryStmt() *ast.TryStmt {
	start := p.pos()
	kw := p.expect(token.TRY)
	body := p.parseCompoundStmt()

	var handlers []*ast.CatchClause
	for p.peek() == token.CATCH {
		handler := p.parseCatchClause()
		if handler != nil {
			handlers = append(handlers, handler)
		}
	}

	hi := body.End()
	if len(handlers) > 0 {
		hi = handlers[len(handlers)-1].End()
	}

	return &ast.TryStmt{
		Span:     ast.Span{Lo: start, Hi: hi},
		Keyword:  kw,
		Body:     body,
		Handlers: handlers,
	}
}

func (p *parser) parseCatchClause() *ast.CatchClause {
	start := p.pos()
	kw := p.expect(token.CATCH)
	lp := p.expect(token.LPAREN)

	var param *ast.ParamDecl
	ell := ast.NoTok

	if p.peek() == token.ELLIPSIS {
		ell = p.next()
	} else {
		param = p.parseParamDecl()
	}

	rp := p.expect(token.RPAREN)
	body := p.parseCompoundStmt()

	return &ast.CatchClause{
		Span:     ast.Span{Lo: start, Hi: body.End()},
		Keyword:  kw,
		Lparen:   lp,
		Param:    param,
		Ellipsis: ell,
		Rparen:   rp,
		Body:     body,
	}
}

func (p *parser) parseAsmStmt() *ast.AsmStmt {
	start := p.pos()
	kw := p.expect(token.ASM)

	var quals []ast.DeclSpec
	for {
		k := p.peek()
		if k == token.VOLATILE || k == token.INLINE || (k == token.GOTO) {
			tok := p.next()
			quals = append(quals, &ast.BasicSpec{
				Span: ast.Span{Lo: tok, Hi: tok + 1},
				Kind: k,
			})
		} else {
			break
		}
	}

	lp := p.expect(token.LPAREN)
	bodyStart := p.pos()
	depth := 1
	for !p.atEOF() && depth > 0 {
		if p.peek() == token.LPAREN {
			depth++
		} else if p.peek() == token.RPAREN {
			depth--
			if depth == 0 {
				break
			}
		}
		p.next()
	}
	bodySpan := ast.Span{Lo: bodyStart, Hi: p.pos()}
	rp := p.expect(token.RPAREN)
	semi := p.expect(token.SEMI)

	return &ast.AsmStmt{
		Span:    ast.Span{Lo: start, Hi: semi + 1},
		Keyword: kw,
		Quals:   quals,
		Lparen:  lp,
		Body:    bodySpan,
		Rparen:  rp,
		Semi:    semi,
	}
}

// declaresSomething reports whether a declaration parsed at block scope declares anything.
func declaresSomething(d ast.Decl) bool {
	sd, isSimple := d.(*ast.SimpleDecl)
	if !isSimple {
		return d != nil
	}
	if len(sd.Inits) == 0 {
		return true // a class or enum definition, or `T;` -- sema's to judge
	}
	if sd.Specs == nil || len(sd.Specs.List) == 0 {
		// No decl-specifier at all (e.g. ::operator delete(p);).
		return false
	}
	for _, init := range sd.Inits {
		if init.Decl == nil || init.Decl.DeclName() == nil {
			return false
		}
	}
	return true
}

// startsWithKeyword reports whether the statement at tok opened with a
// keyword rather than an identifier.
func startsWithKeyword(p *parser, tok ast.Tok) bool {
	k := p.u.Kind(tok)
	return k != token.IDENT && k != token.SCOPE
}
