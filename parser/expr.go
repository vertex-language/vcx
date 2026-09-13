package parser

import (
	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
)

// parseExpr parses a full expression including the comma operator.
func (p *parser) parseExpr() ast.Expr {
	return p.parseBinaryExpr(1)
}

// parseAssignmentExpr parses an expression with precedence above comma.
func (p *parser) parseAssignmentExpr() ast.Expr {
	return p.parseBinaryExpr(2)
}

// parseBinaryExpr implements operator-precedence climbing.
func (p *parser) parseBinaryExpr(minPrec int) ast.Expr {
	lhs := p.parseCastOrUnaryExpr()
	if lhs == nil {
		return &ast.BadExpr{Span: ast.Span{Lo: p.pos(), Hi: p.pos() + 1}}
	}
	return p.parseBinaryExprRemainder(lhs, minPrec)
}

func (p *parser) parseBinaryExprRemainder(lhs ast.Expr, minPrec int) ast.Expr {
	for {
		k := p.peek()

		// When inside template arguments, '>' and '>>' terminate the argument list
		if (k == token.GTR || k == token.SHR) && p.inTemplateArgs > 0 {
			break
		}

		// Assignment operators (right-associative, below conditional)
		if p.isAssignOp(k) && minPrec <= 2 {
			opTok := p.next()
			rhs := p.parseAssignmentExpr()
			lhs = &ast.AssignExpr{
				Span:  ast.Span{Lo: lhs.Pos(), Hi: rhs.End()},
				Lhs:   lhs,
				OpPos: opTok,
				Op:    k,
				Rhs:   rhs,
			}
			continue
		}

		// Conditional operator: cond ? then : else
		if k == token.QUESTION && minPrec <= 2 {
			qTok := p.next()
			thenExpr := p.parseExpr()
			colonTok := p.expect(token.COLON)
			elseExpr := p.parseAssignmentExpr()
			lhs = &ast.CondExpr{
				Span:     ast.Span{Lo: lhs.Pos(), Hi: elseExpr.End()},
				Cond:     lhs,
				Question: qTok,
				Then:     thenExpr,
				Colon:    colonTok,
				Else:     elseExpr,
			}
			continue
		}

		prec := k.Precedence()
		if prec < minPrec {
			break
		}

		opTok := p.next()
		// Left-associative binary operator
		rhs := p.parseBinaryExpr(prec + 1)
		lhs = &ast.BinaryExpr{
			Span:  ast.Span{Lo: lhs.Pos(), Hi: rhs.End()},
			X:     lhs,
			OpPos: opTok,
			Op:    k,
			Y:     rhs,
		}
	}

	// Check for trailing pack expansion: expr...
	// Do not consume if followed by an expression (e.g. GNU case lo ... hi)
	if p.peek() == token.ELLIPSIS && !p.isExprStart(p.peekAt(1)) {
		ell := p.next()
		lhs = &ast.PackExpansion{
			Span:     ast.Span{Lo: lhs.Pos(), Hi: ell + 1},
			X:        lhs,
			Ellipsis: ell,
		}
	}

	return lhs
}

func (p *parser) isAssignOp(k token.Kind) bool {
	switch k {
	case token.ASSIGN, token.ADD_ASSIGN, token.SUB_ASSIGN, token.MUL_ASSIGN,
		token.QUO_ASSIGN, token.REM_ASSIGN, token.SHL_ASSIGN, token.SHR_ASSIGN,
		token.AND_ASSIGN, token.XOR_ASSIGN, token.OR_ASSIGN:
		return true
	}
	return false
}

