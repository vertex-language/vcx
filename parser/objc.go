package parser

import (
	"strings"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
)

// Objective-C++: the syntax an Objective-C++ unit (Mode ObjC) adds to C++.
// An Objective-C keyword is two tokens, AT and the name after it, and the
// rest is ordinary tokens read in contexts only Objective-C has.

// objc reports whether the unit is Objective-C++.
func (p *parser) objc() bool { return p.mode&ObjC != 0 }

// atWord reports whether the cursor is `@word`.
func (p *parser) atWord(word string) bool {
	return p.peek() == token.AT && p.wordAt(1) == word
}

// wordAt is the spelling of the token at offset when it is a name or a
// keyword -- `@class` is AT and the keyword class -- and "" otherwise.
func (p *parser) wordAt(offset int) string {
	if p.selectorTok(offset) {
		return p.text(p.peekTok(offset))
	}
	return ""
}

// selectorTok reports whether the token at offset can be a piece of a
// selector: a name, or any keyword -- `[obj class]`, `-delete:`.
func (p *parser) selectorTok(offset int) bool {
	k := p.peekAt(offset)
	if k == token.IDENT || k.IsKeyword() {
		return true
	}
	// C++'s alternative tokens -- `and`, `or`, `not` -- are words too,
	// and clang reads `-mid:(P)a and:(P)b` as the selector mid:and:.
	t := p.text(p.peekTok(offset))
	return t != "" && (t[0] >= 'a' && t[0] <= 'z')
}

// ident reads the name or keyword at the cursor as an identifier.
func (p *parser) anyIdent() *ast.Ident {
	tok := p.next()
	return &ast.Ident{Span: ast.Span{Lo: tok, Hi: tok + 1}}
}

// expectName reads an identifier, reporting one's absence.
func (p *parser) expectName(what string) *ast.Ident {
	if p.peek() != token.IDENT {
		p.error(p.cur, "expected "+what)
		return nil
	}
	return p.anyIdent()
}

// noteObjCClass records a class name: a type from here on.
func (p *parser) noteObjCClass(name string) {
	if p.objcClasses == nil {
		p.objcClasses = map[string]bool{}
	}
	p.objcClasses[name] = true
	if len(p.names) > 0 {
		p.names[0][name] = nameType
	}
}

func (p *parser) noteObjCProtocol(name string) {
	if p.objcProtocols == nil {
		p.objcProtocols = map[string]bool{}
	}
	p.objcProtocols[name] = true
}

// isObjCObjectName reports whether an identifier names an Objective-C
// object type that may take angle-bracket lists: id, Class, instancetype,
// a class, or a generic class's parameter.
func (p *parser) isObjCObjectName(name string) bool {
	switch name {
	case "id", "Class", "instancetype":
		return true
	}
	return p.objcClasses[name] || p.objcTypeParams[name]
}

// ---- declarations ----

// objcDeclAhead reports whether an Objective-C declaration begins at the
// cursor, after any attributes: `__attribute__((...)) @interface`.
func (p *parser) objcDeclAhead() bool {
	if !p.objc() {
		return false
	}
	if p.peek() == token.AT {
		return objcDeclWords[p.wordAt(1)]
	}
	if p.peek() != token.ATTRIBUTE && p.peek() != token.DECLSPEC && p.peek() != token.EXTERN {
		return false
	}
	save, half := p.cur, p.halfGtr
	ndiags := len(p.diags)
	p.skipObjCDeclPrefix()
	ok := p.peek() == token.AT
	p.cur, p.halfGtr, p.diags = save, half, p.diags[:ndiags]
	return ok
}

// objcDeclWords are the @-keywords that begin a declaration; every other
// @ at the start of a statement is an expression or an @-statement.
var objcDeclWords = map[string]bool{
	"interface": true, "implementation": true, "protocol": true, "class": true,
	"compatibility_alias": true, "end": true,
}

// skipObjCDeclPrefix passes over what the SDK's macros put before an
// @interface: attributes, and an `extern` that means nothing there.
func (p *parser) skipObjCDeclPrefix() []*ast.AttrGroup {
	var attrs []*ast.AttrGroup
	for {
		if p.peek() == token.EXTERN {
			p.next()
			continue
		}
		g := p.parseAttrGroups()
		if len(g) == 0 {
			return attrs
		}
		attrs = append(attrs, g...)
	}
}

// parseObjCDecl reads an @interface, @implementation, @protocol, @class or
// @compatibility_alias.
func (p *parser) parseObjCDecl() ast.Decl {
	start := p.pos()
	attrs := p.skipObjCDeclPrefix()
	at := p.pos()
	switch p.wordAt(1) {
	case "interface":
		return p.parseObjCInterface(start, attrs)
	case "implementation":
		return p.parseObjCImpl(start, attrs)
	case "protocol":
		return p.parseObjCProtocol(start, attrs)
	case "class":
		return p.parseObjCForward(start, false)
	case "compatibility_alias":
		p.next()
		p.next()
		d := &ast.ObjCAliasDecl{Keyword: at, Alias: p.expectName("an alias name"), Class: p.expectName("a class name")}
		if d.Alias != nil {
			p.noteObjCClass(d.Alias.Text(p.u))
		}
		d.Semi = p.expect(token.SEMI)
		d.Span = ast.Span{Lo: start, Hi: p.pos()}
		return d
	case "import":
		// @import Foundation; -- a module import. The SDK's headers have
		// been read through #import already; there is nothing more here.
		for !p.atEOF() && p.peek() != token.SEMI {
			p.next()
		}
		semi := p.expect(token.SEMI)
		return &ast.EmptyDecl{Span: ast.Span{Lo: start, Hi: p.pos()}, Semi: semi}
	case "end":
		p.error(at, "@end without an @interface, @implementation or @protocol to end")
		p.next()
		p.next()
		return &ast.EmptyDecl{Span: ast.Span{Lo: start, Hi: p.pos()}, Semi: ast.NoTok}
	}
	p.error(at, "expected an Objective-C declaration after '@'")
	p.next()
	return &ast.EmptyDecl{Span: ast.Span{Lo: start, Hi: p.pos()}, Semi: ast.NoTok}
}

