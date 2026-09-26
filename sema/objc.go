package sema

import (
	"fmt"
	"github.com/vertex-language/vcx/objcrt"
	"sort"
	"strings"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// Objective-C++: the classes, protocols, methods and properties an
// Objective-C++ unit declares, and what its method bodies, messages and
// literals mean. The tables are the unit's: every @interface read, from
// the SDK's headers or the file, adds to them, and a message is resolved
// against what they hold at the point it is written.

// ObjCTables are the unit's Objective-C declarations.
type ObjCTables struct {
	Classes   map[string]*types.ObjCInterface
	Protocols map[string]*types.ObjCProtocol

	// Instance and Class are every method declared anywhere, by selector:
	// what a message to id, or to a class the method is not declared on,
	// is resolved against.
	Instance map[string][]*types.ObjCMethod
	Class    map[string][]*types.ObjCMethod
}

func newObjCTables() *ObjCTables {
	return &ObjCTables{
		Classes:   map[string]*types.ObjCInterface{},
		Protocols: map[string]*types.ObjCProtocol{},
		Instance:  map[string][]*types.ObjCMethod{},
		Class:     map[string][]*types.ObjCMethod{},
	}
}

// ObjCInfo is what lowering needs of the unit's Objective-C: how each
// message, property reference, instance variable reference and literal
// was resolved, and the classes and categories it implements.
type ObjCInfo struct {
	Sends     map[*ast.ObjCMessageExpr]*ObjCSend
	Props     map[*ast.MemberExpr]*ObjCPropRef
	Ivars     map[ast.Expr]*ObjCIvarRef
	Subs      map[*ast.IndexExpr]*ObjCSubscript
	Literals  map[ast.Expr]*ObjCLiteral
	Impls     []*ObjCImpl
	Protocols []*types.ObjCProtocol // referenced by @protocol(P) or adopted by an implemented class
	// CatchParams are each @catch clause's parameter.
	CatchParams map[*ast.ObjCCatch]*VarSymbol
}

// ObjCSend is one resolved message.
type ObjCSend struct {
	Selector string
	Method   *types.ObjCMethod // nil where no declaration was found
	// Super is a message to super, and Class the receiver's class: the
	// class written, for a class message, and for super the class of the
	// method the message is in.
	Super      bool
	ClassRecv  bool
	Class      *types.ObjCInterface
	Result     types.Type
	ParamTypes []types.Type // what each argument converts to
}

// ObjCPropRef is a property named with dot syntax: its getter, or its
// setter where the reference is assigned to.
type ObjCPropRef struct {
	Prop   *types.ObjCProperty
	Getter string
	Setter string
	// ClassProp is a class property, whose receiver is a class name.
	ClassProp bool
	Class     *types.ObjCInterface
}

// ObjCIvarRef is an instance variable, named alone in a method (self's)
// or through ->.
type ObjCIvarRef struct {
	Class    *types.ObjCInterface
	Ivar     *types.ObjCIvar
	Implicit bool // written alone: self->ivar
}

// ObjCSubscript is `a[i]` on an object: the methods it sends.
type ObjCSubscript struct {
	Keyed  bool // a[key] rather than a[index]
	Getter string
	Setter string
}

// ObjCLiteral is a boxed expression, array or dictionary literal: the
// class and the method it is made with.
type ObjCLiteral struct {
	Class    string
	Selector string
	Arg      types.Type // what a boxed value converts to
}

// ObjCImpl is an @implementation.
type ObjCImpl struct {
	Class    *types.ObjCInterface
	Category string // "" for the class's own
	Methods  []*ObjCMethodImpl
	// Synthesized are the properties whose accessors the implementation
	// provides by synthesis, with the instance variable each is kept in.
	Synthesized []*types.ObjCProperty
	Decl        *ast.ObjCImplDecl
}

// ObjCMethodImpl is one method an @implementation defines.
type ObjCMethodImpl struct {
	Method *types.ObjCMethod
	Func   *FuncSymbol
}

// ObjCClassSymbol makes a class name a symbol for lookup's purposes: a
// name as a message's receiver, and what `[NSString alloc]`'s receiver is.
type ObjCIvarSymbol struct {
	SymName  string
	Class    *types.ObjCInterface
	Ivar     *types.ObjCIvar
	SymPos   ast.Tok
	SymScope *Scope
}

func (s *ObjCIvarSymbol) Name() string     { return s.SymName }
func (s *ObjCIvarSymbol) Type() types.Type { return s.Ivar.Type }
func (s *ObjCIvarSymbol) Pos() ast.Tok     { return s.SymPos }
func (s *ObjCIvarSymbol) Scope() *Scope    { return s.SymScope }

// objcMethodContext is the method whose body is being checked.
type objcMethodContext struct {
	class  *types.ObjCInterface
	method *types.ObjCMethod
	isCls  bool
	// self is the method's self parameter, which a block that reaches an
	// instance variable or super captures.
	self *VarSymbol
}

// objcTables is the unit's tables, made on first use.
func (a *Analyzer) objcTables() *ObjCTables {
	root := a.globalScope
	if root.ObjC == nil {
		root.ObjC = newObjCTables()
	}
	return root.ObjC
}

func (a *Analyzer) objcInfo() *ObjCInfo {
	if a.info.ObjC == nil {
		a.info.ObjC = NewObjCInfo()
	}
	return a.info.ObjC
}

// NewObjCInfo is an Objective-C++ unit's information with nothing in it
// yet.
func NewObjCInfo() *ObjCInfo {
	return &ObjCInfo{
		Sends:       map[*ast.ObjCMessageExpr]*ObjCSend{},
		Props:       map[*ast.MemberExpr]*ObjCPropRef{},
		Ivars:       map[ast.Expr]*ObjCIvarRef{},
		Subs:        map[*ast.IndexExpr]*ObjCSubscript{},
		Literals:    map[ast.Expr]*ObjCLiteral{},
		CatchParams: map[*ast.ObjCCatch]*VarSymbol{},
	}
}

// objcClass is the class of the name, made on first mention: a class a
// unit names before its @interface -- @class NSString -- is one class
// throughout.
func (a *Analyzer) objcClass(name string) *types.ObjCInterface {
	t := a.objcTables()
	c := t.Classes[name]
	if c == nil {
		c = types.NewObjCInterface(name)
		t.Classes[name] = c
	}
	return c
}

func (a *Analyzer) objcProtocol(name string) *types.ObjCProtocol {
	t := a.objcTables()
	p := t.Protocols[name]
	if p == nil {
		p = types.NewObjCProtocol(name)
		t.Protocols[name] = p
	}
	return p
}

// objcSpecType is the type an Objective-C object type specifier names.
func objcSpecType(s *ast.ObjCTypeSpec, scope *Scope, u ast.Unit, info *DeclSpecInfo) types.Type {
	name := s.Name.Text(u)
	switch name {
	case "id", "Class":
		// id<NSCopying>: the protocols are part of the type.
		base := types.ObjCIdObject
		if name == "Class" {
			base = types.ObjCClassObject
		}
		var ps []*types.ObjCProtocol
		if t := scope.root().ObjC; t != nil {
			for _, p := range s.Protocols {
				if proto := t.Protocols[p.Text(u)]; proto != nil {
					ps = append(ps, proto)
				}
			}
		}
		if len(ps) == 0 {
			return &types.Pointer{Elem: base}
		}
		return &types.Pointer{Elem: types.ObjCQualified(base, ps)}
	case "instancetype":
		return types.ObjCInstancetype
	}
	// A generic class's parameter is erased to id.
	for _, sym := range LookupUnqualified(scope, name) {
		if ts, isType := sym.(*TypeSymbol); isType {
			return ts.SymType
		}
	}
	if t := scope.root().ObjC; t != nil {
		if c := t.Classes[name]; c != nil {
			return c
		}
	}
	if info.Unresolved == "" {
		info.Unresolved = name
		info.UnresolvedPos = s.Name.Pos()
	}
	return types.ObjCIdObject
}

// objcTypeOf is a method's written type, id where none was written.
func (a *Analyzer) objcTypeOf(t *ast.TypeId) types.Type {
	if t == nil {
		return types.ObjCId
	}
	if ty := a.noteTypeId(t); ty != nil {
		return ty
	}
	return types.ObjCId
}

// ---- declarations ----

func (a *Analyzer) checkObjCDecl(decl ast.Decl) {
	switch d := decl.(type) {
	case *ast.ObjCForwardDecl:
		for _, n := range d.Names {
			if d.Protocol {
				a.objcProtocol(n.Text(a.unit))
			} else {
				a.objcClass(n.Text(a.unit))
			}
		}
	case *ast.ObjCAliasDecl:
		if d.Alias != nil && d.Class != nil {
			a.objcTables().Classes[d.Alias.Text(a.unit)] = a.objcClass(d.Class.Text(a.unit))
		}
	case *ast.ObjCProtocolDecl:
		a.checkObjCProtocol(d)
	case *ast.ObjCInterfaceDecl:
		a.checkObjCInterface(d)
	case *ast.ObjCImplDecl:
		a.checkObjCImpl(d)
	}
}

// objcParamScope is a scope in which a generic class's parameters name id.
func (a *Analyzer) objcParamScope(c *types.ObjCInterface) *Scope {
	s := NewScope(a.curScope, BlockScope, nil)
	for _, p := range c.TypeParams {
		s.Insert(&TypeSymbol{SymName: p, SymType: types.ObjCId, SymScope: s})
	}
	return s
}

func (a *Analyzer) checkObjCProtocol(d *ast.ObjCProtocolDecl) {
	p := a.objcProtocol(d.Name.Text(a.unit))
	p.Declared = true
	for _, q := range d.Protocols {
		p.Protocols = append(p.Protocols, a.objcProtocol(q.Text(a.unit)))
	}
	a.checkObjCMembers(d.Members, p.Methods, p.ClassMethods, p.Properties, p)
}

func (a *Analyzer) checkObjCInterface(d *ast.ObjCInterfaceDecl) {
	c := a.objcClass(d.Name.Text(a.unit))
	if !d.IsCategory() {
		c.Declared = true
		if d.Super != nil {
			c.Super = a.objcClass(d.Super.Text(a.unit))
		}
		if len(d.TypeParams) > 0 {
			c.TypeParams = nil
			for _, tp := range d.TypeParams {
				if tp.Name != nil {
					c.TypeParams = append(c.TypeParams, tp.Name.Text(a.unit))
				}
			}
		}
	}
	for _, q := range d.Protocols {
		c.Protocols = append(c.Protocols, a.objcProtocol(q.Text(a.unit)))
	}
	old := a.curScope
	a.curScope = a.objcParamScope(c)
	defer func() { a.curScope = old }()
	a.checkObjCIvars(c, d.Ivars)
	a.checkObjCMembers(d.Members, c.Methods, c.ClassMethods, c.Properties, c)
}

// checkObjCIvars adds an @interface's or @implementation's instance
// variables to the class.
func (a *Analyzer) checkObjCIvars(c *types.ObjCInterface, decls []ast.Decl) {
	access := "@protected"
	for _, d := range decls {
		switch d := d.(type) {
		case *ast.ObjCMarkerDecl:
			access = "@" + d.Word
		case *ast.SimpleDecl:
			info := BuildDeclSpecs(d.Specs, a.curScope, a.unit)
			for _, init := range d.Inits {
				decl := init.Decl
				if bf, isBitfield := decl.(*ast.BitfieldDeclarator); isBitfield {
					decl = bf.Inner
				}
				t := BuildDeclarator(decl, info.Type, a.curScope, a.unit)
				if decl == nil || decl.DeclName() == nil {
					continue
				}
				c.Ivars = append(c.Ivars, types.ObjCIvar{Name: NameString(decl.DeclName(), a.unit), Type: t, Access: access})
			}
		}
	}
}

// checkObjCMembers declares the methods and properties of an @interface,
// a category or a @protocol, and checks the C++ declarations among them.
func (a *Analyzer) checkObjCMembers(members []ast.Decl, inst, class map[string]*types.ObjCMethod,
	props map[string]*types.ObjCProperty, owner any) {
	optional := false
	for _, m := range members {
		switch m := m.(type) {
		case *ast.ObjCMarkerDecl:
			optional = m.Word == "optional"
		case *ast.ObjCMethodDecl:
			method := a.objcMethodOf(m, owner)
			method.Optional = optional
			table := inst
			if method.Class {
				table = class
			}
			if _, have := table[method.Selector]; !have {
				table[method.Selector] = method
			}
			a.noteObjCMethod(method)
		case *ast.ObjCPropertyDecl:
			for _, p := range a.objcPropertiesOf(m) {
				if c, isClass := owner.(*types.ObjCInterface); isClass && props[p.Name] == nil {
					c.PropertyOrder = append(c.PropertyOrder, p.Name)
				}
				props[p.Name] = p
				// A property declares its accessors, unless a method of
				// the selector was declared already.
				getter := &types.ObjCMethod{Selector: p.Getter, Class: p.Class, Result: p.Type, Owner: owner}
				table := inst
				if p.Class {
					table = class
				}
				if _, have := table[getter.Selector]; !have {
					table[getter.Selector] = getter
					a.noteObjCMethod(getter)
				}
				if !p.Readonly {
					setter := &types.ObjCMethod{Selector: p.Setter, Class: p.Class, Result: types.Typ(types.Void),
						Params: []types.Type{p.Type}, Owner: owner}
					if _, have := table[setter.Selector]; !have {
						table[setter.Selector] = setter
						a.noteObjCMethod(setter)
					}
				}
			}
		case *ast.ObjCPropertyImplDecl, *ast.EmptyDecl:
		default:
			// A C++ declaration inside an @interface is the enclosing
			// scope's.
			a.CheckDecl(m)
		}
	}
}

func (a *Analyzer) noteObjCMethod(m *types.ObjCMethod) {
	t := a.objcTables()
	if m.Class {
		t.Class[m.Selector] = append(t.Class[m.Selector], m)
	} else {
		t.Instance[m.Selector] = append(t.Instance[m.Selector], m)
	}
}

// objcSelectorOf is a method declaration's selector: `name`, or
// `key:with:`.
func (a *Analyzer) objcSelectorOf(d *ast.ObjCMethodDecl) string {
	var b strings.Builder
	for _, p := range d.Parts {
		if p.Name != nil {
			b.WriteString(p.Name.Text(a.unit))
		}
		if p.Colon.IsValid() {
			b.WriteByte(':')
		}
	}
	return b.String()
}

// objcMethodOf is a method declaration's types.
func (a *Analyzer) objcMethodOf(d *ast.ObjCMethodDecl, owner any) *types.ObjCMethod {
	m := &types.ObjCMethod{
		Selector: a.objcSelectorOf(d),
		Class:    d.IsClassMethod(),
		Result:   a.objcTypeOf(d.Result),
		Variadic: d.Vararg.IsValid(),
		Owner:    owner,
	}
	if types.ObjCClassOf(m.Result) == types.ObjCInstancetypeObject {
		m.Instancetype = true
	}
	for _, p := range d.Parts {
		if p.Colon.IsValid() {
			m.Params = append(m.Params, a.objcParamType(a.objcTypeOf(p.Type)))
		}
	}
	for _, p := range d.Params {
		info := BuildDeclSpecs(p.Specs, a.curScope, a.unit)
		m.Params = append(m.Params, a.objcParamType(BuildDeclarator(p.Decl, info.Type, a.curScope, a.unit)))
	}
	m.Family = types.SelectorFamily(m.Selector)
	if m.Family == "init" && !m.Class && m.Result != nil && types.IsObjCObjectPointer(m.Result) && types.ObjCClassOf(m.Result) == types.ObjCIdObject {
		// An init method's id result is the receiver's type, as
		// instancetype is.
		m.Instancetype = true
	}
	switch m.Family {
	case "alloc", "copy", "mutableCopy", "new", "init":
		m.ReturnsRetained = types.IsObjCRetainable(m.Result)
	}
	if hasAttr(d.Attrs, "ns_returns_retained", a.unit) || hasAttr(d.TailAttr, "ns_returns_retained", a.unit) {
		m.ReturnsRetained = true
	}
	if hasAttr(d.Attrs, "ns_returns_not_retained", a.unit) || hasAttr(d.TailAttr, "ns_returns_not_retained", a.unit) {
		m.ReturnsRetained = false
	}
	return m
}

// objcParamType adjusts a parameter's type as a C function's is: an array
// or a function is passed as a pointer.
func (a *Analyzer) objcParamType(t types.Type) types.Type {
	t = objcIndirectParam(t)
	switch u := types.Unqualify(t).(type) {
	case *types.Array:
		return &types.Pointer{Elem: u.Elem}
	case *types.Func:
		return &types.Pointer{Elem: u}
	}
	return t
}

func hasAttr(groups []*ast.AttrGroup, name string, u ast.Unit) bool {
	for _, g := range groups {
		for _, at := range g.Attrs {
			if at.Name != nil && strings.Trim(at.Name.Text(u), "_") == strings.Trim(name, "_") {
				return true
			}
		}
	}
	return false
}

// objcPropertiesOf is a @property declaration's properties.
func (a *Analyzer) objcPropertiesOf(d *ast.ObjCPropertyDecl) []*types.ObjCProperty {
	info := BuildDeclSpecs(d.Specs, a.curScope, a.unit)
	var out []*types.ObjCProperty
	for _, dcl := range d.Decls {
		if dcl == nil || dcl.DeclName() == nil {
			continue
		}
		name := NameString(dcl.DeclName(), a.unit)
		p := &types.ObjCProperty{
			Name:   name,
			Type:   BuildDeclarator(dcl, info.Type, a.curScope, a.unit),
			Getter: name,
			Setter: "set" + strings.ToUpper(name[:1]) + name[1:] + ":",
			Atomic: true,
		}
		if types.IsObjCRetainable(p.Type) {
			p.Ownership = "strong"
		} else {
			p.Ownership = "assign"
		}
		for _, at := range d.Attrs {
			switch w := at.Name.Text(a.unit); w {
			case "readonly":
				p.Readonly = true
			case "readwrite":
				p.Readonly = false
			case "class":
				p.Class = true
			case "nonatomic":
				p.Atomic = false
			case "atomic":
				p.Atomic = true
			case "getter":
				p.Getter = at.Selector
			case "setter":
				p.Setter = at.Selector
			case "strong", "retain":
				p.Ownership = "strong"
			case "weak", "copy", "assign", "unsafe_unretained":
				p.Ownership = w
			}
		}
		out = append(out, p)
	}
	return out
}

// ---- implementations ----

func (a *Analyzer) checkObjCImpl(d *ast.ObjCImplDecl) {
	c := a.objcClass(d.Name.Text(a.unit))
	impl := &ObjCImpl{Class: c, Decl: d}
	if d.Category != nil {
		impl.Category = d.Category.Text(a.unit)
	} else {
		c.Implemented = true
		if d.Super != nil && c.Super == nil {
			c.Super = a.objcClass(d.Super.Text(a.unit))
		}
		if !c.Declared {
			c.Declared = true
		}
	}
	old := a.curScope
	a.curScope = a.objcParamScope(c)
	defer func() { a.curScope = old }()
	a.checkObjCIvars(c, d.Ivars)

	// Properties: what @synthesize and @dynamic say, and default
	// synthesis for the rest of the class's own.
	explicit := map[string]string{}
	dynamic := map[string]bool{}
	for _, m := range d.Members {
		if pi, ok := m.(*ast.ObjCPropertyImplDecl); ok {
			for i, n := range pi.Names {
				name := n.Text(a.unit)
				if pi.Dynamic {
					dynamic[name] = true
					continue
				}
				ivar := name
				if pi.Ivars[i] != nil {
					ivar = pi.Ivars[i].Text(a.unit)
				}
				explicit[name] = ivar
			}
		}
	}
	if impl.Category == "" {
		defined := map[string]bool{}
		for _, m := range d.Members {
			if md, ok := m.(*ast.ObjCMethodDecl); ok && md.Body != nil {
				defined[a.objcSelectorOf(md)] = true
			}
		}
		firstSynth := len(c.Ivars)
		for _, name := range objcPropertyOrder(c) {
			p := c.Properties[name]
			if p.Class || dynamic[name] {
				continue
			}
			ivar, isExplicit := explicit[name]
			if !isExplicit {
				// Default synthesis, unless every accessor was written.
				if defined[p.Getter] && (p.Readonly || defined[p.Setter]) {
					continue
				}
				ivar = "_" + name
			}
			p.Ivar = ivar
			if iv, _ := c.LookupIvar(ivar); iv == nil {
				c.Ivars = append(c.Ivars, types.ObjCIvar{Name: ivar, Type: objcSynthesizedIvarType(p), Access: "@private", Synthesized: true})
			}
			impl.Synthesized = append(impl.Synthesized, p)
		}
		// Among themselves, synthesized variables go smallest first, in
		// declaration order where sizes tie: clang's layout, which code
		// built by the two has to share.
		synth := c.Ivars[firstSynth:]
		sort.SliceStable(synth, func(i, j int) bool {
			si, _ := a.model.Sizeof(synth[i].Type)
			sj, _ := a.model.Sizeof(synth[j].Type)
			return si < sj
		})
	}

	for _, m := range d.Members {
		switch m := m.(type) {
		case *ast.ObjCMethodDecl:
			if m.Body == nil {
				continue
			}
			if mi := a.checkObjCMethodBody(c, impl.Category, m); mi != nil {
				impl.Methods = append(impl.Methods, mi)
			}
		case *ast.ObjCPropertyDecl:
			// A property declared in the @implementation is the class's.
			for _, p := range a.objcPropertiesOf(m) {
				if c.Properties[p.Name] == nil {
					c.PropertyOrder = append(c.PropertyOrder, p.Name)
				}
				c.Properties[p.Name] = p
			}
		case *ast.ObjCPropertyImplDecl, *ast.ObjCMarkerDecl, *ast.EmptyDecl:
		default:
			a.CheckDecl(m)
		}
	}
	a.objcInfo().Impls = append(a.objcInfo().Impls, impl)
}

func sortedPropNames(m map[string]*types.ObjCProperty) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}

