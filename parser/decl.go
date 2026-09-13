package parser

import (
	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
)

// isDeclStart reports whether the current token sequence starts a declaration.
func (p *parser) isDeclStart() bool {
	k := p.peek()
	switch k {
	case token.TEMPLATE, token.EXPORT,
		token.NAMESPACE, token.USING, token.STATIC_ASSERT,
		token.TYPEDEF, token.FRIEND, token.INLINE, token.VIRTUAL,
		token.CONSTEXPR, token.CONSTEVAL, token.CONSTINIT,
		token.EXTERN, token.STATIC, token.THREAD_LOCAL, token.MUTABLE,
		token.CLASS, token.STRUCT, token.UNION, token.ENUM,
		token.PUBLIC, token.PROTECTED, token.PRIVATE,
		token.VOID, token.BOOL, token.CHAR, token.CHAR8_T, token.CHAR16_T, token.CHAR32_T,
		token.WCHAR_T, token.INT, token.SHORT, token.LONG, token.SIGNED, token.UNSIGNED,
		token.FLOAT, token.DOUBLE, token.AUTO,
		token.INT8, token.INT16, token.INT32, token.INT64, token.INT128,
		token.CONST, token.VOLATILE, token.RESTRICT,
		token.TYPENAME, token.DECLTYPE:
		return true

	case token.SEMI:
		return true

	case token.TILDE:
		// Destructor declaration: ~C()
		return true

	case token.SCOPE:
		// ::A::B or ::~C
		return true

	case token.LBRACK:
		if p.peekAt(1) == token.LBRACK {
			return true // [[attribute]]
		}

	case token.ALIGNAS:
		// An alignment-specifier is an attribute that opens a declaration.
		return true
	}

	if p.isModuleKeyword() || p.isImportKeyword() {
		return true
	}

	// Could be an identifier naming a type, constructor, or structured binding: `auto [a, b]`
	if k == token.IDENT {
		// Lookahead check for `T x;` or `T x = ...;` or `T(x);` vs `x = 1;`
		if p.peekAt(1) == token.IDENT || p.peekAt(1) == token.MUL || p.peekAt(1) == token.AND || p.peekAt(1) == token.LAND {
			return true
		}
		// `errno_t const r = ...` -- a cv-qualifier after a name can
		// only be a type's.
		if p.peekAt(1) == token.CONST || p.peekAt(1) == token.VOLATILE {
			return true
		}
		if p.peekAt(1) == token.SCOPE {
			return true
		}
		// Constrained placeholder (e.g. `Sortable auto v = ...;`).
		if p.peekAt(1) == token.AUTO || p.peekAt(1) == token.DECLTYPE {
			return true
		}
		if p.peekAt(1) == token.LSS && p.isTemplateArgsAt(1) {
			return true
		}
	}

	return false
}

// parseDecl parses any declaration.
func (p *parser) parseDecl() ast.Decl {
	return p.parseDeclInternal(true)
}

// parseDeclNoSemi parses a declaration without requiring a terminating semicolon (used in for loops).
func (p *parser) parseDeclNoSemi() ast.Decl {
	return p.parseDeclInternal(false)
}

