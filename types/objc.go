package types

import (
	"strings"
	"sync"
)

// Objective-C++'s types. An object is only ever reached through a pointer:
// `NSString *` is a Pointer to the ObjCInterface NSString, and `id` is a
// Pointer to the built-in interface ObjCIdObject. So every object pointer
// is a pointer, and what a pointer can do -- compare with nil, test, be
// stored -- an object pointer does too; what only an object pointer can do
// is found by asking ObjCClassOf.

// ObjCInterface is an Objective-C class: its superclass, what it conforms
// to, its instance variables, and the methods and properties its
// @interface, its categories and its class extension declare.
type ObjCInterface struct {
	Name  string
	Super *ObjCInterface

	// Builtin marks id, Class and instancetype: object types with no
	// class of their own.
	Builtin bool

	Protocols []*ObjCProtocol

	// Ivars are the instance variables, in declaration order.
	Ivars []ObjCIvar

	// Methods are the instance methods and ClassMethods the class
	// methods, by selector.
	Methods      map[string]*ObjCMethod
	ClassMethods map[string]*ObjCMethod
	Properties   map[string]*ObjCProperty

	// PropertyOrder is the properties' names in the order they were
	// declared -- interface, extensions and categories, implementation --
	// which is the order their instance variables are synthesized in.
	PropertyOrder []string

	// TypeParams are a generic class's parameters, which are erased: a
	// method written in terms of one is typed id.
	TypeParams []string

	// Declared is set once the class's @interface has been read, and
	// Implemented once an @implementation of it has.
	Declared    bool
	Implemented bool

	// Of is the built-in object a protocol-qualified one qualifies --
	// id for id<NSCopying>, whose Protocols are the qualification -- and
	// nil for any other.
	Of *ObjCInterface
}

// ObjCBase is the class c is: itself, or for a protocol-qualified id or
// Class the built-in one it qualifies.
func ObjCBase(c *ObjCInterface) *ObjCInterface {
	if c != nil && c.Of != nil {
		return c.Of
	}
	return c
}

var (
	qualifiedMu   sync.Mutex
	qualifiedObjC = map[string]*ObjCInterface{}
)

// ObjCQualified is base -- id or Class -- qualified by protocols:
// id<NSCopying>. One object per qualification, so that two spellings of
// the same type are the same type.
func ObjCQualified(base *ObjCInterface, ps []*ObjCProtocol) *ObjCInterface {
	if len(ps) == 0 {
		return base
	}
	key := base.Name
	for _, p := range ps {
		key += "<" + p.Name + ">"
	}
	qualifiedMu.Lock()
	defer qualifiedMu.Unlock()
	if c, ok := qualifiedObjC[key]; ok {
		return c
	}
	c := &ObjCInterface{Name: base.Name, Builtin: true, Protocols: ps, Of: base}
	qualifiedObjC[key] = c
	return c
}

// The built-in object types: what id, Class and instancetype point to.
var (
	ObjCIdObject           = &ObjCInterface{Name: "id", Builtin: true}
	ObjCClassObject        = &ObjCInterface{Name: "Class", Builtin: true}
	ObjCInstancetypeObject = &ObjCInterface{Name: "instancetype", Builtin: true}
)

// ObjCId is the type id, ObjCClassPtr the type Class, and ObjCInstancetype
// the type instancetype -- the receiver's own type, in a method's result.
var (
	ObjCId           Type = &Pointer{Elem: ObjCIdObject}
	ObjCClassPtr     Type = &Pointer{Elem: ObjCClassObject}
	ObjCInstancetype Type = &Pointer{Elem: ObjCInstancetypeObject}
)

// NewObjCInterface is a class of the name, with nothing declared yet.
func NewObjCInterface(name string) *ObjCInterface {
	return &ObjCInterface{
		Name:         name,
		Methods:      map[string]*ObjCMethod{},
		ClassMethods: map[string]*ObjCMethod{},
		Properties:   map[string]*ObjCProperty{},
	}
}

func (*ObjCInterface) Kind() Kind { return ObjCInterfaceKind }

func (c *ObjCInterface) String() string { return c.Name }

func (c *ObjCInterface) Equal(other Type) bool {
	o, ok := strictOther(other).(*ObjCInterface)
	return ok && o == c
}

