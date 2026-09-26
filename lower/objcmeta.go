package lower

import (
	"sort"
	"strconv"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/objcrt"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// The metadata: what the runtime walks at load, and what the methods it
// finds there were compiled into.
//
// A class is not code. It is a pair of structures -- the class object and
// its metaclass, each pointing at a read-only half -- plus a method list,
// an ivar list, a property list, an offset variable per instance
// variable, and an entry in a section the runtime scans. Every list is
// emitted whether or not the program mentions it: the runtime reaches it
// by walking sections, not through any call.

// objcMetaType interns a metadata structure as a named struct type.
func (u *unit) objcMetaType(name string, fields []objcrt.Field) *ir.Type {
	name = "__objc_" + name
	if t := u.mod.LookupType(name); t != nil {
		return t
	}
	t := u.mod.Struct(name).Internal()
	for i, f := range fields {
		fname := f.Name
		if fname == "" {
			fname = "f" + strconv.Itoa(i)
		}
		t.Field(fname, objcFieldType(f))
	}
	return t
}

// objcFieldType is the store type one metadata field occupies.
func objcFieldType(f objcrt.Field) ir.FType {
	switch f.Kind {
	case objcrt.U32, objcrt.Pad:
		return ir.StoreI32.FType()
	case objcrt.U64:
		return ir.StoreI64.FType()
	}
	return ir.StorePtr.FType()
}

// objcListType interns an entsize-prefixed list of n entries: the count is
// part of the type, because the entries are inline.
func (u *unit) objcListType(name string, entry []objcrt.Field, n int) *ir.Type {
	full := "__objc_" + name + "_" + strconv.Itoa(n)
	if t := u.mod.LookupType(full); t != nil {
		return t
	}
	et := u.objcMetaType(name, entry)
	t := u.mod.Struct(full).Internal()
	for _, f := range objcrt.ListHeader {
		t.Field(f.Name, objcFieldType(f))
	}
	t.Field("entries", ir.Array(uint64(n), et.FType()))
	return t
}

// objcListInit pairs the header with the entries objcListType declared.
func objcListInit(entSize int64, entries []ir.Init) ir.Init {
	return ir.List(
		ir.Lit(ir.Int(entSize)),
		ir.Lit(ir.Int(int64(len(entries)))),
		ir.List(entries...))
}

// objcConstList is one read-only list in __objc_const.
func (u *unit) objcConstList(name string, t ir.FType, init ir.Init) ir.Symbol {
	return u.mod.Global(u.symbolName(name), ir.RO, t).
		Internal().
		Section(u.objc.abi.Name(objcrt.SecConst)).
		Align(8).
		Init(init)
}

// orNull is a pointer field that may have nothing in it.
func orNull(s ir.Symbol) ir.Init {
	if s == nil {
		return ir.Lit(ir.Int(0))
	}
	return ir.RelocInit(s)
}

// ---- declared before any body ----

// declareObjCClass defines the class and metaclass objects of a class
// this unit implements, before a body can reference them. The initializer
// comes later, from emitObjCClass.
func (u *unit) declareObjCClass(k *types.ObjCInterface) {
	for _, name := range []string{objcrt.ClassSymbol(k.Name), objcrt.MetaclassSymbol(k.Name)} {
		if _, done := u.objc.classSyms[name]; done {
			continue
		}
		u.objc.classSyms[name] = u.mod.Global(u.symbolName(name), ir.RW, u.objcMetaType("class", objcrt.Class).FType()).
			Export().
			Section(u.objc.abi.Name(objcrt.SecClassData)).
			Align(8)
	}
}

// objcInstanceLayout is where the compiler believes a class's instance
// variables start and end. The runtime compares the start with the
// superclass's real size and slides every offset variable by the
// difference, so these are claims rather than facts.
func (u *unit) objcInstanceLayout(k *types.ObjCInterface) (start, size int64) {
	off := int64(0)
	if k.Super != nil {
		_, off = u.objcInstanceLayout(k.Super)
	}
	start = off
	for i := range k.Ivars {
		sz, align := u.sizeAlign(k.Ivars[i].Type)
		off = roundUp(off, align) + sz
	}
	return start, off
}

func roundUp(n, to int64) int64 {
	if to <= 1 {
		return n
	}
	return (n + to - 1) / to * to
}

// objcIvarOffsets is where each of a class's own instance variables sits.
func (u *unit) objcIvarOffsets(k *types.ObjCInterface) []int64 {
	off, _ := u.objcInstanceLayout(k)
	out := make([]int64, len(k.Ivars))
	for i := range k.Ivars {
		sz, align := u.sizeAlign(k.Ivars[i].Type)
		off = roundUp(off, align)
		out[i] = off
		off += sz
	}
	return out
}

// declareObjCIvars defines the offset variables of a class this unit
// implements.
func (u *unit) declareObjCIvars(k *types.ObjCInterface) {
	offs := u.objcIvarOffsets(k)
	for i := range k.Ivars {
		name := objcrt.IvarOffsetSymbol(k.Name, k.Ivars[i].Name)
		if _, done := u.objc.ivarSyms[name]; done {
			continue
		}
		u.objc.ivarSyms[name] = u.mod.Global(u.symbolName(name), ir.RW, ir.StoreI32.FType()).
			Export().
			Section(u.objc.abi.Name(objcrt.SecIvarOffsets)).
			Align(4).
			Init(ir.Lit(ir.Int(offs[i])))
	}
}

// ---- written after every body ----

// objcMethodEntry is one entry of a method list.
type objcMethodEntry struct {
	method *types.ObjCMethod
	fn     ir.Symbol
}

// emitObjCMetadata writes everything the runtime reads.
func (u *unit) emitObjCMetadata() {
	o := u.objc
	if o == nil {
		return
	}
	for _, impl := range o.info.Impls {
		inst, cls := u.objcImplMethods(impl)
		if impl.Category == "" {
			u.emitObjCClass(impl.Class, inst, cls)
		} else {
			u.emitObjCCategory(impl, inst, cls)
		}
	}
	for _, p := range o.info.Protocols {
		u.objcProtocol(p)
	}
	if len(o.info.Impls) == 0 && len(o.selRefs) == 0 && len(o.classRefs) == 0 &&
		len(o.superRefs) == 0 && len(o.protoSyms) == 0 {
		return
	}
	u.emitObjCList(objcrt.ClassListLabel, objcrt.SecClassList, o.classList)
	u.emitObjCList(objcrt.CategoryListLabel, objcrt.SecCategoryList, o.categoryList)
	u.emitObjCList(objcrt.NonLazyClassListLabel, objcrt.SecNonLazyClassList, o.nonLazyClassList)
	u.emitObjCList(objcrt.NonLazyCategoryListLabel, objcrt.SecNonLazyCategoryList, o.nonLazyCategoryList)
	// The image info word is what tells the runtime an image is one of
	// its own. Without it the selector references are never uniqued, and
	// the first send fails.
	u.mod.Global(u.symbolName(objcrt.ImageInfoLabel), ir.RO, u.objcMetaType("image_info", objcrt.ImageInfo).FType()).
		Internal().
		Section(o.abi.Name(objcrt.SecImageInfo)).
		Align(4).
		Init(ir.List(ir.Lit(ir.Int(0)), ir.Lit(ir.Int(int64(objcrt.ImageInfoFlags())))))
}

// objcImplMethods is an @implementation's methods -- written and
// synthesized -- split into instance and class methods.
func (u *unit) objcImplMethods(impl *sema.ObjCImpl) (inst, cls []objcMethodEntry) {
	written := map[string]bool{}
	for _, m := range impl.Methods {
		f, ok := u.funcs[m.Func]
		if !ok {
			continue
		}
		e := objcMethodEntry{method: m.Method, fn: f}
		if m.Method.Class {
			cls = append(cls, e)
		} else {
			written[m.Method.Selector] = true
			inst = append(inst, e)
		}
	}
	if impl.Category == "" {
		if e, ok := u.objcCxxDestruct(impl.Class); ok {
			inst = append(inst, e)
		}
	}
	for _, p := range impl.Synthesized {
		if !written[p.Getter] {
			if e, ok := u.synthesizeObjCGetter(impl.Class, p); ok {
				inst = append(inst, e)
			}
		}
		if !p.Readonly && !written[p.Setter] {
			if e, ok := u.synthesizeObjCSetter(impl.Class, p); ok {
				inst = append(inst, e)
			}
		}
	}
	return inst, cls
}

// emitObjCClass writes a class, its metaclass, and what they point at.
func (u *unit) emitObjCClass(k *types.ObjCInterface, inst, cls []objcMethodEntry) {
	o := u.objc
	name := u.objcClassName(k.Name)
	instList := u.emitObjCMethodList(objcrt.InstanceMethodsSymbol(k.Name), inst)
	clsList := u.emitObjCMethodList(objcrt.ClassMethodsSymbol(k.Name), cls)
	ivars := u.emitObjCIvarList(k)
	props := u.emitObjCPropertyList(objcrt.PropertiesSymbol(k.Name), k.Properties, false)
	protocols := u.emitObjCProtocolList(objcrt.ClassProtocolsSymbol(k.Name), k.Protocols)

	start, size := u.objcInstanceLayout(k)
	root := k.Super == nil
	clsSize := o.abi.SizeOf(objcrt.Class)

	metaRO := u.emitObjCClassRO(objcrt.MetaclassROSymbol(k.Name), uint32(objcrt.ClassFlags(true, root, true, false, false)),
		clsSize, clsSize, name, clsList, protocols, nil, nil)
	dtor := false
	for _, m := range inst {
		if m.method.Selector == objcrt.CxxDestructSelector {
			dtor = true
		}
	}
	ro := u.emitObjCClassRO(objcrt.ClassROSymbol(k.Name), uint32(objcrt.ClassFlags(false, root, true, dtor, false)),
		start, size, name, instList, protocols, ivars, props)

	// The four links: a class's isa is its metaclass and its superclass
	// the superclass; a metaclass's isa is the *root* metaclass and its
	// superclass the superclass's metaclass -- or, for a root, its own
	// class.
	top := k
	for top.Super != nil {
		top = top.Super
	}
	metaIsa := u.objcClassSymbol(objcrt.MetaclassSymbol(top.Name))
	metaSuper := u.objcClassSymbol(objcrt.ClassSymbol(k.Name))
	var clsSuper ir.Symbol
	if k.Super != nil {
		metaSuper = u.objcClassSymbol(objcrt.MetaclassSymbol(k.Super.Name))
		clsSuper = u.objcClassSymbol(objcrt.ClassSymbol(k.Super.Name))
	}
	meta := u.emitObjCClassObject(objcrt.MetaclassSymbol(k.Name), metaIsa, metaSuper, metaRO)
	class := u.emitObjCClassObject(objcrt.ClassSymbol(k.Name), meta, clsSuper, ro)

	o.classList = append(o.classList, class)
	if objcHasLoad(cls) {
		o.nonLazyClassList = append(o.nonLazyClassList, class)
	}
}

// objcHasLoad reports whether class methods include +load, which the
// runtime calls at image load only for what is on a non-lazy list.
func objcHasLoad(cls []objcMethodEntry) bool {
	for _, m := range cls {
		if m.method.Selector == objcrt.LoadSelector {
			return true
		}
	}
	return false
}

// emitObjCClassObject fills in the five words of a class object.
func (u *unit) emitObjCClassObject(name string, isa, super, ro ir.Symbol) ir.Symbol {
	g, _ := u.objc.classSyms[name].(*ir.Global)
	if g == nil {
		u.declareObjCClass(&types.ObjCInterface{Name: name})
		g = u.objc.classSyms[name].(*ir.Global)
	}
	g.Init(ir.List(
		orNull(isa),
		orNull(super),
		ir.RelocInit(u.objcEmptyCache()),
		ir.Lit(ir.Int(0)),
		orNull(ro),
	))
	return g
}

// objcEmptyCache is the shared empty cache every class points at until
// the runtime gives it one.
func (u *unit) objcEmptyCache() ir.Symbol {
	name := u.symbolName(objcrt.EmptyCache)
	if s := u.mod.Lookup(name); s != nil {
		return s
	}
	return u.mod.ImportGlobal(name, ir.StorePtr.FType())
}

// emitObjCClassRO writes a class's read-only half.
func (u *unit) emitObjCClassRO(name string, flags uint32, start, size int64,
	className, methods, protocols, ivars, props ir.Symbol) ir.Symbol {
	return u.objcConstList(name, u.objcMetaType("class_ro", objcrt.ClassRO).FType(), ir.List(
		ir.Lit(ir.Int(int64(flags))),
		ir.Lit(ir.Int(start)),
		ir.Lit(ir.Int(size)),
		ir.Lit(ir.Int(0)), // reserved
		ir.Lit(ir.Int(0)), // ivarLayout
		ir.RelocInit(className),
		orNull(methods),
		orNull(protocols),
		orNull(ivars),
		ir.Lit(ir.Int(0)), // weakIvarLayout
		orNull(props),
	))
}

// emitObjCMethodList writes a method list, or nothing for no methods.
func (u *unit) emitObjCMethodList(name string, ms []objcMethodEntry) ir.Symbol {
	if len(ms) == 0 {
		return nil
	}
	o := u.objc
	items := make([]ir.Init, 0, len(ms))
	for _, m := range ms {
		items = append(items, ir.List(
			ir.RelocInit(u.objcMethodName(m.method.Selector)),
			ir.RelocInit(u.objcMethodType(o.abi.MethodTypes(m.method.Result, objcParams(m.method), u.model))),
			orNull(m.fn)))
	}
	return u.objcConstList(name, u.objcListType("method", objcrt.Method, len(ms)).FType(),
		objcListInit(o.abi.EntSize(objcrt.Method), items))
}

// objcParams is a method's parameters as the encoder takes them.
func objcParams(m *types.ObjCMethod) []types.Param {
	ps := make([]types.Param, len(m.Params))
	for i, t := range m.Params {
		ps[i] = types.Param{Type: t}
	}
	return ps
}

// emitObjCIvarList writes a class's own instance variables.
func (u *unit) emitObjCIvarList(k *types.ObjCInterface) ir.Symbol {
	if len(k.Ivars) == 0 {
		return nil
	}
	o := u.objc
	items := make([]ir.Init, 0, len(k.Ivars))
	for i := range k.Ivars {
		iv := &k.Ivars[i]
		size, align := u.sizeAlign(iv.Type)
		items = append(items, ir.List(
			ir.RelocInit(u.objcIvarOffset(k.Name, iv.Name)),
			ir.RelocInit(u.objcMethodName(iv.Name)),
			ir.RelocInit(u.objcMethodType(o.abi.EncodeExtended(iv.Type, u.model))),
			ir.Lit(ir.Int(log2(align))),
			ir.Lit(ir.Int(size))))
	}
	return u.objcConstList(objcrt.IvarsSymbol(k.Name), u.objcListType("ivar", objcrt.Ivar, len(k.Ivars)).FType(),
		objcListInit(o.abi.EntSize(objcrt.Ivar), items))
}

func log2(n int64) int64 {
	k := int64(0)
	for n > 1 {
		n >>= 1
		k++
	}
	return k
}

// emitObjCPropertyList writes properties as name and attribute string, in
// name order.
func (u *unit) emitObjCPropertyList(name string, props map[string]*types.ObjCProperty, class bool) ir.Symbol {
	var names []string
	for n, p := range props {
		if p.Class == class {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	o := u.objc
	items := make([]ir.Init, 0, len(names))
	for _, n := range names {
		items = append(items, ir.List(
			ir.RelocInit(u.objcCString(n, objcrt.SecMethodNames, objcrt.PropertyAttrLabel)),
			ir.RelocInit(u.objcCString(u.objcPropertyAttrs(props[n]), objcrt.SecMethodNames, objcrt.PropertyAttrLabel))))
	}
	return u.objcConstList(name, u.objcListType("property", objcrt.Property, len(items)).FType(),
		objcListInit(o.abi.EntSize(objcrt.Property), items))
}

// objcPropertyAttrs is a property's attribute string: T@"NSString",C,N,V_name.
func (u *unit) objcPropertyAttrs(p *types.ObjCProperty) string {
	d := objcrt.PropertyDesc{
		Type:      u.objc.abi.EncodeProperty(p.Type, u.model),
		Readonly:  p.Readonly,
		Copy:      p.Ownership == "copy",
		Retain:    p.Ownership == "strong" || p.Ownership == "retain",
		Weak:      p.Ownership == "weak",
		Nonatomic: !p.Atomic,
		Ivar:      p.Ivar,
	}
	if p.Getter != p.Name {
		d.Getter = p.Getter
	}
	if def := "set" + upperFirst(p.Name) + ":"; p.Setter != def && !p.Readonly {
		d.Setter = p.Setter
	}
	return objcrt.PropertyAttributes(d)
}

func upperFirst(s string) string {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return s
	}
	return string(s[0]-'a'+'A') + s[1:]
}

// emitObjCCategory writes a category's metadata.
func (u *unit) emitObjCCategory(impl *sema.ObjCImpl, inst, cls []objcMethodEntry) {
	o := u.objc
	k, name := impl.Class, impl.Category
	instList := u.emitObjCMethodList(objcrt.CategoryInstanceMethodsSymbol(k.Name, name), inst)
	clsList := u.emitObjCMethodList(objcrt.CategoryClassMethodsSymbol(k.Name, name), cls)
	g := u.objcConstList(objcrt.CategorySymbol(k.Name, name), u.objcMetaType("category", objcrt.Category).FType(), ir.List(
		ir.RelocInit(u.objcClassName(name)),
		ir.RelocInit(u.objcClassSymbol(objcrt.ClassSymbol(k.Name))),
		orNull(instList),
		orNull(clsList),
		ir.Lit(ir.Int(0)), // protocols
		ir.Lit(ir.Int(0)), // instance properties
		ir.Lit(ir.Int(0)), // class properties
		ir.Lit(ir.Int(o.abi.SizeOf(objcrt.Category))),
		ir.Lit(ir.Int(0)),
	))
	o.categoryList = append(o.categoryList, g)
	if objcHasLoad(cls) {
		o.nonLazyCategoryList = append(o.nonLazyCategoryList, g)
	}
}

// emitObjCList writes one of the pointer lists the runtime walks.
func (u *unit) emitObjCList(name string, sec objcrt.Section, syms []ir.Symbol) {
	if len(syms) == 0 {
		return
	}
	items := make([]ir.Init, 0, len(syms))
	for _, s := range syms {
		items = append(items, ir.RelocInit(s))
	}
	u.mod.Global(u.symbolName(name), ir.RW, ir.Array(uint64(len(syms)), ir.StorePtr.FType())).
		Internal().
		Section(u.objc.abi.Name(sec)).
		Align(8).
		Init(ir.List(items...))
}

// ---- protocols ----

// objcProtocolNamed finds a protocol by name.
func (u *unit) objcProtocolNamed(name string) *types.ObjCProtocol {
	if u.res.GlobalScope == nil || u.res.GlobalScope.ObjC == nil {
		return nil
	}
	return u.res.GlobalScope.ObjC.Protocols[name]
}

// objcProtocol is a protocol's object, emitted once per unit on first
// use. Every image that mentions a protocol states its own copy, weak and
// hidden, and the linker keeps one.
func (u *unit) objcProtocol(p *types.ObjCProtocol) ir.Symbol {
	o := u.objc
	if s, ok := o.protoSyms[p.Name]; ok {
		return s
	}
	g := u.mod.Global(u.symbolName(objcrt.ProtocolSymbol(p.Name)), ir.RW, u.objcMetaType("protocol", objcrt.Protocol).FType()).
		Export().Hidden().Weak().
		Section(o.abi.Name(objcrt.SecProtocolData)).
		Align(8)
	o.protoSyms[p.Name] = g

	inst, optInst := objcSplitOptional(p.Methods)
	cls, optCls := objcSplitOptional(p.ClassMethods)
	var refs ir.Symbol
	if len(p.Protocols) > 0 {
		refs = u.emitObjCProtocolList(objcrt.ProtocolRefsSymbol(p.Name), p.Protocols)
	}
	g.Init(ir.List(
		ir.Lit(ir.Int(0)), // isa
		ir.RelocInit(u.objcClassName(p.Name)),
		orNull(refs),
		orNull(u.emitObjCProtocolMethods(objcrt.ProtocolInstanceMethodsSymbol(p.Name), inst)),
		orNull(u.emitObjCProtocolMethods(objcrt.ProtocolClassMethodsSymbol(p.Name), cls)),
		orNull(u.emitObjCProtocolMethods(objcrt.ProtocolOptionalInstanceMethodsSymbol(p.Name), optInst)),
		orNull(u.emitObjCProtocolMethods(objcrt.ProtocolOptionalClassMethodsSymbol(p.Name), optCls)),
		orNull(u.emitObjCPropertyList(objcrt.ProtocolPropertiesSymbol(p.Name), p.Properties, false)),
		ir.Lit(ir.Int(o.abi.SizeOf(objcrt.Protocol))),
		ir.Lit(ir.Int(0)), // flags
		orNull(u.emitObjCMethodTypes(p.Name, inst, cls, optInst, optCls)),
		ir.Lit(ir.Int(0)), // demangledName
		ir.Lit(ir.Int(0)), // classProperties
	))
	u.mod.Global(u.symbolName(objcrt.ProtocolLabelSymbol(p.Name)), ir.RW, ir.StorePtr.FType()).
		Export().Hidden().Weak().
		Section(o.abi.Name(objcrt.SecProtocolList)).
		Align(8).
		Init(ir.RelocInit(g))
	return g
}

// objcSplitOptional splits a protocol's methods into required and
// @optional, each in selector order.
func objcSplitOptional(ms map[string]*types.ObjCMethod) (req, opt []*types.ObjCMethod) {
	sels := make([]string, 0, len(ms))
	for s := range ms {
		sels = append(sels, s)
	}
	sort.Strings(sels)
	for _, s := range sels {
		if ms[s].Optional {
			opt = append(opt, ms[s])
		} else {
			req = append(req, ms[s])
		}
	}
	return req, opt
}

// emitObjCProtocolMethods writes one of a protocol's four method lists,
// whose implementations are null.
func (u *unit) emitObjCProtocolMethods(name string, ms []*types.ObjCMethod) ir.Symbol {
	entries := make([]objcMethodEntry, len(ms))
	for i, m := range ms {
		entries[i] = objcMethodEntry{method: m}
	}
	return u.emitObjCMethodList(name, entries)
}

// emitObjCMethodTypes is a protocol's extendedMethodTypes: one encoding
// per method, in the order of the four lists.
func (u *unit) emitObjCMethodTypes(name string, groups ...[]*types.ObjCMethod) ir.Symbol {
	var items []ir.Init
	for _, g := range groups {
		for _, m := range g {
			items = append(items, ir.RelocInit(u.objcMethodType(u.objc.abi.MethodTypes(m.Result, objcParams(m), u.model))))
		}
	}
	if len(items) == 0 {
		return nil
	}
	return u.objcConstList(objcrt.ProtocolMethodTypesSymbol(name), ir.Array(uint64(len(items)), ir.StorePtr.FType()), ir.List(items...))
}

// emitObjCProtocolList writes a null-terminated protocol list: a count,
// the entries, and a null.
func (u *unit) emitObjCProtocolList(name string, ps []*types.ObjCProtocol) ir.Symbol {
	if len(ps) == 0 {
		return nil
	}
	items := []ir.Init{ir.Lit(ir.Int(int64(len(ps))))}
	for _, p := range ps {
		items = append(items, ir.RelocInit(u.objcProtocol(p)))
	}
	items = append(items, ir.Lit(ir.Int(0)))
	return u.objcConstList(name, ir.Array(uint64(len(items)), ir.StorePtr.FType()), ir.List(items...))
}

// ---- synthesized accessors ----

// objcAccessor declares a synthesized accessor as the function a written
// one would be -- -[Class sel](self, _cmd[, value]) -- and hands its body
// to build.
func (u *unit) objcAccessor(k *types.ObjCInterface, sel string, ret types.Type, value types.Type, build func(fl *fn, self ir.Ptr, cmd ir.Value, val ir.Value)) (objcMethodEntry, bool) {
	m := k.LookupMethod(sel, false)
	if m == nil || u.mod.Err() != nil {
		return objcMethodEntry{}, false
	}
	name := "-[" + k.Name + " " + sel + "]"
	params := []types.Param{{Name: "self", Type: &types.Pointer{Elem: k}}, {Name: "_cmd", Type: &types.Pointer{Elem: types.Typ(types.Void)}}}
	if value != nil {
		params = append(params, types.Param{Name: "value", Type: value})
	}
	sym := &sema.FuncSymbol{SymName: name, AsmLabel: objcrt.MethodSymbol(k.Name, "", sel, false), Internal: true,
		FuncType: &types.Func{Ret: ret, Params: params}}
	for _, p := range params {
		sym.Params = append(sym.Params, &sema.VarSymbol{SymName: p.Name, SymType: p.Type})
	}
	f := u.declareFunc(sym)
	fl := &fn{u: u, sym: sym, f: f, slots: map[sema.Symbol]ir.Ptr{}}
	fl.entry = f.Entry()
	fl.blk = f.Block("body")
	if p, has := u.srets[sym]; has {
		fl.sret, fl.hasSRet = p, true
	}
	args := u.params[sym]
	self, _ := args[0].(ir.Ptr)
	var val ir.Value
	if len(args) > 2 {
		val = args[2]
	}
	build(fl, self, args[1], val)
	fl.entry.Br(fl.f.Blocks()[1].To())
	if fl.blk != nil {
		fl.returnDefault()
	}
	return objcMethodEntry{method: m, fn: f}, true
}

// objcIvarIn is the address of an instance variable of self, in an
// accessor, and its offset.
func (fl *fn) objcIvarIn(k *types.ObjCInterface, self ir.Ptr, ivar string) (ir.Ptr, ir.I64) {
	_, owner := k.LookupIvar(ivar)
	if owner == nil {
		owner = k
	}
	off := fl.blk.I64.SLoad32(fl.blk.Ptr.GetAddr(fl.u.objcIvarOffset(owner.Name, ivar)))
	return fl.blk.Ptr.Add(self, off), off
}

func (u *unit) synthesizeObjCGetter(k *types.ObjCInterface, p *types.ObjCProperty) (objcMethodEntry, bool) {
	return u.objcAccessor(k, p.Getter, p.Type, nil, func(fl *fn, self ir.Ptr, cmd ir.Value, _ ir.Value) {
		addr, off := fl.objcIvarIn(k, self, p.Ivar)
		if rec := classOf(p.Type); rec != nil {
			if fl.hasSRet {
				fl.copyObject(fl.sret, addr, rec, nil)
			}
			fl.blk.Return()
			fl.blk = nil
			return
		}
		switch {
		case p.Ownership == "weak":
			load := fl.u.objcImport(objcrt.LoadWeakRetained, ir.NewSig().Param(ir.TypePtr).Ret(ir.TypePtr))
			ret := fl.u.objcImport(objcrt.AutoreleaseReturnValue, ir.NewSig().Param(ir.TypePtr).Ret(ir.TypePtr))
			v := fl.blk.Call(load, addr).Ptr(0)
			fl.blk.Return(fl.blk.Call(ret, v).Ptr(0))
		case types.IsObjCRetainable(p.Type) && p.Atomic:
			get := fl.u.objcImport(objcrt.GetProperty, ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypeI64).Param(ir.TypeI32).Ret(ir.TypePtr))
			fl.blk.Return(fl.blk.Call(get, self, cmd, off, fl.blk.I32.Const(1)).Ptr(0))
		default:
			fl.blk.Return(fl.load(addr, p.Type))
		}
		fl.blk = nil
	})
}

func (u *unit) synthesizeObjCSetter(k *types.ObjCInterface, p *types.ObjCProperty) (objcMethodEntry, bool) {
	return u.objcAccessor(k, p.Setter, types.Typ(types.Void), p.Type, func(fl *fn, self ir.Ptr, cmd ir.Value, val ir.Value) {
		addr, off := fl.objcIvarIn(k, self, p.Ivar)
		if rec := classOf(p.Type); rec != nil {
			if src, ok := val.(ir.Ptr); ok {
				fl.copyObject(addr, src, rec, nil)
			}
			return
		}
		switch {
		case p.Ownership == "weak":
			store := fl.u.objcImport(objcrt.StoreWeak, ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr).Ret(ir.TypePtr))
			fl.blk.Call(store, addr, val)
		case types.IsObjCRetainable(p.Type) && (p.Ownership == "copy" || p.Atomic && p.Ownership != "assign" && p.Ownership != "unsafe_unretained"):
			set := fl.u.objcImport(objcrt.SetPropertySymbol(p.Atomic, p.Ownership == "copy"),
				ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypeI64))
			fl.blk.Call(set, self, cmd, val, off)
		case types.IsObjCRetainable(p.Type) && p.Ownership != "assign" && p.Ownership != "unsafe_unretained":
			store := fl.u.objcImport(objcrt.StoreStrong, ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr))
			fl.blk.Call(store, addr, val)
		default:
			fl.store(addr, val, p.Type)
		}
	})
}