func (p *parser) parseDeclInternal(expectSemi bool) ast.Decl {
	// A pragma phase 4 passed through occupies a whole logical line and is
	// not a declaration; see skipPragmaLines.
	p.skipPragmaLines()
	if p.peek() == token.RBRACE || p.atEOF() {
		// The pragma was the last thing in the scope.
		return nil
	}

	start := p.pos()

	// Empty declaration: lone semicolon
	if p.peek() == token.SEMI {
		semi := p.next()
		return &ast.EmptyDecl{
			Span: ast.Span{Lo: semi, Hi: semi + 1},
			Semi: semi,
		}
	}

	// Template declaration: template <...>
	if p.peek() == token.TEMPLATE || (p.peek() == token.EXTERN && p.peekAt(1) == token.TEMPLATE) {
		return p.parseTemplateDecl()
	}

	// Export declaration: export ...
	if p.peek() == token.EXPORT {
		if p.isModuleKeywordAt(1) {
			return p.parseModuleDecl()
		}
		if p.isImportKeywordAt(1) {
			return p.parseImportDecl()
		}
		return p.parseExportDecl()
	}

	// Module declaration: module ...
	if p.isModuleKeyword() {
		return p.parseModuleDecl()
	}

	// Import declaration: import ...
	if p.isImportKeyword() {
		return p.parseImportDecl()
	}

	// Namespace declaration: namespace ...
	if p.peek() == token.NAMESPACE || (p.peek() == token.INLINE && p.peekAt(1) == token.NAMESPACE) {
		return p.parseNamespaceDecl()
	}

	// Using declaration or directive or alias: using ...
	if p.peek() == token.USING {
		return p.parseUsingOrAliasDecl()
	}

	// Static assert: static_assert(...)
	if p.peek() == token.STATIC_ASSERT {
		return p.parseStaticAssertDecl()
	}

	// Linkage spec: extern "C" ...
	if p.peek() == token.EXTERN && p.peekAt(1) == token.STRING_LIT {
		return p.parseLinkageDecl()
	}

	// Asm declaration: asm(...)
	if p.peek() == token.ASM {
		stmt := p.parseAsmStmt()
		return &ast.AsmDecl{
			Span: ast.Span{Lo: stmt.Pos(), Hi: stmt.End()},
			Stmt: stmt,
		}
	}

	// Attributes preceding declaration
	attrs := p.parseAttrGroups()

	// Structured binding: auto [a, b, c] = expr;
	if (p.peek() == token.AUTO || (p.peek() == token.CONST && p.peekAt(1) == token.AUTO)) && p.isStructuredBinding() {
		return p.parseStructuredBinding(attrs, expectSemi)
	}

	// Declaration specifiers
	specs := p.parseDeclSpecs()

	// If specs ends with class/struct/enum definition and no declarator follows before semicolon
	if len(specs.List) > 0 && p.peek() == token.SEMI {
		semi := p.next()
		return &ast.SimpleDecl{
			Span:  ast.Span{Lo: start, Hi: semi + 1},
			Attrs: attrs,
			Specs: specs,
			Semi:  semi,
		}
	}

	// Parse first declarator
	firstDecl := p.parseDeclarator()
	p.noteDeclarator(specs, firstDecl)

	// Check if this is a function definition
	if p.isFuncDefinition(firstDecl) {
		return p.parseFuncDecl(start, attrs, specs, firstDecl)
	}

	// Otherwise, it's a simple declaration with one or more init-declarators
	var inits []*ast.InitDeclarator
	firstInit := p.parseInitDeclaratorSuffix(firstDecl)
	inits = append(inits, firstInit)

	for p.peek() == token.COMMA {
		p.next()
		nextDecl := p.parseDeclarator()
		p.noteDeclarator(specs, nextDecl)
		inits = append(inits, p.parseInitDeclaratorSuffix(nextDecl))
	}

	semi := ast.NoTok
	if expectSemi {
		semi = p.expect(token.SEMI)
	}

	hi := p.pos()
	if semi != ast.NoTok {
		hi = semi + 1
	} else if len(inits) > 0 {
		hi = inits[len(inits)-1].End()
	}

	return &ast.SimpleDecl{
		Span:  ast.Span{Lo: start, Hi: hi},
		Attrs: attrs,
		Specs: specs,
		Inits: inits,
		Semi:  semi,
	}
}

func (p *parser) isFuncDefinition(d ast.Declarator) bool {
	if d == nil {
		return false
	}
	// Check if declarator is a function declarator (or wraps one)
	if !p.hasFuncShape(d) {
		return false
	}
	// Followed by body '{', ctor-init ':', 'try', or '= default', '= delete'
	k := p.peek()
	if k == token.LBRACE || k == token.COLON || k == token.TRY {
		return true
	}
	if k == token.ASSIGN && (p.peekAt(1) == token.DEFAULT || p.peekAt(1) == token.DELETE) {
		return true
	}
	return false
}

func (p *parser) hasFuncShape(d ast.Declarator) bool {
	curr := d
	for curr != nil {
		switch node := curr.(type) {
		case *ast.FuncDeclarator:
			return true
		case *ast.PointerDeclarator:
			curr = node.Inner
		case *ast.ParenDeclarator:
			curr = node.Inner
		case *ast.ArrayDeclarator:
			curr = node.Inner
		default:
			return false
		}
	}
	return false
}