// parseCastOrUnaryExpr parses unary operators, casts, new/delete, etc.
func (p *parser) parseCastOrUnaryExpr() ast.Expr {
	k := p.peek()

	// Prefix operators: ++, --, +, -, !, ~, *, &
	switch k {
	case token.INC, token.DEC, token.ADD, token.SUB, token.NOT, token.TILDE, token.MUL, token.AND:
		opTok := p.next()
		operand := p.parseCastOrUnaryExpr()
		return &ast.UnaryExpr{
			Span:  ast.Span{Lo: opTok, Hi: operand.End()},
			OpPos: opTok,
			Op:    k,
			X:     operand,
		}

	case token.SIZEOF:
		return p.parseSizeofExpr()

	case token.ALIGNOF:
		start := p.next()
		lp := p.expect(token.LPAREN)
		typeId := p.parseTypeId()
		rp := p.expect(token.RPAREN)
		return &ast.AlignofExpr{
			Span:    ast.Span{Lo: start, Hi: rp + 1},
			Keyword: start,
			Lparen:  lp,
			Type:    typeId,
			Rparen:  rp,
		}

	case token.NOEXCEPT:
		start := p.next()
		lp := p.expect(token.LPAREN)
		x := p.parseExpr()
		rp := p.expect(token.RPAREN)
		return &ast.NoexceptExpr{
			Span:    ast.Span{Lo: start, Hi: rp + 1},
			Keyword: start,
			Lparen:  lp,
			X:       x,
			Rparen:  rp,
		}

	case token.NEW:
		return p.parseNewExpr(ast.NoTok)

	case token.DELETE:
		return p.parseDeleteExpr(ast.NoTok)

	case token.THROW:
		start := p.next()
		var x ast.Expr
		// Bare throw (rethrow) if followed by delimiter
		if !p.isStmtTerminator(p.peek()) && p.peek() != token.RPAREN && p.peek() != token.COLON && p.peek() != token.COMMA {
			x = p.parseAssignmentExpr()
		}
		hi := start + 1
		if x != nil {
			hi = x.End()
		}
		return &ast.ThrowExpr{
			Span:    ast.Span{Lo: start, Hi: hi},
			Keyword: start,
			X:       x,
		}

	case token.CO_AWAIT:
		start := p.next()
		x := p.parseCastOrUnaryExpr()
		return &ast.CoAwaitExpr{
			Span:    ast.Span{Lo: start, Hi: x.End()},
			Keyword: start,
			X:       x,
		}

	case token.CO_YIELD:
		start := p.next()
		x := p.parseAssignmentExpr()
		return &ast.CoYieldExpr{
			Span:    ast.Span{Lo: start, Hi: x.End()},
			Keyword: start,
			X:       x,
		}

	case token.SCOPE:
		// Leading :: could be ::new or ::delete
		if p.peekAt(1) == token.NEW {
			global := p.next()
			return p.parseNewExpr(global)
		}
		if p.peekAt(1) == token.DELETE {
			global := p.next()
			return p.parseDeleteExpr(global)
		}
	}

	// C-style cast vs parenthesized expression: ( Type ) x
	if p.peek() == token.LPAREN {
		if p.isCastExpr() {
			lp := p.next()
			typeId := p.parseTypeId()
			rp := p.expect(token.RPAREN)
			x := p.parseCastOrUnaryExpr()
			return &ast.CastExpr{
				Span:   ast.Span{Lo: lp, Hi: x.End()},
				Lparen: lp,
				Type:   typeId,
				Rparen: rp,
				X:      x,
			}
		}
	}

	return p.parsePostfixExpr()
}

func (p *parser) isCastExpr() bool {
	// ( Type ) operand
	if p.peek() != token.LPAREN {
		return false
	}
	// Look inside the parentheses
	if !p.isTypeStart(p.peekAt(1)) {
		return false
	}
	// A name the source declared as something other than a type cannot
	// begin a cast type.
	if p.peekAt(1) == token.IDENT && p.peekAt(2) != token.SCOPE && !p.identMayBeType(p.peekTok(1)) {
		return false
	}

	// Scan to matching ')'
	depth := 1
	angles := 0
	brackets := 0
	for i := 2; ; i++ {
		k := p.peekAt(i)
		if k == token.EOF || k == token.SEMI || k == token.LBRACE || k == token.RBRACE {
			return false
		}
		if k == token.LPAREN {
			depth++
			continue
		}
		if k == token.LBRACK {
			brackets++
			continue
		}
		if k == token.RBRACK {
			brackets--
			continue
		}
		if k == token.INT_LIT && brackets == 0 {
			return false
		}
		if k == token.LSS {
			angles++
			continue
		}
		if k == token.GTR {
			angles--
			continue
		}
		if k == token.SHR {
			angles -= 2
			continue
		}
		if k != token.RPAREN {
			// Disambiguate cast expression from parenthesized expression
			// by checking whether the contents can form a type-id.
			if !typeIdToken(k) {
				return false
			}
			continue
		}
		depth--
		if depth == 0 {
			// An unbalanced `<` means the parentheses hold a comparison and
			// not a template-id: `(a < b)` is one `<` and no `>`, and
			// reading it as a type made `(a < b) * 2` a dereference of 2.
			if angles != 0 {
				return false
			}
			// The token after ')' must be able to start an expression, or
			// there is nothing for the cast to apply to.
			return p.isExprStart(p.peekAt(i + 1))
		}
	}
}

// typeIdToken reports whether a token may appear inside a type-id.
func typeIdToken(k token.Kind) bool {
	switch k {
	case token.IDENT, token.SCOPE, token.COMMA, token.ELLIPSIS,
		token.MUL, token.AND, token.LAND,
		token.LBRACK, token.RBRACK, token.LSS, token.GTR, token.SHR,
		token.INT_LIT,
		token.CONST, token.VOLATILE, token.RESTRICT,
		token.TYPENAME, token.DECLTYPE, token.AUTO,
		token.CLASS, token.STRUCT, token.UNION, token.ENUM,
		token.VOID, token.BOOL, token.CHAR, token.CHAR8_T, token.CHAR16_T,
		token.CHAR32_T, token.WCHAR_T, token.INT, token.SHORT, token.LONG,
		token.SIGNED, token.UNSIGNED, token.FLOAT, token.DOUBLE,
		token.INT8, token.INT16, token.INT32, token.INT64, token.INT128,
		token.NOEXCEPT:
		return true
	}
	return false
}