func (p *parser) parseObjCInterface(start ast.Tok, attrs []*ast.AttrGroup) ast.Decl {
	d := &ast.ObjCInterfaceDecl{Attrs: attrs, Keyword: p.pos(), Lparen: ast.NoTok}
	p.next()
	p.next()
	d.Name = p.expectName("a class name")
	if d.Name == nil {
		p.advanceTo(token.SEMI)
		return &ast.EmptyDecl{Span: ast.Span{Lo: start, Hi: p.pos()}, Semi: ast.NoTok}
	}
	name := d.Name.Text(p.u)
	p.noteObjCClass(name)

	p.pushNames()
	defer p.popNames()
	saved := p.objcTypeParams
	defer func() { p.objcTypeParams = saved }()

	if p.peek() == token.LSS && p.angleIsTypeParamList() {
		d.TypeParams = p.parseObjCTypeParams()
		p.recordClassParams(name, d.TypeParams)
	} else {
		p.declareClassParams(name)
	}

	if p.peek() == token.LPAREN {
		d.Lparen = p.next()
		if p.peek() == token.IDENT {
			d.Category = p.anyIdent()
		}
		p.expect(token.RPAREN)
	} else if p.peek() == token.COLON {
		p.next()
		d.Super = p.expectName("a superclass name")
		if d.Super != nil && p.peek() == token.LSS && !p.angleIsProtocolList() {
			d.SuperArgs = p.parseObjCTypeArgs()
		}
	}
	if p.peek() == token.LSS {
		d.Protocols = p.parseObjCProtocolRefs()
	}
	if p.peek() == token.LBRACE {
		d.Ivars = p.parseObjCIvars()
	}
	d.Members = p.parseObjCMembers(false)
	d.AtEnd = p.expectObjCEnd()
	d.Span = ast.Span{Lo: start, Hi: p.pos()}
	return d
}

func (p *parser) parseObjCImpl(start ast.Tok, attrs []*ast.AttrGroup) ast.Decl {
	d := &ast.ObjCImplDecl{Attrs: attrs, Keyword: p.pos()}
	p.next()
	p.next()
	d.Name = p.expectName("a class name")
	if d.Name == nil {
		p.advanceTo(token.SEMI)
		return &ast.EmptyDecl{Span: ast.Span{Lo: start, Hi: p.pos()}, Semi: ast.NoTok}
	}
	name := d.Name.Text(p.u)
	p.noteObjCClass(name)
	p.pushNames()
	defer p.popNames()
	saved := p.objcTypeParams
	defer func() { p.objcTypeParams = saved }()
	p.declareClassParams(name)

	if p.peek() == token.LPAREN {
		p.next()
		d.Category = p.expectName("a category name")
		p.expect(token.RPAREN)
	} else if p.peek() == token.COLON {
		p.next()
		d.Super = p.expectName("a superclass name")
	}
	if p.peek() == token.LBRACE {
		d.Ivars = p.parseObjCIvars()
	}
	d.Members = p.parseObjCMembers(true)
	d.AtEnd = p.expectObjCEnd()
	d.Span = ast.Span{Lo: start, Hi: p.pos()}
	return d
}

func (p *parser) parseObjCProtocol(start ast.Tok, attrs []*ast.AttrGroup) ast.Decl {
	at := p.pos()
	// @protocol A; and @protocol A, B; forward-declare.
	if p.peekAt(2) == token.IDENT && (p.peekAt(3) == token.SEMI || p.peekAt(3) == token.COMMA) {
		return p.parseObjCForward(start, true)
	}
	p.next()
	p.next()
	d := &ast.ObjCProtocolDecl{Attrs: attrs, Keyword: at, Name: p.expectName("a protocol name")}
	if d.Name == nil {
		p.advanceTo(token.SEMI)
		return &ast.EmptyDecl{Span: ast.Span{Lo: start, Hi: p.pos()}, Semi: ast.NoTok}
	}
	p.noteObjCProtocol(d.Name.Text(p.u))
	if p.peek() == token.LSS {
		d.Protocols = p.parseObjCProtocolRefs()
	}
	p.pushNames()
	d.Members = p.parseObjCMembers(false)
	p.popNames()
	d.AtEnd = p.expectObjCEnd()
	d.Span = ast.Span{Lo: start, Hi: p.pos()}
	return d
}