func (p *parser) parseFuncDecl(start ast.Tok, attrs []*ast.AttrGroup, specs *ast.DeclSpecs, decl ast.Declarator) *ast.FuncDecl {
	fd := &ast.FuncDecl{
		Span:      ast.Span{Lo: start, Hi: p.pos()},
		Attrs:     attrs,
		Specs:     specs,
		Decl:      decl,
		Colon:     ast.NoTok,
		Assign:    ast.NoTok,
		Defaulted: ast.NoTok,
		Deleted:   ast.NoTok,
		DelLparen: ast.NoTok,
		DelRparen: ast.NoTok,
		Semi:      ast.NoTok,
	}

	// In a function-try-block, ctor-initializers sit inside the try block:
	// `try ctor-initializer_opt compound-statement handler-seq`.
	if p.peek() == token.TRY {
		p.pushNames()
		p.noteParams(decl)
		fd.Body = p.parseFunctionTryBlock(fd)
		p.popNames()
		fd.Span.Hi = fd.Body.End()
		return fd
	}

	if p.peek() == token.COLON {
		p.parseCtorInitializer(fd)
	}

	// Body or = default / = delete
	if p.peek() == token.ASSIGN {
		fd.Assign = p.next()
		if p.peek() == token.DEFAULT {
			fd.Defaulted = p.next()
		} else if p.peek() == token.DELETE {
			fd.Deleted = p.next()
			// C++26 = delete("reason")
			if p.peek() == token.LPAREN {
				fd.DelLparen = p.next()
				fd.Reason = p.parseStringLit()
				fd.DelRparen = p.expect(token.RPAREN)
			}
		}
		fd.Semi = p.expect(token.SEMI)
		fd.Span.Hi = fd.Semi + 1
		return fd
	}

	if p.mode&SkipBodies != 0 {
		p.skipBalanced(token.LBRACE, token.RBRACE)
		fd.Span.Hi = p.pos()
		return fd
	}

	p.pushNames()
	p.noteParams(decl)
	fd.Body = p.parseCompoundStmt()
	p.popNames()
	fd.Span.Hi = fd.Body.End()
	return fd
}

// parseCtorInitializer reads `: Base(args), member{val}` onto fd.
func (p *parser) parseCtorInitializer(fd *ast.FuncDecl) {
	fd.Colon = p.next()
	for !p.atEOF() && p.peek() != token.LBRACE && p.peek() != token.TRY {
		init := p.parseMemInit()
		if init != nil {
			fd.Inits = append(fd.Inits, init)
		}
		if p.peek() == token.COMMA {
			p.next()
		} else {
			break
		}
	}
}