func (p *parser) isExprStart(k token.Kind) bool {
	switch k {
	case token.IDENT, token.INT_LIT, token.FLOAT_LIT, token.CHAR_LIT, token.STRING_LIT,
		token.TRUE, token.FALSE, token.NULLPTR, token.THIS,
		token.LPAREN, token.LBRACE, token.LBRACK,
		token.INC, token.DEC, token.ADD, token.SUB, token.MUL, token.AND, token.NOT, token.TILDE,
		token.SIZEOF, token.ALIGNOF, token.NOEXCEPT, token.NEW, token.DELETE, token.THROW,
		token.STATIC_CAST, token.DYNAMIC_CAST, token.CONST_CAST, token.REINTERPRET_CAST,
		token.TYPEID:
		return true

	// Functional cast start: int(a) or double{x}.
	case token.VOID, token.BOOL, token.CHAR, token.CHAR8_T, token.CHAR16_T,
		token.CHAR32_T, token.WCHAR_T, token.INT, token.SHORT, token.LONG,
		token.SIGNED, token.UNSIGNED, token.FLOAT, token.DOUBLE,
		token.INT8, token.INT16, token.INT32, token.INT64, token.INT128,
		token.DECLTYPE, token.TYPENAME, token.SCOPE:
		return true
	}
	return false
}

func (p *parser) isStmtTerminator(k token.Kind) bool {
	return k == token.SEMI || k == token.RBRACE || k == token.EOF
}

// parseSizeofExpr parses sizeof(T), sizeof expr, or sizeof...(pack).
func (p *parser) parseSizeofExpr() *ast.SizeofExpr {
	start := p.next()
	ell := ast.NoTok

	if p.peek() == token.ELLIPSIS {
		ell = p.next()
		lp := p.expect(token.LPAREN)
		name := p.parseName()
		rp := p.expect(token.RPAREN)
		return &ast.SizeofExpr{
			Span:     ast.Span{Lo: start, Hi: rp + 1},
			Keyword:  start,
			Ellipsis: ell,
			Lparen:   lp,
			X:        name,
			Rparen:   rp,
		}
	}

	if p.peek() == token.LPAREN {
		lp := p.next()
		if p.startsTypeId() {
			typeId := p.parseTypeId()
			rp := p.expect(token.RPAREN)
			return &ast.SizeofExpr{
				Span:     ast.Span{Lo: start, Hi: rp + 1},
				Keyword:  start,
				Ellipsis: ast.NoTok,
				Lparen:   lp,
				Type:     typeId,
				Rparen:   rp,
			}
		}
		x := p.parseExpr()
		rp := p.expect(token.RPAREN)
		return &ast.SizeofExpr{
			Span:     ast.Span{Lo: start, Hi: rp + 1},
			Keyword:  start,
			Ellipsis: ast.NoTok,
			Lparen:   lp,
			X:        x,
			Rparen:   rp,
		}
	}

	x := p.parseCastOrUnaryExpr()
	return &ast.SizeofExpr{
		Ellipsis: ast.NoTok,
		Span:     ast.Span{Lo: start, Hi: x.End()},
		Keyword:  start,
		Lparen:   ast.NoTok,
		X:        x,
		Rparen:   ast.NoTok,
	}
}

// parensArePlacement reports whether '(' after `new` opens a new-placement
// rather than a parenthesized type-id.
func (p *parser) parensArePlacement() bool {
	depth := 0
	for i := 0; ; i++ {
		switch p.peekAt(i) {
		case token.LPAREN:
			depth++
		case token.RPAREN:
			depth--
			if depth == 0 {
				k := p.peekAt(i + 1)
				return k == token.IDENT || k == token.SCOPE || p.isTypeStart(k)
			}
		case token.EOF, token.SEMI, token.LBRACE, token.RBRACE:
			return false
		}
	}
}