// parseObjCForward reads `@class A, B<T>;` or `@protocol A, B;`.
func (p *parser) parseObjCForward(start ast.Tok, protocol bool) ast.Decl {
	d := &ast.ObjCForwardDecl{Keyword: p.pos(), Protocol: protocol}
	p.next()
	p.next()
	for {
		id := p.expectName("a name")
		if id == nil {
			p.advanceTo(token.SEMI)
			break
		}
		name := id.Text(p.u)
		d.Names = append(d.Names, id)
		if protocol {
			p.noteObjCProtocol(name)
		} else {
			p.noteObjCClass(name)
			if p.peek() == token.LSS {
				// A forward declaration may state a generic class's
				// parameters, which its @implementation reads back.
				p.pushNames()
				saved := p.objcTypeParams
				params := p.parseObjCTypeParams()
				p.objcTypeParams = saved
				p.popNames()
				p.recordClassParams(name, params)
			}
		}
		if p.peek() != token.COMMA {
			break
		}
		p.next()
	}
	d.Semi = p.expect(token.SEMI)
	d.Span = ast.Span{Lo: start, Hi: p.pos()}
	return d
}

func (p *parser) expectObjCEnd() ast.Tok {
	if p.atWord("end") {
		at := p.next()
		p.next()
		return at
	}
	p.error(p.cur, "expected @end")
	return ast.NoTok
}

// parseObjCMembers reads what an @interface, @protocol or @implementation
// holds, up to its @end: methods, properties, and ordinary C++
// declarations, which in an @implementation may be function definitions.
func (p *parser) parseObjCMembers(impl bool) []ast.Decl {
	var out []ast.Decl
	for {
		p.skipPragmaLines()
		if p.atEOF() || p.atWord("end") {
			return out
		}
		before := p.cur
		if d := p.parseObjCMember(impl); d != nil {
			out = append(out, d)
		}
		if p.cur == before {
			p.next()
		}
	}
}

func (p *parser) parseObjCMember(impl bool) ast.Decl {
	start := p.pos()
	switch p.peek() {
	case token.SEMI:
		semi := p.next()
		return &ast.EmptyDecl{Span: ast.Span{Lo: semi, Hi: semi + 1}, Semi: semi}
	case token.SUB, token.ADD:
		return p.parseObjCMethod(impl)
	case token.AT:
		switch w := p.wordAt(1); w {
		case "property":
			return p.parseObjCProperty()
		case "synthesize", "dynamic":
			return p.parseObjCPropertyImpl()
		case "required", "optional":
			at := p.next()
			p.next()
			return &ast.ObjCMarkerDecl{Span: ast.Span{Lo: start, Hi: p.pos()}, Keyword: at, Word: w}
		}
	}
	return p.parseDecl()
}

// parseObjCIvars reads `{ @private int a; ... }`.
func (p *parser) parseObjCIvars() []ast.Decl {
	p.expect(token.LBRACE)
	var out []ast.Decl
	p.pushNames()
	defer p.popNames()
	for !p.atEOF() && p.peek() != token.RBRACE {
		p.skipPragmaLines()
		if p.peek() == token.RBRACE {
			break
		}
		start := p.pos()
		if p.peek() == token.AT {
			switch w := p.wordAt(1); w {
			case "private", "protected", "public", "package":
				at := p.next()
				p.next()
				out = append(out, &ast.ObjCMarkerDecl{Span: ast.Span{Lo: start, Hi: p.pos()}, Keyword: at, Word: w})
				continue
			}
		}
		before := p.cur
		if d := p.parseDecl(); d != nil {
			out = append(out, d)
		}
		if p.cur == before {
			p.next()
		}
	}
	p.expect(token.RBRACE)
	return out
}

// parseObjCMethod reads `- (T)sel`, `+ (T)do:(A)a with:(B)b, ...;`, and a
// definition's body.
func (p *parser) parseObjCMethod(impl bool) ast.Decl {
	start := p.pos()
	d := &ast.ObjCMethodDecl{Sign: p.peek(), Vararg: ast.NoTok, Semi: ast.NoTok}
	p.next()
	if p.peek() == token.LPAREN {
		d.Result = p.parseObjCMethodType()
	}
	d.Attrs = p.parseAttrGroups()

	p.pushNames()
	defer p.popNames()
	p.noteValue("self")
	p.noteValue("_cmd")
	p.noteValue("super")

	if p.selectorTok(0) && p.peekAt(1) != token.COLON {
		tok := p.next()
		d.Parts = []*ast.ObjCSelPart{{Span: ast.Span{Lo: tok, Hi: tok + 1},
			Name: &ast.Ident{Span: ast.Span{Lo: tok, Hi: tok + 1}}, Colon: ast.NoTok}}
	} else {
		for (p.selectorTok(0) && p.peekAt(1) == token.COLON) || p.peek() == token.COLON {
			plo := p.pos()
			part := &ast.ObjCSelPart{}
			if p.peek() != token.COLON {
				part.Name = p.anyIdent()
			}
			part.Colon = p.expect(token.COLON)
			if p.peek() == token.LPAREN {
				part.Type = p.parseObjCMethodType()
			}
			part.Attrs = p.parseAttrGroups()
			if p.selectorTok(0) {
				part.Param = p.anyIdent()
				p.noteValue(part.Param.Text(p.u))
			} else {
				p.error(p.cur, "expected a parameter name in the method's selector")
			}
			part.Span = ast.Span{Lo: plo, Hi: p.pos()}
			d.Parts = append(d.Parts, part)
		}
		if len(d.Parts) == 0 {
			p.error(p.cur, "expected a selector")
			p.advanceTo(token.SEMI, token.LBRACE)
		}
		for p.peek() == token.COMMA {
			p.next()
			if p.peek() == token.ELLIPSIS {
				d.Vararg = p.next()
				break
			}
			prm := p.parseParamDecl()
			if prm.Decl != nil {
				if id, ok := prm.Decl.DeclName().(*ast.Ident); ok {
					p.noteValue(id.Text(p.u))
				}
			}
			d.Params = append(d.Params, prm)
		}
	}
	d.TailAttr = p.parseAttrGroups()

	// A definition may have a stray semicolon before its body.
	if impl && p.peek() == token.SEMI && p.peekAt(1) == token.LBRACE {
		p.next()
	}
	if p.peek() == token.LBRACE {
		if p.mode&SkipBodies != 0 {
			lb := p.pos()
			p.skipBalanced(token.LBRACE, token.RBRACE)
			d.Body = &ast.CompoundStmt{Span: ast.Span{Lo: lb, Hi: p.pos()}, Lbrace: lb, Rbrace: p.pos() - 1}
		} else {
			d.Body = p.parseCompoundStmt()
		}
	} else {
		d.Semi = p.expect(token.SEMI)
	}
	d.Span = ast.Span{Lo: start, Hi: p.pos()}
	return d
}