// parseFunctionTryBlock reads `try ctor-initializer_opt compound-statement handler-seq`.
func (p *parser) parseFunctionTryBlock(fd *ast.FuncDecl) *ast.TryStmt {
	start := p.pos()
	kw := p.expect(token.TRY)

	if p.peek() == token.COLON {
		p.parseCtorInitializer(fd)
	}

	body := p.parseCompoundStmt()

	var handlers []*ast.CatchClause
	for p.peek() == token.CATCH {
		if h := p.parseCatchClause(); h != nil {
			handlers = append(handlers, h)
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

func (p *parser) parseMemInit() *ast.MemInit {
	start := p.pos()
	name := p.parseName()
	if name == nil {
		return nil
	}

	var args []ast.Expr
	lp, rp := ast.NoTok, ast.NoTok
	var braced *ast.InitList

	if p.peek() == token.LBRACE {
		braced = p.parseInitList()
	} else if p.peek() == token.LPAREN {
		lp = p.next()
		for !p.atEOF() && p.peek() != token.RPAREN {
			args = append(args, p.parseAssignmentExpr())
			if p.peek() == token.COMMA {
				p.next()
			} else {
				break
			}
		}
		rp = p.expect(token.RPAREN)
	}

	ell := ast.NoTok
	if p.peek() == token.ELLIPSIS {
		ell = p.next()
	}

	hi := name.End()
	if braced != nil {
		hi = braced.End()
	} else if rp != ast.NoTok {
		hi = rp + 1
	}
	if ell != ast.NoTok {
		hi = ell + 1
	}

	return &ast.MemInit{
		Span:     ast.Span{Lo: start, Hi: hi},
		Name:     name,
		Lparen:   lp,
		Args:     args,
		Rparen:   rp,
		Braced:   braced,
		Ellipsis: ell,
	}
}

func (p *parser) parseInitDeclaratorSuffix(decl ast.Declarator) *ast.InitDeclarator {
	init := &ast.InitDeclarator{
		Span:   ast.Span{Lo: decl.Pos(), Hi: decl.End()},
		Decl:   decl,
		Assign: ast.NoTok,
		Lparen: ast.NoTok,
		Rparen: ast.NoTok,
		Asm:    ast.NoTok,
	}

	// GNU's asm label comes straight after the declarator, and attributes
	// may follow it: `char *realpath(const char *, char *)
	// __asm("_realpath$DARWIN_EXTSN") __attribute__((...));`.
	if p.peek() == token.ASM && p.peekAt(1) == token.LPAREN {
		init.Asm = p.next()
		p.next()
		for p.peek() == token.STRING_LIT {
			init.AsmLabel = append(init.AsmLabel, p.next())
		}
		rp := p.expect(token.RPAREN)
		init.Span.Hi = rp + 1
		p.parseAttrGroups()
	}

	if p.peek() == token.ASSIGN {
		init.Assign = p.next()
		if p.peek() == token.LBRACE {
			init.Braced = p.parseInitList()
			init.Span.Hi = init.Braced.End()
		} else {
			init.Value = p.parseAssignmentExpr()
			init.Span.Hi = init.Value.End()
		}
	} else if p.peek() == token.LBRACE {
		init.Braced = p.parseInitList()
		init.Span.Hi = init.Braced.End()
	} else if p.peek() == token.LPAREN {
		init.Lparen = p.next()
		for !p.atEOF() && p.peek() != token.RPAREN {
			init.Args = append(init.Args, p.parseAssignmentExpr())
			if p.peek() == token.COMMA {
				p.next()
			} else {
				break
			}
		}
		init.Rparen = p.expect(token.RPAREN)
		init.Span.Hi = init.Rparen + 1
	}

	if p.peek() == token.REQUIRES {
		init.Requires = p.parseRequiresClause()
		init.Span.Hi = init.Requires.End()
	}

	return init
}

func (p *parser) isStructuredBinding() bool {
	// Look ahead for `[a, b]`
	for i := 0; ; i++ {
		k := p.peekAt(i)
		if k == token.LBRACK {
			return true
		}
		if k == token.AND || k == token.LAND || k == token.CONST || k == token.VOLATILE || k == token.AUTO {
			continue
		}
		break
	}
	return false
}

func (p *parser) parseStructuredBinding(attrs []*ast.AttrGroup, expectSemi bool) *ast.StructuredBinding {
	start := p.pos()
	specs := p.parseDeclSpecs()

	refTok := ast.NoTok
	var refKind token.Kind
	if p.peek() == token.AND || p.peek() == token.LAND {
		refKind = p.peek()
		refTok = p.next()
	}

	lb := p.expect(token.LBRACK)
	var names []*ast.Ident
	for !p.atEOF() && p.peek() != token.RBRACK {
		names = append(names, p.parseIdent())
		if p.peek() == token.COMMA {
			p.next()
		} else {
			break
		}
	}
	rb := p.expect(token.RBRACK)

	assign := ast.NoTok
	var val ast.Expr
	lp, rp := ast.NoTok, ast.NoTok
	var args []ast.Expr
	var braced *ast.InitList

	if p.peek() == token.ASSIGN {
		assign = p.next()
		if p.peek() == token.LBRACE {
			braced = p.parseInitList()
		} else {
			val = p.parseAssignmentExpr()
		}
	} else if p.peek() == token.LBRACE {
		braced = p.parseInitList()
	} else if p.peek() == token.LPAREN {
		lp = p.next()
		for !p.atEOF() && p.peek() != token.RPAREN {
			args = append(args, p.parseAssignmentExpr())
			if p.peek() == token.COMMA {
				p.next()
			} else {
				break
			}
		}
		rp = p.expect(token.RPAREN)
	}

	semi := ast.NoTok
	if expectSemi {
		semi = p.expect(token.SEMI)
	}

	hi := rb + 1
	if semi != ast.NoTok {
		hi = semi + 1
	} else if val != nil {
		hi = val.End()
	} else if braced != nil {
		hi = braced.End()
	}

	return &ast.StructuredBinding{
		Span:    ast.Span{Lo: start, Hi: hi},
		Attrs:   attrs,
		Specs:   specs,
		Ref:     refTok,
		RefKind: refKind,
		Lbrack:  lb,
		Names:   names,
		Rbrack:  rb,
		Assign:  assign,
		Value:   val,
		Lparen:  lp,
		Args:    args,
		Rparen:  rp,
		Braced:  braced,
		Semi:    semi,
	}
}

// parseMemberDecl parses a declaration inside a class (includes public:/private:/protected:)
func (p *parser) parseMemberDecl() ast.Decl {
	// A pragma line inside a class body -- `#pragma warning(pop)` before
	// the closing brace -- is not a member.
	p.skipPragmaLines()
	if p.peek() == token.RBRACE || p.atEOF() {
		return nil
	}

	// Access specifier: public:, protected:, private:
	k := p.peek()
	if k == token.PUBLIC || k == token.PROTECTED || k == token.PRIVATE {
		tok := p.next()
		colon := p.expect(token.COLON)
		return &ast.AccessDecl{
			Span:    ast.Span{Lo: tok, Hi: colon + 1},
			Keyword: tok,
			Kind:    k,
			Colon:   colon,
		}
	}

	return p.parseDecl()
}

func (p *parser) parseNamespaceDecl() *ast.NamespaceDecl {
	start := p.pos()
	inlineTok := ast.NoTok
	if p.peek() == token.INLINE {
		inlineTok = p.next()
	}
	kw := p.expect(token.NAMESPACE)

	attrs := p.parseAttrGroups()

	// Check namespace alias: namespace N = A::B;
	if p.peek() == token.IDENT && p.peekAt(1) == token.ASSIGN {
		name := p.parseIdent()
		assign := p.next()
		target := p.parseName()
		semi := p.expect(token.SEMI)
		return &ast.NamespaceDecl{
			Span:    ast.Span{Lo: start, Hi: semi + 1},
			Inline:  inlineTok,
			Keyword: kw,
			Attrs:   attrs,
			Names:   []*ast.Ident{name},
			Decls: []ast.Decl{
				&ast.NamespaceAliasDecl{
					Span:    ast.Span{Lo: start, Hi: semi + 1},
					Keyword: kw,
					Name:    name,
					Assign:  assign,
					Target:  target,
					Semi:    semi,
				},
			},
		}
	}

	var names []*ast.Ident
	var inlines []ast.Tok

	for p.peek() == token.IDENT || p.peek() == token.INLINE {
		inl := ast.NoTok
		if p.peek() == token.INLINE {
			inl = p.next()
		}
		inlines = append(inlines, inl)
		if p.peek() == token.IDENT {
			names = append(names, p.parseIdent())
		}
		if p.peek() == token.SCOPE {
			p.next()
		} else {
			break
		}
	}

	lb := p.expect(token.LBRACE)
	var decls []ast.Decl
	// Reopening a namespace continues the existing namespace scope.
	var path []string
	for _, n := range names {
		path = append(path, n.Text(p.u))
	}
	p.pushNamespaceNames(path)
	for !p.atEOF() && p.peek() != token.RBRACE {
		prev := p.pos()
		d := p.parseDecl()
		if d != nil {
			decls = append(decls, d)
		}
		if p.pos() == prev {
			p.advanceTo(token.SEMI, token.RBRACE)
			if p.peek() == token.SEMI {
				p.next()
			}
		}
	}
	p.popNamespaceNames(path)
	rb := p.expect(token.RBRACE)

	return &ast.NamespaceDecl{
		Span:    ast.Span{Lo: start, Hi: rb + 1},
		Inline:  inlineTok,
		Keyword: kw,
		Attrs:   attrs,
		Names:   names,
		Inlines: inlines,
		Lbrace:  lb,
		Decls:   decls,
		Rbrace:  rb,
	}
}

func (p *parser) parseUsingOrAliasDecl() ast.Decl {
	start := p.pos()
	attrs := p.parseAttrGroups()
	kw := p.expect(token.USING)

	// Using directive: using namespace N;
	if p.peek() == token.NAMESPACE {
		ns := p.next()
		name := p.parseName()
		semi := p.expect(token.SEMI)
		return &ast.UsingDirectiveDecl{
			Span:      ast.Span{Lo: start, Hi: semi + 1},
			Attrs:     attrs,
			Keyword:   kw,
			Namespace: ns,
			Name:      name,
			Semi:      semi,
		}
	}

	// Alias declaration: using T = TypeId;
	if p.peek() == token.IDENT && (p.peekAt(1) == token.ASSIGN || (p.peekAt(1) == token.LBRACK && p.peekAt(2) == token.LBRACK) ||
		p.peekAt(1) == token.ATTRIBUTE || p.peekAt(1) == token.DECLSPEC) {
		name := p.parseIdent()
		p.noteType(name.Text(p.u))
		if p.declaringTemplate {
			p.noteTemplate(name.Text(p.u), nameType)
			p.declaringTemplate = false
		}
		// Attributes can appear between the name and `=` (e.g. `using name [[deprecated]] = T;`).
		aliasAttrs := p.parseAttrGroups()
		attrs = append(attrs, aliasAttrs...)
		assign := p.expect(token.ASSIGN)
		typeId := p.parseTypeId()
		semi := p.expect(token.SEMI)
		return &ast.AliasDecl{
			Span:    ast.Span{Lo: start, Hi: semi + 1},
			Attrs:   attrs,
			Keyword: kw,
			Name:    name,
			Assign:  assign,
			Type:    typeId,
			Semi:    semi,
		}
	}

	// Using declaration: using A::b; or using enum E;
	enumTok := ast.NoTok
	if p.peek() == token.ENUM {
		enumTok = p.next()
	}
	typenameTok := ast.NoTok
	if p.peek() == token.TYPENAME {
		typenameTok = p.next()
	}

	var names []ast.Name
	for !p.atEOF() && p.peek() != token.SEMI {
		name := p.parseName()
		if name != nil {
			names = append(names, name)
		}
		if p.peek() == token.COMMA {
			p.next()
		} else {
			break
		}
	}

	semi := p.expect(token.SEMI)
	return &ast.UsingDecl{
		Span:     ast.Span{Lo: start, Hi: semi + 1},
		Keyword:  kw,
		Enum:     enumTok,
		Typename: typenameTok,
		Names:    names,
		Semi:     semi,
	}
}

func (p *parser) parseStaticAssertDecl() *ast.StaticAssertDecl {
	start := p.pos()
	kw := p.expect(token.STATIC_ASSERT)
	lp := p.expect(token.LPAREN)
	cond := p.parseAssignmentExpr()

	comma := ast.NoTok
	var msg ast.Expr
	if p.peek() == token.COMMA {
		comma = p.next()
		msg = p.parseAssignmentExpr()
	}
	rp := p.expect(token.RPAREN)
	semi := p.expect(token.SEMI)

	return &ast.StaticAssertDecl{
		Span:    ast.Span{Lo: start, Hi: semi + 1},
		Keyword: kw,
		Lparen:  lp,
		Cond:    cond,
		Comma:   comma,
		Msg:     msg,
		Rparen:  rp,
		Semi:    semi,
	}
}

func (p *parser) parseLinkageDecl() *ast.LinkageDecl {
	start := p.pos()
	kw := p.expect(token.EXTERN)
	lang := p.parseStringLit()

	if p.peek() == token.LBRACE {
		lb := p.next()
		var decls []ast.Decl
		for !p.atEOF() && p.peek() != token.RBRACE {
			prev := p.pos()
			d := p.parseDecl()
			if d != nil {
				decls = append(decls, d)
			}
			if p.pos() == prev {
				p.advanceTo(token.SEMI, token.RBRACE)
				if p.peek() == token.SEMI {
					p.next()
				}
			}
		}
		rb := p.expect(token.RBRACE)
		return &ast.LinkageDecl{
			Span:    ast.Span{Lo: start, Hi: rb + 1},
			Keyword: kw,
			Lang:    lang,
			Lbrace:  lb,
			Decls:   decls,
			Rbrace:  rb,
		}
	}

	d := p.parseDecl()
	return &ast.LinkageDecl{
		Span:    ast.Span{Lo: start, Hi: d.End()},
		Keyword: kw,
		Lang:    lang,
		Lbrace:  ast.NoTok,
		Decls:   []ast.Decl{d},
		Rbrace:  ast.NoTok,
	}
}

func (p *parser) parseModuleDecl() ast.Decl {
	start := p.pos()
	exportTok := ast.NoTok
	if p.peek() == token.EXPORT {
		exportTok = p.next()
	}
	kw := p.next() // consume 'module'

	// Global module fragment: `module;`
	if p.peek() == token.SEMI {
		semi := p.next()
		return &ast.ModuleDecl{
			Span:    ast.Span{Lo: start, Hi: semi + 1},
			Export:  exportTok,
			Keyword: kw,
			Semi:    semi,
		}
	}

	// Private module fragment: `module :private;`
	var privTok ast.Tok = ast.NoTok
	var colonTok ast.Tok = ast.NoTok
	var name []*ast.Ident
	var part []*ast.Ident

	if p.peek() == token.COLON {
		colonTok = p.next()
		if p.peek() == token.IDENT && p.text(p.cur) == "private" {
			privTok = p.next()
		} else {
			part = p.parseModulePath()
		}
	} else {
		name = p.parseModulePath()
		if p.peek() == token.COLON {
			colonTok = p.next()
			part = p.parseModulePath()
		}
	}

	attrs := p.parseAttrGroups()
	semi := p.expect(token.SEMI)

	return &ast.ModuleDecl{
		Span:    ast.Span{Lo: start, Hi: semi + 1},
		Export:  exportTok,
		Keyword: kw,
		Name:    name,
		Colon:   colonTok,
		Part:    part,
		Private: privTok,
		Attrs:   attrs,
		Semi:    semi,
	}
}

func (p *parser) parseImportDecl() ast.Decl {
	start := p.pos()
	exportTok := ast.NoTok
	if p.peek() == token.EXPORT {
		exportTok = p.next()
	}
	kw := p.next() // consume 'import'

	var hdrSpan ast.Span
	var name []*ast.Ident
	var colonTok ast.Tok = ast.NoTok
	var part []*ast.Ident

	if p.peek() == token.HEADER_NAME {
		tok := p.next()
		hdrSpan = ast.Span{Lo: tok, Hi: tok + 1}
	} else if p.peek() == token.COLON {
		colonTok = p.next()
		part = p.parseModulePath()
	} else {
		name = p.parseModulePath()
		if p.peek() == token.COLON {
			colonTok = p.next()
			part = p.parseModulePath()
		}
	}

	attrs := p.parseAttrGroups()
	semi := p.expect(token.SEMI)

	return &ast.ImportDecl{
		Span:    ast.Span{Lo: start, Hi: semi + 1},
		Export:  exportTok,
		Keyword: kw,
		Name:    name,
		Colon:   colonTok,
		Part:    part,
		Header:  hdrSpan,
		Attrs:   attrs,
		Semi:    semi,
	}
}

func (p *parser) parseModulePath() []*ast.Ident {
	var idents []*ast.Ident
	for p.peek() == token.IDENT {
		idents = append(idents, p.parseIdent())
		if p.peek() == token.PERIOD {
			p.next()
		} else {
			break
		}
	}
	return idents
}

func (p *parser) parseExportDecl() ast.Decl {
	start := p.pos()
	kw := p.expect(token.EXPORT)

	if p.peek() == token.LBRACE {
		lb := p.next()
		var decls []ast.Decl
		for !p.atEOF() && p.peek() != token.RBRACE {
			prev := p.pos()
			d := p.parseDecl()
			if d != nil {
				decls = append(decls, d)
			}
			if p.pos() == prev {
				p.advanceTo(token.SEMI, token.RBRACE)
				if p.peek() == token.SEMI {
					p.next()
				}
			}
		}
		rb := p.expect(token.RBRACE)
		return &ast.ExportDecl{
			Span:    ast.Span{Lo: start, Hi: rb + 1},
			Keyword: kw,
			Lbrace:  lb,
			Decls:   decls,
			Rbrace:  rb,
		}
	}

	d := p.parseDecl()
	return &ast.ExportDecl{
		Span:    ast.Span{Lo: start, Hi: d.End()},
		Keyword: kw,
		Lbrace:  ast.NoTok,
		Decls:   []ast.Decl{d},
		Rbrace:  ast.NoTok,
	}
}
