package ast

import "github.com/vertex-language/vcx/token"

// The Objective-C half of Objective-C++: what a .mm unit adds to C++. Every
// node here is built only when the parser runs in its Objective-C mode; a
// C++ unit never has one.
//
// An Objective-C keyword is two tokens, @ and a name, and a node records
// the position of the @ as its Keyword.

// ---- declarations ----

// ObjCInterfaceDecl is `@interface Name<T> : Super<U> <P> { ivars } ... @end`,
// a category `@interface Name (Cat) <P> ... @end`, or a class extension,
// `@interface Name () ... @end`.
type ObjCInterfaceDecl struct {
	Span
	Attrs      []*AttrGroup
	Keyword    Tok
	Name       *Ident
	TypeParams []*ObjCTypeParam

	// Category is set for a category or an extension: Lparen is the
	// position of its parenthesis, and Category its name, nil for an
	// extension.
	Lparen   Tok
	Category *Ident

	Super     *Ident // nil for a root class, and for a category
	SuperArgs []*TypeId
	Protocols []*Ident
	Ivars     []Decl
	Members   []Decl
	AtEnd     Tok
}

// IsCategory reports whether the declaration is a category or an extension.
func (d *ObjCInterfaceDecl) IsCategory() bool { return d.Lparen.IsValid() }

// ObjCTypeParam is one parameter of a generic class, `__covariant T : B *`.
type ObjCTypeParam struct {
	Span
	Variance string // "__covariant", "__contravariant", or ""
	Name     *Ident
	Bound    *TypeId
}

// ObjCImplDecl is `@implementation Name { ivars } ... @end`, or a
// category's implementation, `@implementation Name (Cat) ... @end`.
type ObjCImplDecl struct {
	Span
	Attrs    []*AttrGroup
	Keyword  Tok
	Name     *Ident
	Category *Ident // nil for a class's implementation
	Super    *Ident // restated superclass, rarely written
	Ivars    []Decl
	Members  []Decl
	AtEnd    Tok
}

// ObjCProtocolDecl is `@protocol Name <Q> ... @end`.
type ObjCProtocolDecl struct {
	Span
	Attrs     []*AttrGroup
	Keyword   Tok
	Name      *Ident
	Protocols []*Ident
	Members   []Decl
	AtEnd     Tok
}

// ObjCForwardDecl is `@class A, B<T>;` or `@protocol A, B;`.
type ObjCForwardDecl struct {
	Span
	Keyword  Tok
	Protocol bool // @protocol rather than @class
	Names    []*Ident
	Semi     Tok
}

// ObjCAliasDecl is `@compatibility_alias Alias Class;`.
type ObjCAliasDecl struct {
	Span
	Keyword Tok
	Alias   *Ident
	Class   *Ident
	Semi    Tok
}

// ObjCMarkerDecl is a member that marks what follows it: @required and
// @optional in a protocol, and @private, @protected, @public and @package
// among instance variables. Word is the name after the @.
type ObjCMarkerDecl struct {
	Span
	Keyword Tok
	Word    string
}

// ObjCMethodDecl is a method's declaration, or its definition where Body is
// set: `- (T)name`, `+ (T)do:(A)a with:(B)b, ...`.
type ObjCMethodDecl struct {
	Span
	Sign     token.Kind // SUB for an instance method, ADD for a class method
	Result   *TypeId    // nil when not written: id
	Attrs    []*AttrGroup
	Parts    []*ObjCSelPart // one part with no colon for a unary method
	Params   []*ParamDecl   // the C parameters after `, ` of a variadic method
	Vararg   Tok
	TailAttr []*AttrGroup
	Body     *CompoundStmt
	Semi     Tok
}

// IsClassMethod reports whether the method was written with +.
func (d *ObjCMethodDecl) IsClassMethod() bool { return d.Sign == token.ADD }