// objcMethodTypeWords are the words that may open a method's parenthesized
// type ahead of the type itself: the distributed-object qualifiers, and the
// nullabilities without their underscores, which mean nothing elsewhere.
var objcMethodTypeWords = map[string]bool{
	"in": true, "out": true, "inout": true, "bycopy": true, "byref": true, "oneway": true,
	"nullable": true, "nonnull": true, "null_unspecified": true, "null_resettable": true,
}

// parseObjCMethodType reads `( [qualifiers] T )`.
func (p *parser) parseObjCMethodType() *ast.TypeId {
	p.expect(token.LPAREN)
	for p.peek() == token.IDENT && objcMethodTypeWords[p.text(p.cur)] {
		p.next()
	}
	var t *ast.TypeId
	if p.peek() != token.RPAREN {
		t = p.parseTypeId()
	}
	p.expect(token.RPAREN)
	return t
}

// parseObjCProperty reads `@property (attrs) T a, *b;`.
func (p *parser) parseObjCProperty() ast.Decl {
	start := p.pos()
	d := &ast.ObjCPropertyDecl{Keyword: p.pos()}
	p.next()
	p.next()
	if p.peek() == token.LPAREN {
		p.next()
		for !p.atEOF() && p.peek() != token.RPAREN {
			// A macro that expands to nothing leaves an empty place.
			if p.peek() == token.COMMA {
				p.next()
				continue
			}
			alo := p.pos()
			if !p.selectorTok(0) {
				p.error(p.cur, "expected a property attribute")
				p.advanceTo(token.RPAREN)
				break
			}
			a := &ast.ObjCPropertyAttr{Name: p.anyIdent()}
			if w := a.Name.Text(p.u); (w == "getter" || w == "setter") && p.peek() == token.ASSIGN {
				p.next()
				var sel strings.Builder
				for p.selectorTok(0) || p.peek() == token.COLON {
					sel.WriteString(p.text(p.next()))
				}
				a.Selector = sel.String()
			}
			a.Span = ast.Span{Lo: alo, Hi: p.pos()}
			d.Attrs = append(d.Attrs, a)
			if p.peek() != token.COMMA {
				break
			}
			p.next()
		}
		p.expect(token.RPAREN)
	}
	d.Specs = p.parseDeclSpecs()
	for {
		dcl := p.parseDeclarator()
		p.parseAttrGroups()
		d.Decls = append(d.Decls, dcl)
		if p.peek() != token.COMMA {
			break
		}
		p.next()
	}
	d.Semi = p.expect(token.SEMI)
	d.Span = ast.Span{Lo: start, Hi: p.pos()}
	return d
}

// parseObjCPropertyImpl reads `@synthesize a = _a, b;` or `@dynamic a;`.
func (p *parser) parseObjCPropertyImpl() ast.Decl {
	start := p.pos()
	d := &ast.ObjCPropertyImplDecl{Keyword: p.pos(), Dynamic: p.wordAt(1) == "dynamic"}
	p.next()
	p.next()
	for {
		name := p.expectName("a property name")
		if name == nil {
			p.advanceTo(token.SEMI)
			break
		}
		var ivar *ast.Ident
		if p.peek() == token.ASSIGN {
			p.next()
			ivar = p.expectName("an instance variable name")
		}
		d.Names = append(d.Names, name)
		d.Ivars = append(d.Ivars, ivar)
		if p.peek() != token.COMMA {
			break
		}
		p.next()
	}
	d.Semi = p.expect(token.SEMI)
	d.Span = ast.Span{Lo: start, Hi: p.pos()}
	return d
}

// ---- generics and protocol lists ----

// parseObjCTypeParams reads `<__covariant T : B *, U>` and makes each
// parameter a type name in the current scope.
func (p *parser) parseObjCTypeParams() []*ast.ObjCTypeParam {
	p.expect(token.LSS)
	var out []*ast.ObjCTypeParam
	for !p.atEOF() && !p.atRangle() {
		plo := p.pos()
		tp := &ast.ObjCTypeParam{}
		if w := p.wordAt(0); w == "__covariant" || w == "__contravariant" {
			tp.Variance = w
			p.next()
		}
		tp.Name = p.expectName("a type parameter name")
		if tp.Name == nil {
			p.advanceTo(token.GTR)
			break
		}
		p.noteObjCTypeParam(tp.Name.Text(p.u))
		if p.peek() == token.COLON {
			p.next()
			tp.Bound = p.parseTypeId()
		}
		tp.Span = ast.Span{Lo: plo, Hi: p.pos()}
		out = append(out, tp)
		if p.peek() != token.COMMA {
			break
		}
		p.next()
	}
	p.expectRangle()
	return out
}