func (p *parser) parseNewExpr(global ast.Tok) *ast.NewExpr {
	start := p.pos()
	if global != ast.NoTok {
		start = global
	}
	kw := p.expect(token.NEW)

	var placement []ast.Expr
	placeLp, placeRp := ast.NoTok, ast.NoTok
	if p.peek() == token.LPAREN && p.parensArePlacement() {
		placeLp = p.next()
		for !p.atEOF() && p.peek() != token.RPAREN {
			placement = append(placement, p.parseAssignmentExpr())
			if p.peek() == token.COMMA {
				p.next()
			} else {
				break
			}
		}
		placeRp = p.expect(token.RPAREN)
	}

	lp, rp := ast.NoTok, ast.NoTok
	if p.peek() == token.LPAREN {
		lp = p.next()
	}

	// Only unparenthesized forms use restricted new-type-id declarator rules.
	if lp == ast.NoTok {
		p.inNewTypeId = true
	}
	typeId := p.parseTypeId()
	p.inNewTypeId = false

	if lp != ast.NoTok {
		rp = p.expect(token.RPAREN)
	}

	var init ast.Expr
	var newArgs []ast.Expr
	newLp, newRp := ast.NoTok, ast.NoTok
	if p.peek() == token.LBRACE {
		init = p.parseInitList()
	} else if p.peek() == token.LPAREN {
		// Parenthesized argument list as *ParenExpr
		initLp := p.next()
		var args []ast.Expr
		for !p.atEOF() && p.peek() != token.RPAREN {
			args = append(args, p.parseAssignmentExpr())
			if p.peek() == token.COMMA {
				p.next()
			} else {
				break
			}
		}
		initRp := p.expect(token.RPAREN)
		var inner ast.Expr
		if len(args) == 1 {
			inner = args[0]
		}
		init = &ast.ParenExpr{
			Span:   ast.Span{Lo: initLp, Hi: initRp + 1},
			Lparen: initLp,
			X:      inner,
			Rparen: initRp,
		}
		newArgs, newLp, newRp = args, initLp, initRp
	}

	hi := typeId.End()
	if rp != ast.NoTok {
		hi = rp + 1
	}
	if init != nil {
		hi = init.End()
	}

	return &ast.NewExpr{
		Span:      ast.Span{Lo: start, Hi: hi},
		Global:    global,
		Keyword:   kw,
		Placement: placement,
		PlaceLp:   placeLp,
		PlaceRp:   placeRp,
		Lparen:    lp,
		Type:      typeId,
		Rparen:    rp,
		Init:      init,
		Args:      newArgs,
		ArgsLp:    newLp,
		ArgsRp:    newRp,
	}
}

// parseDeleteExpr parses delete X or delete[] X
func (p *parser) parseDeleteExpr(global ast.Tok) *ast.DeleteExpr {
	start := p.pos()
	if global != ast.NoTok {
		start = global
	}
	kw := p.expect(token.DELETE)
	lb, rb := ast.NoTok, ast.NoTok
	if p.peek() == token.LBRACK && p.peekAt(1) == token.RBRACK {
		lb = p.next()
		rb = p.next()
	}
	x := p.parseCastOrUnaryExpr()
	return &ast.DeleteExpr{
		Span:    ast.Span{Lo: start, Hi: x.End()},
		Global:  global,
		Keyword: kw,
		Lbrack:  lb,
		Rbrack:  rb,
		X:       x,
	}
}

// parsePostfixExpr parses postfix expressions: calls, subscripts, member accesses, etc.
func (p *parser) parsePostfixExpr() ast.Expr {
	expr := p.parsePrimaryExpr()

	for {
		switch p.peek() {
		case token.LBRACK:
			// Subscript: expr[args...]
			lb := p.next()
			var args []ast.Expr
			for !p.atEOF() && p.peek() != token.RBRACK {
				args = append(args, p.parseAssignmentExpr())
				if p.peek() == token.COMMA {
					p.next()
				} else {
					break
				}
			}
			rb := p.expect(token.RBRACK)
			expr = &ast.IndexExpr{
				Span:   ast.Span{Lo: expr.Pos(), Hi: rb + 1},
				X:      expr,
				Lbrack: lb,
				Args:   args,
				Rbrack: rb,
			}

		case token.LPAREN:
			// Function call: expr(args...)
			lp := p.next()
			var args []ast.Expr
			for !p.atEOF() && p.peek() != token.RPAREN {
				args = append(args, p.parseAssignmentExpr())
				if p.peek() == token.COMMA {
					p.next()
				} else {
					break
				}
			}
			rp := p.expect(token.RPAREN)
			expr = &ast.CallExpr{
				Span:   ast.Span{Lo: expr.Pos(), Hi: rp + 1},
				Fun:    expr,
				Lparen: lp,
				Args:   args,
				Rparen: rp,
			}

		case token.PERIOD, token.ARROW:
			// Member access: expr.sel or expr->sel
			opKind := p.peek()
			opTok := p.next()
			// A member's name is the class's, which the name table does
			// not see; the `template` keyword, when written, is left for
			// the name parser, which always opens arguments after it.
			p.memberSelector = true
			sel := p.parseName()
			p.memberSelector = false
			if sel == nil {
				sel = &ast.BadName{Span: ast.Span{Lo: p.pos(), Hi: p.pos() + 1}}
			}
			tmplTok := ast.NoTok
			if tn, isTemplate := sel.(*ast.TemplateName); isTemplate {
				tmplTok = tn.Template
			}
			expr = &ast.MemberExpr{
				Span:     ast.Span{Lo: expr.Pos(), Hi: sel.End()},
				X:        expr,
				OpPos:    opTok,
				Op:       opKind,
				Template: tmplTok,
				Sel:      sel,
			}

		case token.INC, token.DEC:
			// Postfix ++ / --
			opKind := p.peek()
			opTok := p.next()
			expr = &ast.IncDecExpr{
				Span:  ast.Span{Lo: expr.Pos(), Hi: opTok + 1},
				X:     expr,
				OpPos: opTok,
				Op:    opKind,
			}

		default:
			return expr
		}
	}
}