// objcCxxDestruct is the .cxx_destruct method of a class whose instance
// variables ARC manages or C++ destroys: what the runtime calls, after the
// last dealloc, to end them. It is what releases a strong ivar.
func (u *unit) objcCxxDestruct(k *types.ObjCInterface) (objcMethodEntry, bool) {
	needed := false
	for i := range k.Ivars {
		t := k.Ivars[i].Type
		if q := ownership(t); q == types.QObjCStrong || q == types.QObjCWeak {
			needed = true
		}
		if rec := classOf(t); rec != nil && u.destructor(rec) != nil {
			needed = true
		}
	}
	if !needed {
		return objcMethodEntry{}, false
	}
	m := &types.ObjCMethod{Selector: objcrt.CxxDestructSelector, Result: types.Typ(types.Void), Owner: k}
	name := "-[" + k.Name + " .cxx_destruct]"
	params := []types.Param{{Name: "self", Type: &types.Pointer{Elem: k}}, {Name: "_cmd", Type: &types.Pointer{Elem: types.Typ(types.Void)}}}
	sym := &sema.FuncSymbol{SymName: name, AsmLabel: objcrt.MethodSymbol(k.Name, "", "_cxx_destruct", false), Internal: true,
		FuncType: &types.Func{Ret: types.Typ(types.Void), Params: params}}
	for _, p := range params {
		sym.Params = append(sym.Params, &sema.VarSymbol{SymName: p.Name, SymType: p.Type})
	}
	f := u.declareFunc(sym)
	fl := &fn{u: u, sym: sym, f: f, slots: map[sema.Symbol]ir.Ptr{}}
	fl.entry = f.Entry()
	fl.blk = f.Block("body")
	self, _ := u.params[sym][0].(ir.Ptr)
	// Last declared, first destroyed.
	for i := len(k.Ivars) - 1; i >= 0; i-- {
		iv := k.Ivars[i]
		addr, _ := fl.objcIvarIn(k, self, iv.Name)
		switch ownership(iv.Type) {
		case types.QObjCStrong:
			fl.blk.Call(u.objcImport(objcrt.StoreStrong, ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr)), addr, fl.blk.Ptr.Const())
		case types.QObjCWeak:
			fl.blk.Call(u.objcImport(objcrt.DestroyWeak, ir.NewSig().Param(ir.TypePtr)), addr)
		default:
			if rec := classOf(iv.Type); rec != nil {
				fl.destroy(addr, rec)
			}
		}
	}
	fl.entry.Br(fl.f.Blocks()[1].To())
	fl.blk.Return()
	return objcMethodEntry{method: m, fn: f}, true
}