func (p *parser) noteObjCTypeParam(name string) {
	m := map[string]bool{name: true}
	for k := range p.objcTypeParams {
		m[k] = true
	}
	p.objcTypeParams = m
	p.noteType(name)
}

// recordClassParams remembers a generic class's parameters, which its
// categories and @implementation are written in terms of without restating
// them; declareClassParams brings them back.
func (p *parser) recordClassParams(class string, params []*ast.ObjCTypeParam) {
	if len(params) == 0 {
		return
	}
	if p.objcClassParams == nil {
		p.objcClassParams = map[string][]string{}
	}
	var names []string
	for _, tp := range params {
		if tp.Name != nil {
			names = append(names, tp.Name.Text(p.u))
		}
	}
	p.objcClassParams[class] = names
}

func (p *parser) declareClassParams(class string) {
	for _, n := range p.objcClassParams[class] {
		p.noteObjCTypeParam(n)
	}
}

// parseObjCProtocolRefs reads `<P, Q>`.
func (p *parser) parseObjCProtocolRefs() []*ast.Ident {
	p.expect(token.LSS)
	var out []*ast.Ident
	for !p.atEOF() && !p.atRangle() {
		id := p.expectName("a protocol name")
		if id == nil {
			p.advanceTo(token.GTR)
			break
		}
		out = append(out, id)
		if p.peek() != token.COMMA {
			break
		}
		p.next()
	}
	p.expectRangle()
	return out
}

// parseObjCTypeArgs reads `<NSString *, id>`.
func (p *parser) parseObjCTypeArgs() []*ast.TypeId {
	p.expect(token.LSS)
	p.inTemplateArgs++
	defer func() { p.inTemplateArgs-- }()
	var out []*ast.TypeId
	for !p.atEOF() && !p.atRangle() {
		before := p.cur
		out = append(out, p.parseTypeId())
		if p.cur == before {
			p.advanceTo(token.GTR)
			break
		}
		if p.peek() != token.COMMA {
			break
		}
		p.next()
	}
	p.expectRangle()
	return out
}

// atRangle reports whether the cursor closes an angle-bracket list: a >,
// or the first half of a >>.
func (p *parser) atRangle() bool {
	k := p.peek()
	return k == token.GTR || k == token.SHR
}

// expectRangle consumes one >, splitting a >> as a template argument list
// would: `NSArray<NSArray<id>>`.
func (p *parser) expectRangle() {
	if p.atRangle() {
		p.consumeGreater()
		return
	}
	p.error(p.cur, "expected >")
}

// angleIsProtocolList decides what a < after an object type or superclass
// opens: a protocol list is bare names, each a protocol; anything else is
// type arguments.
func (p *parser) angleIsProtocolList() bool {
	for n := 1; n < 64; n++ {
		switch k := p.peekAt(n); k {
		case token.IDENT:
			if n%2 == 0 {
				return false
			}
		case token.COMMA:
			if n%2 == 1 {
				return false
			}
		case token.GTR, token.SHR:
			if n%2 != 0 {
				return false
			}
			name := p.text(p.peekTok(1))
			if p.objcProtocols[name] {
				return true
			}
			// A class or a type is an argument; a name nothing declared
			// is likelier a protocol declared later.
			return !(p.objcClasses[name] || p.kindOf(name) == nameType)
		default:
			return false
		}
	}
	return false
}

// angleIsTypeParamList decides what a < after the class name of an
// @interface opens: parameters, unless every name in it is a protocol.
func (p *parser) angleIsTypeParamList() bool {
	for n := 1; n < 64; n++ {
		switch k := p.peekAt(n); k {
		case token.COLON:
			return true
		case token.IDENT:
			w := p.text(p.peekTok(n))
			if w == "__covariant" || w == "__contravariant" || !p.objcProtocols[w] {
				return true
			}
		case token.COMMA:
		case token.GTR, token.SHR:
			return false
		default:
			return true
		}
	}
	return true
}

// ---- types ----

// parseObjCTypeSpec reads an object type: a class name, id, Class or
// instancetype, and its angle-bracket lists.
func (p *parser) parseObjCTypeSpec(kindof bool, start ast.Tok) *ast.ObjCTypeSpec {
	s := &ast.ObjCTypeSpec{Name: p.anyIdent(), Kindof: kindof}
	name := s.Name.Text(p.u)
	if p.peek() == token.LSS {
		if name == "id" || name == "Class" || p.angleIsProtocolList() {
			s.Protocols = p.parseObjCProtocolRefs()
		} else {
			s.TypeArgs = p.parseObjCTypeArgs()
			if p.peek() == token.LSS {
				s.Protocols = p.parseObjCProtocolRefs()
			}
		}
	}
	s.Span = ast.Span{Lo: start, Hi: p.pos()}
	return s
}

// ---- attributes ----

// opensAttribute reports whether `[[` at the cursor opens an attribute.
// In Objective-C++ it may open a message whose receiver is a message,
// `[[NSObject alloc] init]`: an attribute's first word is followed by ]],
// (, a comma, :: or ..., and a message's receiver by its selector.
func (p *parser) opensAttribute() bool {
	if !p.objc() {
		return true
	}
	switch p.peekAt(2) {
	case token.RBRACK:
		return true
	case token.USING:
		return true
	case token.IDENT:
	default:
		return p.peekAt(2).IsKeyword() && p.peekAt(3) == token.RBRACK
	}
	switch p.peekAt(3) {
	case token.RBRACK, token.LPAREN, token.COMMA, token.SCOPE, token.ELLIPSIS:
		return true
	}
	return false
}