// parsePrimaryExpr parses primary expressions: literals, id, lambda, folds, etc.
func (p *parser) parsePrimaryExpr() ast.Expr {
	start := p.pos()

	switch p.peek() {
	case token.INT_LIT, token.FLOAT_LIT, token.CHAR_LIT:
		k := p.peek()
		tok := p.next()
		return &ast.BasicLit{
			Span: ast.Span{Lo: tok, Hi: tok + 1},
			Kind: k,
		}

	case token.TRUE, token.FALSE, token.NULLPTR:
		k := p.peek()
		tok := p.next()
		return &ast.BasicLit{
			Span: ast.Span{Lo: tok, Hi: tok + 1},
			Kind: k,
		}

	case token.STRING_LIT:
		return p.parseStringLit()

	case token.THIS:
		tok := p.next()
		return &ast.ThisExpr{
			Span: ast.Span{Lo: tok, Hi: tok + 1},
		}

	case token.LBRACK:
		// Lambda expression: [captures] ...
		return p.parseLambdaExpr()

	case token.LBRACE:
		// Braced-init-list: { ... }
		return p.parseInitList()

	case token.STATIC_CAST, token.DYNAMIC_CAST, token.CONST_CAST, token.REINTERPRET_CAST:
		return p.parseNamedCastExpr()

	case token.TYPEID:
		kw := p.next()
		lp := p.expect(token.LPAREN)
		var typeId *ast.TypeId
		var x ast.Expr
		if p.startsTypeId() {
			typeId = p.parseTypeId()
		} else {
			x = p.parseExpr()
		}
		rp := p.expect(token.RPAREN)
		return &ast.TypeidExpr{
			Span:    ast.Span{Lo: kw, Hi: rp + 1},
			Keyword: kw,
			Lparen:  lp,
			Type:    typeId,
			X:       x,
			Rparen:  rp,
		}

	case token.REQUIRES:
		return p.parseRequiresExpr()

	case token.LPAREN:
		return p.parseParenOrFoldExpr()
	}

	// Builtin type trait: __is_base_of(A, B) -- one of the spellings the
	// toolsets' headers use (see typeTraits), whose operands are types.
	// Any other `__name(` is a call: `__std_exception_copy(&a, &b)`,
	// `__assume(false)`, `__builtin_addressof(x)`.
	if p.peek() == token.IDENT && typeTraits[p.text(p.cur)] && p.peekAt(1) == token.LPAREN && p.kindOf(p.text(p.cur)) != nameValue {
		name := p.parseIdent()
		lp := p.next()
		var args []*ast.TypeId
		for !p.atEOF() && p.peek() != token.RPAREN {
			args = append(args, p.parseTypeId())
			if p.peek() == token.COMMA {
				p.next()
			} else {
				break
			}
		}
		rp := p.expect(token.RPAREN)
		return &ast.TypeTraitExpr{
			Span:   ast.Span{Lo: name.Pos(), Hi: rp + 1},
			Name:   name,
			Lparen: lp,
			Args:   args,
			Rparen: rp,
		}
	}

	// Names, id-expressions, or functional casts: T(args) or T{args}
	if p.isTypeStart(p.peek()) {
		// Could be a functional cast like `int(5)` or `std::string{"abc"}`
		if p.isFunctionalCast() {
			// Functional cast: what precedes '(' or '{' is a type-specifier, never a declarator.
			specsStart := p.pos()
			specs := p.parseDeclSpecs()
			typeId := &ast.TypeId{
				Span:  ast.Span{Lo: specsStart, Hi: p.pos()},
				Specs: specs,
			}
			if p.peek() == token.LBRACE {
				initList := p.parseInitList()
				return &ast.FunctionalCastExpr{
					Span: ast.Span{Lo: typeId.Pos(), Hi: initList.End()},
					Type: typeId,
					Args: initList,
				}
			}
			lp := p.expect(token.LPAREN)
			var args []ast.Expr
			for !p.atEOF() && p.peek() != token.RPAREN {
				args = append(args, p.parseAssignmentExpr())
				if p.peek() == token.COMMA {
					p.next()
				} else {
					break
				}
			}
			rp := p.expect(token.RPAREN)
			return &ast.FunctionalCastExpr{
				Span:    ast.Span{Lo: typeId.Pos(), Hi: rp + 1},
				Type:    typeId,
				Lparen:  lp,
				ArgList: args,
				Rparen:  rp,
			}
		}
	}

	name := p.parseName()
	if name != nil {
		return name
	}

	p.error(p.cur, "expected expression")
	tok := p.next()
	return &ast.BadExpr{Span: ast.Span{Lo: start, Hi: tok + 1}}
}

func (p *parser) isFunctionalCast() bool {
	// Check simple type keywords followed immediately by '(' or '{'
	k := p.peek()
	switch k {
	case token.TYPENAME:
		// In an expression, typename-specifier is a functional cast.
		return true
	case token.INT, token.FLOAT, token.DOUBLE, token.CHAR, token.BOOL, token.VOID,
		token.INT8, token.INT16, token.INT32, token.INT64, token.INT128,
		token.UNSIGNED, token.SIGNED, token.SHORT, token.LONG:
		if p.peekAt(1) == token.LPAREN || p.peekAt(1) == token.LBRACE {
			return true
		}
	}
	return p.isBracedFunctionalCast()
}