// objcSelType is SEL: the runtime header's typedef, or a pointer where
// the unit has not read one.
func (a *Analyzer) objcSelType() types.Type {
	for _, sym := range a.globalScope.LookupLocal("SEL") {
		if ts, ok := sym.(*TypeSymbol); ok {
			return ts.SymType
		}
	}
	return &types.Pointer{Elem: types.Typ(types.Void)}
}

// checkObjCMethodBody checks a method definition as the C function it is
// compiled to -- `-[Class sel](self, _cmd, params...)`, internal to the
// unit -- with the class's instance variables in scope.
func (a *Analyzer) checkObjCMethodBody(c *types.ObjCInterface, category string, d *ast.ObjCMethodDecl) *ObjCMethodImpl {
	sel := a.objcSelectorOf(d)
	method := c.LookupMethod(sel, d.IsClassMethod())
	own := a.objcMethodOf(d, c)
	if method == nil || method.Owner != any(c) && !ownedByCategory(method, c) {
		// A method the interface did not declare is the implementation's.
		table := c.Methods
		if own.Class {
			table = c.ClassMethods
		}
		if table[sel] == nil {
			table[sel] = own
			a.noteObjCMethod(own)
		}
		if method == nil {
			method = own
		}
	}

	sign := "-"
	selfType := types.Type(&types.Pointer{Elem: c})
	if d.IsClassMethod() {
		sign = "+"
		selfType = types.ObjCClassPtr
	}
	name := sign + "[" + c.Name
	if category != "" {
		name += "(" + category + ")"
	}
	name += " " + sel + "]"

	params := []types.Param{{Name: "self", Type: selfType}, {Name: "_cmd", Type: a.objcSelType()}}
	i := 0
	for _, p := range d.Parts {
		if !p.Colon.IsValid() {
			continue
		}
		pname := ""
		if p.Param != nil {
			pname = p.Param.Text(a.unit)
		}
		t := types.ObjCId
		if i < len(own.Params) {
			t = own.Params[i]
		}
		params = append(params, types.Param{Name: pname, Type: t})
		i++
	}
	for _, p := range d.Params {
		pname := ""
		if p.Decl != nil && p.Decl.DeclName() != nil {
			pname = NameString(p.Decl.DeclName(), a.unit)
		}
		t := types.ObjCId
		if i < len(own.Params) {
			t = own.Params[i]
		}
		params = append(params, types.Param{Name: pname, Type: t})
		i++
	}
	result := own.Result
	if own.Instancetype {
		result = selfType
		if d.IsClassMethod() {
			result = &types.Pointer{Elem: c}
		}
	}
	fn := &FuncSymbol{
		SymName:  name,
		FuncType: &types.Func{Ret: result, Params: params, Variadic: own.Variadic},
		SymPos:   d.Pos(),
		SymScope: a.globalScope,
		Body:     d.Body,
		Internal: true,
		AsmLabel: objcrt.MethodSymbol(c.Name, category, sel, d.IsClassMethod()),
	}
	a.functions = append(a.functions, fn)

	// The class's instance variables, then the function's own scope.
	ivars := NewScope(a.curScope, BlockScope, nil)
	for k := c; k != nil; k = k.Super {
		for j := range k.Ivars {
			iv := &k.Ivars[j]
			if len(ivars.LookupLocal(iv.Name)) == 0 {
				ivars.Insert(&ObjCIvarSymbol{SymName: iv.Name, Class: k, Ivar: iv, SymScope: ivars})
			}
		}
	}
	fnScope := NewScope(ivars, FunctionScope, fn)
	oldScope, oldFunc, oldCtx := a.curScope, a.curFunc, a.objcMethod
	a.curScope, a.curFunc = fnScope, fn
	a.objcMethod = &objcMethodContext{class: c, method: method, isCls: d.IsClassMethod()}
	fn.Params = a.declareParams(fn.FuncType, fnScope)
	if len(fn.Params) > 0 {
		a.objcMethod.self = fn.Params[0]
	}
	a.CheckStmt(d.Body)
	a.curScope, a.curFunc, a.objcMethod = oldScope, oldFunc, oldCtx
	return &ObjCMethodImpl{Method: method, Func: fn}
}