// ---- expressions ----

// parseObjCAtExpr reads what an @ opens in an expression.
func (p *parser) parseObjCAtExpr() ast.Expr {
	start := p.pos()
	at := p.pos()
	switch p.peekAt(1) {
	case token.STRING_LIT:
		// @"a" @"b" "c": one string, as adjacent literals are.
		s := &ast.StringLit{}
		lo := ast.NoTok
		for {
			if p.peek() == token.AT && p.peekAt(1) == token.STRING_LIT {
				p.next()
			}
			if p.peek() != token.STRING_LIT {
				break
			}
			tok := p.next()
			if !lo.IsValid() {
				lo = tok
			}
			s.Segs = append(s.Segs, ast.Span{Lo: tok, Hi: tok + 1})
		}
		s.Span = ast.Span{Lo: lo, Hi: p.pos()}
		return &ast.ObjCStringLit{Span: ast.Span{Lo: start, Hi: p.pos()}, At: at, Str: s}
	case token.LBRACK:
		p.next()
		p.next()
		a := &ast.ObjCArrayLit{At: at}
		for !p.atEOF() && p.peek() != token.RBRACK {
			a.Elems = append(a.Elems, p.parseAssignmentExpr())
			if p.peek() != token.COMMA {
				break
			}
			p.next()
		}
		p.expect(token.RBRACK)
		a.Span = ast.Span{Lo: start, Hi: p.pos()}
		return a
	case token.LBRACE:
		p.next()
		p.next()
		d := &ast.ObjCDictLit{At: at}
		for !p.atEOF() && p.peek() != token.RBRACE {
			d.Keys = append(d.Keys, p.parseAssignmentExpr())
			p.expect(token.COLON)
			d.Values = append(d.Values, p.parseAssignmentExpr())
			if p.peek() != token.COMMA {
				break
			}
			p.next()
		}
		p.expect(token.RBRACE)
		d.Span = ast.Span{Lo: start, Hi: p.pos()}
		return d
	case token.LPAREN:
		p.next()
		p.next()
		x := p.parseExpr()
		p.expect(token.RPAREN)
		return &ast.ObjCBoxedExpr{Span: ast.Span{Lo: start, Hi: p.pos()}, At: at, X: x}
	case token.INT_LIT, token.FLOAT_LIT, token.CHAR_LIT, token.ADD, token.SUB, token.TRUE, token.FALSE:
		p.next()
		x := p.parseCastOrUnaryExpr()
		return &ast.ObjCBoxedExpr{Span: ast.Span{Lo: start, Hi: p.pos()}, At: at, X: x}
	}
	switch w := p.wordAt(1); w {
	case "selector":
		p.next()
		p.next()
		p.expect(token.LPAREN)
		var sel strings.Builder
		for !p.atEOF() && p.peek() != token.RPAREN {
			if p.peek() == token.SCOPE {
				// `a::` scans as a name and ::.
				sel.WriteString("::")
				p.next()
				continue
			}
			if !p.selectorTok(0) && p.peek() != token.COLON {
				p.error(p.cur, "expected a selector")
				p.advanceTo(token.RPAREN)
				break
			}
			sel.WriteString(p.text(p.next()))
		}
		p.expect(token.RPAREN)
		return &ast.ObjCSelectorExpr{Span: ast.Span{Lo: start, Hi: p.pos()}, Keyword: at, Name: sel.String()}
	case "protocol":
		p.next()
		p.next()
		p.expect(token.LPAREN)
		name := p.expectName("a protocol name")
		p.expect(token.RPAREN)
		return &ast.ObjCProtocolExpr{Span: ast.Span{Lo: start, Hi: p.pos()}, Keyword: at, Name: name}
	case "encode":
		p.next()
		p.next()
		p.expect(token.LPAREN)
		t := p.parseTypeId()
		p.expect(token.RPAREN)
		return &ast.ObjCEncodeExpr{Span: ast.Span{Lo: start, Hi: p.pos()}, Keyword: at, Type: t}
	case "available":
		p.next()
		return p.parseObjCAvailable(start, at)
	case "__objc_yes", "__objc_no", "YES", "NO":
		p.next()
		x := p.parseCastOrUnaryExpr()
		return &ast.ObjCBoxedExpr{Span: ast.Span{Lo: start, Hi: p.pos()}, At: at, X: x}
	}
	p.error(at, "expected an Objective-C expression after '@'")
	p.next()
	return &ast.BadExpr{Span: ast.Span{Lo: start, Hi: p.pos()}}
}

// parseObjCAvailable reads `available(macos 10.15, ios 13, *)` after the
// @, or the same after __builtin_available.
func (p *parser) parseObjCAvailable(start, keyword ast.Tok) ast.Expr {
	p.next()
	p.expect(token.LPAREN)
	e := &ast.ObjCAvailableExpr{Keyword: keyword, Platforms: map[string]string{}}
	for !p.atEOF() && p.peek() != token.RPAREN {
		if p.peek() == token.MUL {
			p.next()
			break
		}
		platform := p.text(p.next())
		var v strings.Builder
		for p.peek() == token.INT_LIT || p.peek() == token.FLOAT_LIT || p.peek() == token.PERIOD {
			v.WriteString(p.text(p.next()))
		}
		e.Platforms[platform] = v.String()
		if p.peek() != token.COMMA {
			break
		}
		p.next()
	}
	p.expect(token.RPAREN)
	e.Span = ast.Span{Lo: start, Hi: p.pos()}
	return e
}