// isBracedFunctionalCast reports whether the cursor is at `T { ... }`.
func (p *parser) isBracedFunctionalCast() bool {
	i := 0
	if p.peekAt(i) == token.SCOPE {
		i++
	}
	if p.peekAt(i) != token.IDENT {
		return false
	}
	for {
		i++ // the identifier itself
		if p.peekAt(i) == token.LSS {
			depth := 0
			parens := 0
			for {
				switch p.peekAt(i) {
				case token.LSS:
					depth++
				case token.GTR:
					if parens == 0 {
						depth--
					}
				case token.SHR:
					if parens == 0 {
						depth -= 2
					}
				case token.LPAREN:
					// `make_index_sequence<1 + sizeof...(_Rest)>{}` -- a
					// parenthesized argument, whose brackets are its own.
					parens++
				case token.RPAREN:
					if parens == 0 {
						return false
					}
					parens--
				case token.EOF, token.SEMI, token.LBRACE, token.RBRACE,
					token.LBRACK, token.RBRACK:
					// Not a template-argument-list: `a < b` and stop.
					return false
				}
				i++
				if depth < 0 {
					// `>>` closed this list and the one around it:
					// `bool_constant<is_void_v<_Ty>> {` -- the brace is
					// the enclosing construct's.
					return false
				}
				if depth == 0 {
					break
				}
			}
		}
		if p.peekAt(i) == token.SCOPE {
			i++
			if p.peekAt(i) != token.IDENT {
				return false
			}
			continue
		}
		break
	}
	return p.peekAt(i) == token.LBRACE
}

// parseStringLit parses adjacent string literal tokens.
func (p *parser) parseStringLit() *ast.StringLit {
	start := p.pos()
	var segs []ast.Span
	raw := false

	for p.peek() == token.STRING_LIT {
		tok := p.next()
		segs = append(segs, ast.Span{Lo: tok, Hi: tok + 1})
	}

	return &ast.StringLit{
		Span: ast.Span{Lo: start, Hi: p.pos()},
		Segs: segs,
		Raw:  raw,
	}
}

// parseParenOrFoldExpr parses ( expr ) or fold expressions: ( pack op ... ) etc.
func (p *parser) parseParenOrFoldExpr() ast.Expr {
	lp := p.expect(token.LPAREN)

	// Save and temporarily clear inTemplateArgs inside parentheses
	prevTmpl := p.inTemplateArgs
	p.inTemplateArgs = 0
	defer func() {
		p.inTemplateArgs = prevTmpl
	}()

	// Unary fold: ( ... op pack )
	if p.peek() == token.ELLIPSIS {
		ell := p.next()
		opTok := p.pos()
		opKind := p.peek()
		p.next()
		right := p.parseCastOrUnaryExpr()
		rp := p.expect(token.RPAREN)
		return &ast.FoldExpr{
			Span:     ast.Span{Lo: lp, Hi: rp + 1},
			Lparen:   lp,
			Ellipsis: ell,
			OpPos:    opTok,
			Op:       opKind,
			Right:    right,
			Rparen:   rp,
		}
	}

	x := p.parseCastOrUnaryExpr()

	// Fold expressions with left operand: ( pack op ... ) or ( pack op ... op init )
	if p.peek() != token.RPAREN && p.isFoldOp(p.peek()) && p.peekAt(1) == token.ELLIPSIS {
		opTok := p.pos()
		opKind := p.peek()
		p.next()
		ell := p.next() // consume ...

		var op2Tok ast.Tok = ast.NoTok
		var op2Kind token.Kind
		var right ast.Expr

		if p.isFoldOp(p.peek()) {
			op2Tok = p.pos()
			op2Kind = p.peek()
			p.next()
			right = p.parseCastOrUnaryExpr()
		}

		rp := p.expect(token.RPAREN)
		return &ast.FoldExpr{
			Span:     ast.Span{Lo: lp, Hi: rp + 1},
			Lparen:   lp,
			Left:     x,
			OpPos:    opTok,
			Op:       opKind,
			Ellipsis: ell,
			Op2Pos:   op2Tok,
			Op2:      op2Kind,
			Right:    right,
			Rparen:   rp,
		}
	}

	// Normal parenthesized expression: continue parsing binary operators
	x = p.parseBinaryExprRemainder(x, 1)

	rp := p.expect(token.RPAREN)
	return &ast.ParenExpr{
		Span:   ast.Span{Lo: lp, Hi: rp + 1},
		Lparen: lp,
		X:      x,
		Rparen: rp,
	}
}

func (p *parser) isFoldOp(k token.Kind) bool {
	return k.Precedence() > 0 || p.isAssignOp(k)
}