func ownedByCategory(m *types.ObjCMethod, c *types.ObjCInterface) bool {
	oc, ok := m.Owner.(*types.ObjCInterface)
	return ok && oc == c
}

// objcIvarExpr is an instance variable named alone in a method: self's.
func (a *Analyzer) objcIvarExpr(e ast.Expr, s *ObjCIvarSymbol) ExprInfo {
	a.objcInfo().Ivars[e] = &ObjCIvarRef{Class: s.Class, Ivar: s.Ivar, Implicit: true}
	a.objcCaptureSelf()
	return ExprInfo{Type: s.Ivar.Type, ValCat: LValue}
}

var _ = fmt.Sprintf
var _ = token.AT

// ---- statements ----

func (a *Analyzer) checkObjCStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.ObjCAutoreleaseStmt:
		a.CheckStmt(s.Body)
	case *ast.ObjCSyncStmt:
		a.objcObjectOperand(s.X, "@synchronized")
		a.CheckStmt(s.Body)
	case *ast.ObjCThrowStmt:
		if s.X != nil {
			a.objcObjectOperand(s.X, "@throw")
		}
	case *ast.ObjCTryStmt:
		a.CheckStmt(s.Body)
		for _, c := range s.Catches {
			saved := a.curScope
			a.curScope = NewScope(saved, BlockScope, nil)
			if p := c.Param; p != nil && p.Decl != nil {
				info := BuildDeclSpecs(p.Specs, a.curScope, a.unit)
				t := BuildDeclarator(p.Decl, info.Type, a.curScope, a.unit)
				if name := NameString(p.Decl.DeclName(), a.unit); name != "" && t != nil {
					sym := &VarSymbol{SymName: name, SymType: t, SymPos: c.Pos(), SymScope: a.curScope, Defined: true}
					a.curScope.Insert(sym)
					a.objcInfo().CatchParams[c] = sym
					a.recordDef(&ast.InitDeclarator{Span: ast.Span{Lo: p.Pos(), Hi: p.End()}, Decl: p.Decl}, sym)
				}
			}
			a.CheckStmt(c.Body)
			a.curScope = saved
		}
		if s.Finally != nil {
			a.CheckStmt(s.Finally)
		}
	case *ast.ObjCForInStmt:
		saved := a.curScope
		a.curScope = NewScope(saved, BlockScope, nil)
		defer func() { a.curScope = saved }()
		if s.Decl != nil {
			a.CheckDecl(s.Decl)
		} else if s.X != nil {
			a.CheckExpr(s.X)
		}
		a.objcObjectOperand(s.Coll, "fast enumeration")
		a.loopDepth++
		a.CheckStmt(s.Body)
		a.loopDepth--
	}
}