// IsSubclassOf reports whether c is d or inherits from it.
func (c *ObjCInterface) IsSubclassOf(d *ObjCInterface) bool {
	for k := c; k != nil; k = k.Super {
		if k == d {
			return true
		}
	}
	return false
}

// LookupMethod finds the method a message with the selector reaches on an
// instance (class false) or on the class itself (class true): the class's
// own, then its superclasses', then the protocols any of them conforms to.
// A class object also answers its root class's instance methods, as
// +[NSObject respondsToSelector:] shows.
func (c *ObjCInterface) LookupMethod(sel string, class bool) *ObjCMethod {
	for k := c; k != nil; k = k.Super {
		table := k.Methods
		if class {
			table = k.ClassMethods
		}
		if m := table[sel]; m != nil {
			return m
		}
		for _, p := range k.Protocols {
			if m := p.LookupMethod(sel, class); m != nil {
				return m
			}
		}
	}
	if class {
		root := c
		for root.Super != nil {
			root = root.Super
		}
		if m := root.Methods[sel]; m != nil {
			return m
		}
	}
	return nil
}

// LookupProperty finds a property through the superclasses and protocols.
func (c *ObjCInterface) LookupProperty(name string) *ObjCProperty {
	for k := c; k != nil; k = k.Super {
		if p := k.Properties[name]; p != nil {
			return p
		}
		for _, proto := range k.Protocols {
			if p := proto.LookupProperty(name); p != nil {
				return p
			}
		}
	}
	return nil
}

// LookupIvar finds an instance variable, and the class that declares it.
func (c *ObjCInterface) LookupIvar(name string) (*ObjCIvar, *ObjCInterface) {
	for k := c; k != nil; k = k.Super {
		for i := range k.Ivars {
			if k.Ivars[i].Name == name {
				return &k.Ivars[i], k
			}
		}
	}
	return nil, nil
}

// ObjCIvar is one instance variable.
type ObjCIvar struct {
	Name string
	Type Type
	// Access is @private, @protected, @public or @package's.
	Access string
	// Synthesized marks the variable a property's @synthesize (or its
	// default synthesis) made.
	Synthesized bool
}

// ObjCProtocol is an Objective-C protocol.
type ObjCProtocol struct {
	Name         string
	Protocols    []*ObjCProtocol
	Methods      map[string]*ObjCMethod
	ClassMethods map[string]*ObjCMethod
	Properties   map[string]*ObjCProperty
	Declared     bool
}

// NewObjCProtocol is a protocol of the name, with nothing declared yet.
func NewObjCProtocol(name string) *ObjCProtocol {
	return &ObjCProtocol{
		Name:         name,
		Methods:      map[string]*ObjCMethod{},
		ClassMethods: map[string]*ObjCMethod{},
		Properties:   map[string]*ObjCProperty{},
	}
}

// LookupMethod finds a method of the protocol or of one it refines.
func (p *ObjCProtocol) LookupMethod(sel string, class bool) *ObjCMethod {
	table := p.Methods
	if class {
		table = p.ClassMethods
	}
	if m := table[sel]; m != nil {
		return m
	}
	for _, q := range p.Protocols {
		if m := q.LookupMethod(sel, class); m != nil {
			return m
		}
	}
	return nil
}

// LookupProperty finds a property of the protocol or one it refines.
func (p *ObjCProtocol) LookupProperty(name string) *ObjCProperty {
	if prop := p.Properties[name]; prop != nil {
		return prop
	}
	for _, q := range p.Protocols {
		if prop := q.LookupProperty(name); prop != nil {
			return prop
		}
	}
	return nil
}

// ObjCMethod is a method's declaration.
type ObjCMethod struct {
	Selector string
	Class    bool // a class method, +
	Result   Type
	Params   []Type
	Variadic bool
	// Instancetype says the result is instancetype: at a message, the
	// receiver's type.
	Instancetype bool
	// Family is the method family ARC reads a result's ownership from:
	// alloc, copy, init, mutableCopy or new, or "".
	Family string
	// ReturnsRetained is ns_returns_retained, what the families alloc,
	// copy, mutableCopy and new -- and init, which consumes its receiver
	// -- say of their result without it.
	ReturnsRetained bool
	// Owner is the class or protocol that declares it.
	Owner any
	// Optional is a protocol's @optional method.
	Optional bool
}