// parseInitList parses a braced-init-list `{ items... }`
func (p *parser) parseInitList() *ast.InitList {
	lb := p.expect(token.LBRACE)
	var items []ast.Expr
	comma := ast.NoTok

	for !p.atEOF() && p.peek() != token.RBRACE {
		if p.peek() == token.PERIOD {
			// Designated initializer: .name = value or .name{...}
			dot := p.next()
			name := p.parseIdent()
			assign := ast.NoTok
			var val ast.Expr
			if p.peek() == token.ASSIGN {
				assign = p.next()
				val = p.parseAssignmentExpr()
			} else if p.peek() == token.LBRACE {
				val = p.parseInitList()
			}
			items = append(items, &ast.DesignatedInit{
				Span:   ast.Span{Lo: dot, Hi: val.End()},
				Dot:    dot,
				Name:   name,
				Assign: assign,
				Value:  val,
			})
		} else if p.peek() == token.LBRACE {
			items = append(items, p.parseInitList())
		} else {
			items = append(items, p.parseAssignmentExpr())
		}

		if p.peek() == token.COMMA {
			comma = p.next()
		} else {
			break
		}
	}

	rb := p.expect(token.RBRACE)
	return &ast.InitList{
		Span:   ast.Span{Lo: lb, Hi: rb + 1},
		Lbrace: lb,
		Items:  items,
		Comma:  comma,
		Rbrace: rb,
	}
}

// parseNamedCastExpr parses static_cast<T>(x), etc.
func (p *parser) parseNamedCastExpr() *ast.NamedCastExpr {
	start := p.pos()
	kind := p.peek()
	kw := p.next()

	less := p.expect(token.LSS)
	typeId := p.parseTypeId()
	gtr := p.expect(token.GTR)

	lp := p.expect(token.LPAREN)
	x := p.parseExpr()
	rp := p.expect(token.RPAREN)

	return &ast.NamedCastExpr{
		Span:    ast.Span{Lo: start, Hi: rp + 1},
		Keyword: kw,
		Kind:    kind,
		Less:    less,
		Type:    typeId,
		Greater: gtr,
		Lparen:  lp,
		X:       x,
		Rparen:  rp,
	}
}

// parseLambdaExpr parses `[captures] <templs>opt (params)opt specs... { body }`
func (p *parser) parseLambdaExpr() *ast.LambdaExpr {
	start := p.pos()
	lb := p.expect(token.LBRACK)

	defTok := ast.NoTok
	var defKind token.Kind
	var captures []*ast.Capture

	// Check capture default: [=] or [&]
	if p.peek() == token.ASSIGN || (p.peek() == token.AND && (p.peekAt(1) == token.COMMA || p.peekAt(1) == token.RBRACK)) {
		defKind = p.peek()
		defTok = p.next()
		if p.peek() == token.COMMA {
			p.next()
		}
	}

	for !p.atEOF() && p.peek() != token.RBRACK {
		cap := p.parseSingleCapture()
		if cap != nil {
			captures = append(captures, cap)
		}
		if p.peek() == token.COMMA {
			p.next()
		} else {
			break
		}
	}

	rb := p.expect(token.RBRACK)

	var templ *ast.TemplateParams
	if p.peek() == token.LSS {
		less := p.next()
		templ = p.parseTemplateParamsContent(less, less)
	}

	lp, rp := ast.NoTok, ast.NoTok
	var params []*ast.ParamDecl
	var vararg ast.Tok = ast.NoTok

	if p.peek() == token.LPAREN {
		lp = p.next()
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
		rp = p.expect(token.RPAREN)
	}

	var specs []ast.DeclSpec
	for {
		k := p.peek()
		if k == token.MUTABLE || k == token.CONSTEXPR || k == token.CONSTEVAL || k == token.STATIC {
			tok := p.next()
			specs = append(specs, &ast.BasicSpec{
				Span: ast.Span{Lo: tok, Hi: tok + 1},
				Kind: k,
			})
		} else {
			break
		}
	}

	var noex *ast.NoexceptSpec
	if p.peek() == token.NOEXCEPT {
		noex = p.parseNoexceptSpec()
	}

	attrs := p.parseAttrGroups()
	var attrList []*ast.Attr
	for _, ag := range attrs {
		attrList = append(attrList, ag.Attrs...)
	}

	var trailing *ast.TrailingReturn
	if p.peek() == token.ARROW {
		arr := p.next()
		retType := p.parseTypeId()
		trailing = &ast.TrailingReturn{
			Span:  ast.Span{Lo: arr, Hi: retType.End()},
			Arrow: arr,
			Type:  retType,
		}
	}

	var req *ast.RequiresClause
	if p.peek() == token.REQUIRES {
		req = p.parseRequiresClause()
	}

	body := p.parseCompoundStmt()

	return &ast.LambdaExpr{
		Span:     ast.Span{Lo: start, Hi: body.End()},
		Lbrack:   lb,
		Default:  defTok,
		DefKind:  defKind,
		Captures: captures,
		Rbrack:   rb,
		Templ:    templ,
		Lparen:   lp,
		Params:   params,
		Vararg:   vararg,
		Rparen:   rp,
		Specs:    specs,
		Noexcept: noex,
		Attrs:    attrList,
		Trailing: trailing,
		Requires: req,
		Body:     body,
	}
}