// objcObjectOperand checks an operand that must be an object.
func (a *Analyzer) objcObjectOperand(x ast.Expr, what string) {
	info := a.CheckExpr(x)
	if !types.IsObjCRetainable(types.Decay(types.RemoveReference(info.Type))) && !isDependentExpr(info) {
		a.errorAt(x.Pos(), fmt.Sprintf("%s takes an object, not %q", what, info.Type))
	}
}

// objcSynthesizedIvarType is the type of the instance variable a property
// is synthesized into: the property's, owned as the property says.
func objcSynthesizedIvarType(p *types.ObjCProperty) types.Type {
	if !types.IsObjCRetainable(p.Type) || types.QualsOf(p.Type)&types.QObjCOwnership != 0 {
		return p.Type
	}
	switch p.Ownership {
	case "weak":
		return types.WithOwnership(p.Type, types.QObjCWeak)
	case "assign", "unsafe_unretained":
		return types.WithOwnership(p.Type, types.QObjCUnsafe)
	}
	return p.Type
}

// objcPropertyOrder is a class's properties in declaration order, then any
// the order missed by name.
func objcPropertyOrder(c *types.ObjCInterface) []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range c.PropertyOrder {
		if !seen[n] && c.Properties[n] != nil {
			seen[n] = true
			out = append(out, n)
		}
	}
	for _, n := range sortedPropNames(c.Properties) {
		if !seen[n] {
			out = append(out, n)
		}
	}
	return out
}