// FuncType is the method as the C function objc_msgSend is called as: the
// receiver and the selector, then its parameters.
func (m *ObjCMethod) FuncType() *Func {
	params := []Param{{Type: ObjCId}, {Type: &Pointer{Elem: Typ(Void)}}}
	for _, p := range m.Params {
		params = append(params, Param{Type: p})
	}
	return &Func{Ret: m.Result, Params: params, Variadic: m.Variadic}
}

// SelectorFamily is the ARC method family of a selector: its first word,
// where that is one of the five, followed by an upper-case letter or
// nothing -- `copyWithZone:` is copy's, `copyright` is no family's.
func SelectorFamily(sel string) string {
	s := strings.TrimLeft(sel, "_")
	for _, f := range []string{"alloc", "copy", "init", "mutableCopy", "new"} {
		if !strings.HasPrefix(s, f) {
			continue
		}
		rest := s[len(f):]
		if rest == "" || rest[0] == ':' || (rest[0] >= 'A' && rest[0] <= 'Z') {
			return f
		}
	}
	return ""
}

// ObjCProperty is a declared property.
type ObjCProperty struct {
	Name     string
	Type     Type
	Getter   string
	Setter   string
	Readonly bool
	Class    bool // a class property
	// Ownership is strong, weak, copy, assign or unsafe_unretained.
	Ownership string
	Atomic    bool
	// Ivar is the instance variable an @implementation synthesizes for it,
	// "" where it is @dynamic or has not been synthesized.
	Ivar string
}

// BlockPointer is a block's type, `R (^)(Params)`.
type BlockPointer struct {
	Func *Func
}

func (*BlockPointer) Kind() Kind { return BlockPointerKind }

func (b *BlockPointer) String() string {
	var ps []string
	for _, p := range b.Func.Params {
		ps = append(ps, p.Type.String())
	}
	return b.Func.Ret.String() + " (^)(" + strings.Join(ps, ", ") + ")"
}

func (b *BlockPointer) Equal(other Type) bool {
	o, ok := strictOther(other).(*BlockPointer)
	return ok && (b.Func == o.Func || b.Func.Equal(o.Func))
}

// ObjCClassOf is the class an object pointer points to, or nil for a type
// that is not an object pointer. id, Class and instancetype give their
// built-in objects.
func ObjCClassOf(t Type) *ObjCInterface {
	if t == nil {
		return nil
	}
	p, ok := Unqualify(t).(*Pointer)
	if !ok || p.Elem == nil {
		return nil
	}
	c, _ := Unqualify(p.Elem).(*ObjCInterface)
	return c
}

// IsObjCObjectPointer reports whether t is an object pointer.
func IsObjCObjectPointer(t Type) bool { return ObjCClassOf(t) != nil }

// IsObjCRetainable reports whether ARC manages a value of type t: an
// object pointer or a block.
func IsObjCRetainable(t Type) bool {
	if IsObjCObjectPointer(t) {
		return true
	}
	_, isBlock := Unqualify(t).(*BlockPointer)
	return isBlock
}

// Ownership is how ARC manages a value of type t: the ownership qualifier
// written on it, QObjCStrong for a retainable type with none, and 0 for a
// type ARC does not manage.
func Ownership(t Type) Qual {
	if !IsObjCRetainable(t) {
		return 0
	}
	if q := QualsOf(t) & QObjCOwnership; q != 0 {
		return q
	}
	return QObjCStrong
}

// WithOwnership is t with its ownership qualifier replaced by q.
func WithOwnership(t Type, q Qual) Type {
	if in, ok := t.(*Qualified); ok {
		rest := in.Q&^QObjCOwnership | q
		if rest == 0 {
			return in.T
		}
		return &Qualified{Q: rest, T: in.T}
	}
	return Qualify(t, q)
}

// OwnershipNamed is the qualifier objc_ownership(name) spells.
func OwnershipNamed(name string) Qual {
	switch name {
	case "strong":
		return QObjCStrong
	case "weak":
		return QObjCWeak
	case "none":
		return QObjCUnsafe
	case "autoreleasing":
		return QObjCAutoreleasing
	}
	return 0
}