func (p *parser) parseSingleCapture() *ast.Capture {
	start := p.pos()
	amp := ast.NoTok
	star := ast.NoTok
	thisTok := ast.NoTok
	var name *ast.Ident
	assign := ast.NoTok
	var init ast.Expr
	ell := ast.NoTok

	if p.peek() == token.MUL && p.peekAt(1) == token.THIS {
		star = p.next()
		thisTok = p.next()
		return &ast.Capture{
			Span: ast.Span{Lo: start, Hi: thisTok + 1},
			Star: star,
			This: thisTok,
			Amp:  ast.NoTok,
		}
	}

	if p.peek() == token.THIS {
		thisTok = p.next()
		return &ast.Capture{
			Span: ast.Span{Lo: start, Hi: thisTok + 1},
			Star: ast.NoTok,
			This: thisTok,
			Amp:  ast.NoTok,
		}
	}

	if p.peek() == token.AND {
		amp = p.next()
	}

	if p.peek() == token.IDENT {
		name = p.parseIdent()
	}

	if p.peek() == token.ASSIGN {
		assign = p.next()
		init = p.parseAssignmentExpr()
	}

	if p.peek() == token.ELLIPSIS {
		ell = p.next()
	}

	hi := start + 1
	if name != nil {
		hi = name.End()
	}
	if init != nil {
		hi = init.End()
	}
	if ell != ast.NoTok {
		hi = ell + 1
	}

	return &ast.Capture{
		Span:     ast.Span{Lo: start, Hi: hi},
		Amp:      amp,
		Star:     ast.NoTok,
		This:     ast.NoTok,
		Name:     name,
		Assign:   assign,
		Init:     init,
		Ellipsis: ell,
	}
}

// parseRequiresExpr parses `requires (params) { requirements... }`
func (p *parser) parseRequiresExpr() *ast.RequiresExpr {
	start := p.pos()
	kw := p.expect(token.REQUIRES)

	lp, rp := ast.NoTok, ast.NoTok
	var params []*ast.ParamDecl

	if p.peek() == token.LPAREN {
		lp = p.next()
		for !p.atEOF() && p.peek() != token.RPAREN {
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
		rp = p.expect(token.RPAREN)
	}

	lb := p.expect(token.LBRACE)
	var reqs []ast.Node

	for !p.atEOF() && p.peek() != token.RBRACE {
		req := p.parseRequirement()
		if req != nil {
			reqs = append(reqs, req)
		}
	}

	rb := p.expect(token.RBRACE)

	return &ast.RequiresExpr{
		Span:    ast.Span{Lo: start, Hi: rb + 1},
		Keyword: kw,
		Lparen:  lp,
		Params:  params,
		Rparen:  rp,
		Lbrace:  lb,
		Reqs:    reqs,
		Rbrace:  rb,
	}
}

func (p *parser) parseRequirement() ast.Node {
	start := p.pos()

	// Nested requirement: requires expr;
	if p.peek() == token.REQUIRES {
		kw := p.next()
		expr := p.parseExpr()
		semi := p.expect(token.SEMI)
		return &ast.NestedReq{
			Span:    ast.Span{Lo: start, Hi: semi + 1},
			Keyword: kw,
			X:       expr,
			Semi:    semi,
		}
	}

	// Type requirement: typename T::type;
	if p.peek() == token.TYPENAME {
		tn := p.next()
		name := p.parseName()
		semi := p.expect(token.SEMI)
		return &ast.TypeReq{
			Span:     ast.Span{Lo: start, Hi: semi + 1},
			Typename: tn,
			Name:     name,
			Semi:     semi,
		}
	}

	// Compound requirement: { expr } noexceptopt -> constraintopt ;
	if p.peek() == token.LBRACE {
		lb := p.next()
		expr := p.parseExpr()
		rb := p.expect(token.RBRACE)
		noex := ast.NoTok
		if p.peek() == token.NOEXCEPT {
			noex = p.next()
		}
		arrow := ast.NoTok
		var ret ast.Expr
		if p.peek() == token.ARROW {
			arrow = p.next()
			ret = p.parseAssignmentExpr()
		}
		semi := p.expect(token.SEMI)
		return &ast.CompoundReq{
			Span:     ast.Span{Lo: start, Hi: semi + 1},
			Lbrace:   lb,
			X:        expr,
			Rbrace:   rb,
			Noexcept: noex,
			Arrow:    arrow,
			Ret:      ret,
			Semi:     semi,
		}
	}

	// Simple requirement: expr;
	expr := p.parseExpr()
	semi := p.expect(token.SEMI)
	return &ast.SimpleReq{
		Span: ast.Span{Lo: start, Hi: semi + 1},
		X:    expr,
		Semi: semi,
	}
}