// ObjCSelPart is one `name : (T) param` of a method's selector, or the
// whole selector of a unary method, which has no colon. Name is nil for a
// part written without one, as in `do:(int)a :(int)b`.
type ObjCSelPart struct {
	Span
	Name  *Ident
	Colon Tok // NoTok in a unary selector
	Type  *TypeId
	Attrs []*AttrGroup
	Param *Ident
}

// ObjCPropertyDecl is `@property (attrs) T name;`.
type ObjCPropertyDecl struct {
	Span
	Keyword Tok
	Attrs   []*ObjCPropertyAttr
	Specs   *DeclSpecs
	Decls   []Declarator
	Semi    Tok
}

// ObjCPropertyAttr is one of a property's attributes: nonatomic, copy,
// getter=name, setter=name:.
type ObjCPropertyAttr struct {
	Span
	Name     *Ident
	Selector string // getter= and setter='s selector, colon included
}

// ObjCPropertyImplDecl is `@synthesize a = _a, b;` or `@dynamic a;`.
type ObjCPropertyImplDecl struct {
	Span
	Keyword Tok
	Dynamic bool
	Names   []*Ident
	Ivars   []*Ident // one per name, nil where none was written
	Semi    Tok
}

// ---- types ----

// ObjCTypeSpec is an Objective-C object type as a specifier: a class name,
// id, Class or instancetype, with the angle-bracket lists a class may take
// -- `NSArray<NSString *> <NSCopying>`, `id<NSCopying>`. Kindof is
// `__kindof`.
type ObjCTypeSpec struct {
	Span
	Name      *Ident
	TypeArgs  []*TypeId
	Protocols []*Ident
	Kindof    bool
}

func (*ObjCTypeSpec) declSpecNode() {}

// ---- expressions ----

// ObjCMessageExpr is `[receiver selector]` or `[receiver key:arg key:arg]`.
// Recv is an expression, an *ObjCSuperExpr, or an *ObjCClassRecv.
type ObjCMessageExpr struct {
	Span
	Lbrack Tok
	Recv   Expr
	Parts  []*ObjCMsgPart
	Rbrack Tok
}

// ObjCMsgPart is one `name: arg` of a message, or the unary selector. The
// last part of a variadic method's message carries its extra arguments.
type ObjCMsgPart struct {
	Span
	Name  *Ident
	Colon Tok
	Args  []Expr
}

// ObjCSuperExpr is super as a message's receiver.
type ObjCSuperExpr struct {
	Span
}

// ObjCClassRecv is a class name as a message's receiver, with the type
// arguments `[NSArray<NSString *> array]` may write.
type ObjCClassRecv struct {
	Span
	Name     *Ident
	TypeArgs []*TypeId
}

// ObjCStringLit is @"text", with any string literals adjacent to it
// concatenated: @"a" "b".
type ObjCStringLit struct {
	Span
	At  Tok
	Str *StringLit
}

// ObjCSelectorExpr is @selector(name:with:).
type ObjCSelectorExpr struct {
	Span
	Keyword Tok
	Name    string
}

// ObjCProtocolExpr is @protocol(Name).
type ObjCProtocolExpr struct {
	Span
	Keyword Tok
	Name    *Ident
}

// ObjCEncodeExpr is @encode(T).
type ObjCEncodeExpr struct {
	Span
	Keyword Tok
	Type    *TypeId
}

// ObjCBoxedExpr is @(e), @42, @-1, @'c', @YES.
type ObjCBoxedExpr struct {
	Span
	At Tok
	X  Expr
}

// ObjCArrayLit is @[a, b].
type ObjCArrayLit struct {
	Span
	At    Tok
	Elems []Expr
}

// ObjCDictLit is @{k: v, ...}.
type ObjCDictLit struct {
	Span
	At     Tok
	Keys   []Expr
	Values []Expr
}

// ObjCBoolLit is __objc_yes or __objc_no, which YES and NO expand to.
type ObjCBoolLit struct {
	Span
	Value bool
}