// tryObjCMessage reads a message send at a [, or leaves the cursor where
// it was and reports false when what is there is not one -- a lambda.
func (p *parser) tryObjCMessage() (ast.Expr, bool) {
	save, half, ndiags, errTok, resyncs := p.cur, p.halfGtr, len(p.diags), p.errTok, p.resyncs
	m := p.parseObjCMessage()
	if len(p.diags) == ndiags && m != nil {
		return m, true
	}
	p.cur, p.halfGtr, p.diags, p.errTok, p.resyncs = save, half, p.diags[:ndiags], errTok, resyncs
	return nil, false
}

// parseObjCMessage reads `[receiver sel]` or `[receiver key:arg ...]`.
func (p *parser) parseObjCMessage() *ast.ObjCMessageExpr {
	start := p.pos()
	m := &ast.ObjCMessageExpr{Lbrack: p.next()}
	switch {
	case p.peek() == token.RBRACK:
		return nil
	case p.peek() == token.IDENT && p.text(p.cur) == "super" && p.selectorTok(1):
		tok := p.next()
		m.Recv = &ast.ObjCSuperExpr{Span: ast.Span{Lo: tok, Hi: tok + 1}}
	case p.peek() == token.IDENT && p.isObjCObjectName(p.text(p.cur)) && (p.selectorTok(1) || p.peekAt(1) == token.LSS):
		// A class as the receiver: `[NSString string]`, and with type
		// arguments, `[NSArray<NSString *> array]`.
		name := p.anyIdent()
		r := &ast.ObjCClassRecv{Name: name}
		if p.peek() == token.LSS {
			r.TypeArgs = p.parseObjCTypeArgs()
		}
		r.Span = ast.Span{Lo: name.Pos(), Hi: p.pos()}
		m.Recv = r
	default:
		m.Recv = p.parseExpr()
	}
	if m.Recv == nil {
		return nil
	}
	if p.selectorTok(0) && p.peekAt(1) != token.COLON {
		tok := p.next()
		m.Parts = []*ast.ObjCMsgPart{{Span: ast.Span{Lo: tok, Hi: tok + 1},
			Name: &ast.Ident{Span: ast.Span{Lo: tok, Hi: tok + 1}}, Colon: ast.NoTok}}
	} else {
		for (p.selectorTok(0) && p.peekAt(1) == token.COLON) || p.peek() == token.COLON {
			plo := p.pos()
			part := &ast.ObjCMsgPart{}
			if p.peek() != token.COLON {
				part.Name = p.anyIdent()
			}
			part.Colon = p.expect(token.COLON)
			for {
				part.Args = append(part.Args, p.parseAssignmentExpr())
				if p.peek() != token.COMMA {
					break
				}
				p.next()
			}
			part.Span = ast.Span{Lo: plo, Hi: p.pos()}
			m.Parts = append(m.Parts, part)
		}
		if len(m.Parts) == 0 {
			p.error(p.cur, "expected a selector in the message")
			return nil
		}
	}
	m.Rbrack = p.expect(token.RBRACK)
	m.Span = ast.Span{Lo: start, Hi: p.pos()}
	return m
}

// parseBlockExpr reads a block literal: `^ [R] [(params)] { body }`.
func (p *parser) parseBlockExpr() ast.Expr {
	start := p.pos()
	b := &ast.BlockExpr{Caret: p.next(), Lparen: ast.NoTok, Vararg: ast.NoTok}
	// A return type is specifiers and pointers; the first ( is the
	// parameters'.
	if p.peek() != token.LPAREN && p.peek() != token.LBRACE && p.isTypeStart(p.peek()) {
		tlo := p.pos()
		specs := p.parseDeclSpecs()
		var d ast.Declarator = &ast.NameDeclarator{Span: ast.Span{Lo: p.pos(), Hi: p.pos()}}
		for p.peek() == token.MUL || p.peek() == token.XOR || p.peek() == token.AND {
			op := p.pos()
			k := p.peek()
			p.next()
			d = &ast.PointerDeclarator{Span: ast.Span{Lo: op, Hi: p.pos()}, OpPos: op, Kind: k, Inner: d}
		}
		b.Result = &ast.TypeId{Span: ast.Span{Lo: tlo, Hi: p.pos()}, Specs: specs, Decl: d}
	}
	p.pushNames()
	defer p.popNames()
	if p.peek() == token.LPAREN {
		b.Lparen = p.next()
		for !p.atEOF() && p.peek() != token.RPAREN {
			if p.peek() == token.ELLIPSIS {
				b.Vararg = p.next()
				break
			}
			if p.peek() == token.VOID && p.peekAt(1) == token.RPAREN {
				p.next()
				break
			}
			prm := p.parseParamDecl()
			if prm.Decl != nil {
				if id, ok := prm.Decl.DeclName().(*ast.Ident); ok {
					p.noteValue(id.Text(p.u))
				}
			}
			b.Params = append(b.Params, prm)
			if p.peek() != token.COMMA {
				break
			}
			p.next()
		}
		p.expect(token.RPAREN)
	}
	p.parseAttrGroups()
	if p.peek() != token.LBRACE {
		p.error(p.cur, "expected { to open the block's body")
		return &ast.BadExpr{Span: ast.Span{Lo: start, Hi: p.pos()}}
	}
	b.Body = p.parseCompoundStmt()
	b.Span = ast.Span{Lo: start, Hi: p.pos()}
	return b
}