// ObjCAvailableExpr is @available(macos 10.15, *) or __builtin_available.
type ObjCAvailableExpr struct {
	Span
	Keyword Tok
	// Platforms maps a platform named in the check to its version.
	Platforms map[string]string `ast:"-"`
}

// ObjCBridgeCast is ARC's cast between an object pointer and a C pointer:
// `(__bridge T)x`, `(__bridge_retained T)x` (the C pointer takes a
// reference) and `(__bridge_transfer T)x` (the object takes it over).
type ObjCBridgeCast struct {
	Span
	Lparen Tok
	Kind   string // "__bridge", "__bridge_retained" or "__bridge_transfer"
	Type   *TypeId
	X      Expr
}

// BlockExpr is a block literal, `^ R (params) { body }`: R and the
// parameters are optional.
type BlockExpr struct {
	Span
	Caret  Tok
	Result *TypeId // nil when not written: deduced from the body
	Lparen Tok     // NoTok when no parameter list was written
	Params []*ParamDecl
	Vararg Tok
	Body   *CompoundStmt
}

// ---- statements ----

// ObjCForInStmt is fast enumeration: `for (T x in c) S` or `for (x in c) S`.
type ObjCForInStmt struct {
	Span
	For  Tok
	Decl *SimpleDecl // the declaration form's loop variable, or nil
	X    Expr        // the expression form's, or nil
	Coll Expr
	Body Stmt
}

// ObjCTryStmt is @try { } @catch (T e) { } ... @finally { }.
type ObjCTryStmt struct {
	Span
	Keyword Tok
	Body    *CompoundStmt
	Catches []*ObjCCatch
	Finally *CompoundStmt
}

// ObjCCatch is one @catch clause; Param is nil for @catch (...).
type ObjCCatch struct {
	Span
	Keyword Tok
	Param   *ParamDecl
	Body    *CompoundStmt
}

// ObjCThrowStmt is @throw e; or, rethrowing, @throw;.
type ObjCThrowStmt struct {
	Span
	Keyword Tok
	X       Expr
}

// ObjCSyncStmt is @synchronized (e) { }.
type ObjCSyncStmt struct {
	Span
	Keyword Tok
	X       Expr
	Body    *CompoundStmt
}

// ObjCAutoreleaseStmt is @autoreleasepool { }.
type ObjCAutoreleaseStmt struct {
	Span
	Keyword Tok
	Body    *CompoundStmt
}

func (*ObjCInterfaceDecl) declNode()    {}
func (*ObjCImplDecl) declNode()         {}
func (*ObjCProtocolDecl) declNode()     {}
func (*ObjCForwardDecl) declNode()      {}
func (*ObjCAliasDecl) declNode()        {}
func (*ObjCMarkerDecl) declNode()       {}
func (*ObjCMethodDecl) declNode()       {}
func (*ObjCPropertyDecl) declNode()     {}
func (*ObjCPropertyImplDecl) declNode() {}
func (*ObjCTypeParam) declNode()        {}

func (*ObjCMessageExpr) exprNode()   {}
func (*ObjCSuperExpr) exprNode()     {}
func (*ObjCClassRecv) exprNode()     {}
func (*ObjCStringLit) exprNode()     {}
func (*ObjCSelectorExpr) exprNode()  {}
func (*ObjCProtocolExpr) exprNode()  {}
func (*ObjCEncodeExpr) exprNode()    {}
func (*ObjCBoxedExpr) exprNode()     {}
func (*ObjCArrayLit) exprNode()      {}
func (*ObjCDictLit) exprNode()       {}
func (*ObjCBoolLit) exprNode()       {}
func (*ObjCAvailableExpr) exprNode() {}
func (*BlockExpr) exprNode()         {}
func (*ObjCBridgeCast) exprNode()    {}

func (*ObjCForInStmt) stmtNode()       {}
func (*ObjCTryStmt) stmtNode()         {}
func (*ObjCThrowStmt) stmtNode()       {}
func (*ObjCSyncStmt) stmtNode()        {}
func (*ObjCAutoreleaseStmt) stmtNode() {}