// ---- statements ----

// parseObjCStmt reads @try, @throw, @synchronized and @autoreleasepool, or
// an expression statement that opens with an @ expression.
func (p *parser) parseObjCStmt() ast.Stmt {
	start := p.pos()
	at := p.pos()
	switch p.wordAt(1) {
	case "autoreleasepool":
		p.next()
		p.next()
		body := p.parseCompoundStmt()
		return &ast.ObjCAutoreleaseStmt{Span: ast.Span{Lo: start, Hi: p.pos()}, Keyword: at, Body: body}
	case "synchronized":
		p.next()
		p.next()
		p.expect(token.LPAREN)
		x := p.parseExpr()
		p.expect(token.RPAREN)
		body := p.parseCompoundStmt()
		return &ast.ObjCSyncStmt{Span: ast.Span{Lo: start, Hi: p.pos()}, Keyword: at, X: x, Body: body}
	case "throw":
		p.next()
		p.next()
		s := &ast.ObjCThrowStmt{Keyword: at}
		if p.peek() != token.SEMI {
			s.X = p.parseExpr()
		}
		p.expect(token.SEMI)
		s.Span = ast.Span{Lo: start, Hi: p.pos()}
		return s
	case "try":
		p.next()
		p.next()
		s := &ast.ObjCTryStmt{Keyword: at, Body: p.parseCompoundStmt()}
		for p.atWord("catch") {
			clo := p.pos()
			c := &ast.ObjCCatch{Keyword: p.next()}
			p.next()
			p.expect(token.LPAREN)
			p.pushNames()
			if p.peek() == token.ELLIPSIS {
				p.next()
			} else {
				c.Param = p.parseParamDecl()
				if c.Param.Decl != nil {
					if id, ok := c.Param.Decl.DeclName().(*ast.Ident); ok {
						p.noteValue(id.Text(p.u))
					}
				}
			}
			p.expect(token.RPAREN)
			c.Body = p.parseCompoundStmt()
			p.popNames()
			c.Span = ast.Span{Lo: clo, Hi: p.pos()}
			s.Catches = append(s.Catches, c)
		}
		if p.atWord("finally") {
			p.next()
			p.next()
			s.Finally = p.parseCompoundStmt()
		}
		s.Span = ast.Span{Lo: start, Hi: p.pos()}
		return s
	}
	x := p.parseExpr()
	semi := p.expect(token.SEMI)
	return &ast.ExprStmt{Span: ast.Span{Lo: start, Hi: p.pos()}, X: x, Semi: semi}
}

// isForIn reports whether the for statement whose ( has been read is fast
// enumeration: an `in` at the top level of the parentheses before any ;.
func (p *parser) isForIn() bool {
	if !p.objc() {
		return false
	}
	depth := 0
	for i := 0; i < 256; i++ {
		switch k := p.peekAt(i); k {
		case token.LPAREN, token.LBRACK, token.LBRACE:
			depth++
		case token.RPAREN, token.RBRACK, token.RBRACE:
			if depth == 0 {
				return false
			}
			depth--
		case token.SEMI, token.EOF:
			return false
		case token.IDENT:
			if depth == 0 && i > 0 && p.text(p.peekTok(i)) == "in" {
				return true
			}
		}
	}
	return false
}

// parseObjCForIn reads the rest of `for (T x in c) S` or `for (x in c) S`
// once the ( has been read.
func (p *parser) parseObjCForIn(start, kw ast.Tok) ast.Stmt {
	s := &ast.ObjCForInStmt{For: kw}
	p.pushNames()
	defer p.popNames()
	if p.isDeclStart() {
		dlo := p.pos()
		specs := p.parseDeclSpecs()
		d := p.parseDeclarator()
		p.noteDeclarator(specs, d)
		s.Decl = &ast.SimpleDecl{Span: ast.Span{Lo: dlo, Hi: p.pos()}, Specs: specs,
			Inits: []*ast.InitDeclarator{{Span: ast.Span{Lo: d.Pos(), Hi: d.End()}, Decl: d,
				Assign: ast.NoTok, Lparen: ast.NoTok, Rparen: ast.NoTok}}, Semi: ast.NoTok}
	} else {
		s.X = p.parseAssignmentExpr()
	}
	if p.peek() == token.IDENT && p.text(p.cur) == "in" {
		p.next()
	} else {
		p.error(p.cur, "expected 'in'")
	}
	s.Coll = p.parseExpr()
	p.expect(token.RPAREN)
	s.Body = p.parseStmt()
	s.Span = ast.Span{Lo: start, Hi: p.pos()}
	return s
}

// moveTypeAttrs moves the attributes that qualify a type -- ARC's
// objc_ownership, what `__weak Obj *w` begins with -- from those written
// before a declaration to its specifiers, where the type is built.
func (p *parser) moveTypeAttrs(attrs []*ast.AttrGroup, specs *ast.DeclSpecs) []*ast.AttrGroup {
	if !p.objc() || specs == nil {
		return attrs
	}
	kept := attrs[:0:0]
	for _, g := range attrs {
		typeAttr := false
		for _, at := range g.Attrs {
			if at != nil && at.Name != nil && at.Name.Text(p.u) == "objc_ownership" {
				typeAttr = true
			}
		}
		if typeAttr {
			specs.Attrs = append(specs.Attrs, g)
			continue
		}
		kept = append(kept, g)
	}
	return kept
}
