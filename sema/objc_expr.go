package sema

import (
	"fmt"
	"strings"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/literal"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// objcConversion is an implicit conversion between object pointers and
// blocks: to and from id, from a class to its superclass, a block to id,
// and nullptr to either.
func objcConversion(f, t types.Type) (ConvRank, bool) {
	fc, tc := types.ObjCClassOf(f), types.ObjCClassOf(t)
	_, fb := f.(*types.BlockPointer)
	_, tb := t.(*types.BlockPointer)
	if b, ok := f.(*types.Basic); ok && b.Kind() == types.NullptrKind && (tc != nil || tb) {
		return RankConversion, true
	}
	switch {
	case fc != nil && tc != nil:
		if fc == tc {
			return RankExactMatch, true
		}
		if idLike(fc) || idLike(tc) || fc.IsSubclassOf(tc) {
			return RankConversion, true
		}
		return 0, false
	case fb && tc != nil:
		return RankConversion, idLike(tc)
	case tb && fc != nil:
		return RankConversion, idLike(fc)
	case fb && tb:
		return RankConversion, true
	}
	return 0, false
}

// idLike reports whether an object type converts to and from any other:
// id, instancetype, and Class, which is a class object's type.
func idLike(c *types.ObjCInterface) bool {
	c = types.ObjCBase(c)
	return c == types.ObjCIdObject || c == types.ObjCInstancetypeObject || c == types.ObjCClassObject
}

// checkObjCExpr checks an Objective-C expression, and reports false for
// an expression that is not one.
func (a *Analyzer) checkObjCExpr(expr ast.Expr) (ExprInfo, bool) {
	switch e := expr.(type) {
	case *ast.ObjCMessageExpr:
		return a.checkObjCMessage(e), true
	case *ast.ObjCStringLit:
		if _, err := literal.Decode(a.unit, e.Str); err != nil {
			a.errorAt(e.Pos(), err.Error())
		}
		a.objcInfo().Literals[e] = &ObjCLiteral{Class: "NSString"}
		return ExprInfo{Type: a.objcPtr("NSString"), ValCat: PrValue}, true
	case *ast.ObjCSelectorExpr:
		return ExprInfo{Type: a.objcSelType(), ValCat: PrValue}, true
	case *ast.ObjCProtocolExpr:
		if e.Name != nil {
			p := a.objcProtocol(e.Name.Text(a.unit))
			a.objcInfo().Protocols = append(a.objcInfo().Protocols, p)
		}
		return ExprInfo{Type: a.objcPtr("Protocol"), ValCat: PrValue}, true
	case *ast.ObjCEncodeExpr:
		a.noteTypeId(e.Type)
		return ExprInfo{Type: &types.Pointer{Elem: types.Qualify(types.Typ(types.Char), types.QConst)}, ValCat: PrValue}, true
	case *ast.ObjCBoolLit:
		v := int64(0)
		if e.Value {
			v = 1
		}
		return ExprInfo{Type: types.Typ(types.Bool), ValCat: PrValue, IsConst: true, ConstVal: v}, true
	case *ast.ObjCAvailableExpr:
		return ExprInfo{Type: types.Typ(types.Bool), ValCat: PrValue}, true
	case *ast.ObjCBoxedExpr:
		return a.checkObjCBoxed(e), true
	case *ast.ObjCArrayLit:
		for _, x := range e.Elems {
			a.objcObjectArg(x)
		}
		a.objcInfo().Literals[e] = &ObjCLiteral{Class: "NSArray", Selector: "arrayWithObjects:count:"}
		return ExprInfo{Type: a.objcPtr("NSArray"), ValCat: PrValue}, true
	case *ast.ObjCDictLit:
		for i := range e.Keys {
			a.objcObjectArg(e.Keys[i])
			a.objcObjectArg(e.Values[i])
		}
		a.objcInfo().Literals[e] = &ObjCLiteral{Class: "NSDictionary", Selector: "dictionaryWithObjects:forKeys:count:"}
		return ExprInfo{Type: a.objcPtr("NSDictionary"), ValCat: PrValue}, true
	case *ast.ObjCBridgeCast:
		t := a.noteTypeId(e.Type)
		a.CheckExpr(e.X)
		return ExprInfo{Type: t, ValCat: PrValue}, true
	case *ast.BlockExpr:
		return a.checkBlockExpr(e), true
	case *ast.ObjCSuperExpr, *ast.ObjCClassRecv:
		a.errorAt(e.Pos(), "only a message's receiver may be super or a class name")
		return ExprInfo{Type: types.ObjCId, ValCat: PrValue}, true
	}
	return ExprInfo{}, false
}

// objcPtr is a pointer to the named class.
func (a *Analyzer) objcPtr(name string) types.Type {
	return &types.Pointer{Elem: a.objcClass(name)}
}

// objcObjectArg checks an element of a collection literal, which is an
// object.
func (a *Analyzer) objcObjectArg(x ast.Expr) {
	info := a.CheckExpr(x)
	if !types.IsObjCRetainable(types.Decay(types.RemoveReference(info.Type))) && !isNullConstant(x, info) {
		a.errorAt(x.Pos(), fmt.Sprintf("a collection literal holds objects, not %q", info.Type))
	}
}

// checkObjCBoxed types @(e): an NSNumber made by the method for the
// value's type, or an NSString from a C string.
func (a *Analyzer) checkObjCBoxed(e *ast.ObjCBoxedExpr) ExprInfo {
	info := a.CheckExpr(e.X)
	t := types.Unqualify(types.Decay(types.RemoveReference(info.Type)))
	if en, isEnum := t.(*types.Enum); isEnum && en.Underlying != nil {
		t = types.Unqualify(en.Underlying)
	}
	lit := &ObjCLiteral{Class: "NSNumber", Arg: t}
	if p, isPtr := t.(*types.Pointer); isPtr {
		if b, isBasic := types.Unqualify(p.Elem).(*types.Basic); isBasic && b.Kind() == types.Char {
			lit.Class, lit.Selector = "NSString", "stringWithUTF8String:"
			a.objcInfo().Literals[e] = lit
			return ExprInfo{Type: a.objcPtr("NSString"), ValCat: PrValue}
		}
	}
	if rec, isRec := t.(*types.Record); isRec {
		// A struct is boxed in an NSValue of its bytes and encoding.
		lit.Class, lit.Selector = "NSValue", "valueWithBytes:objCType:"
		lit.Arg = rec
		a.objcInfo().Literals[e] = lit
		return ExprInfo{Type: a.objcPtr("NSValue"), ValCat: PrValue}
	}
	b, isBasic := t.(*types.Basic)
	if !isBasic {
		a.errorAt(e.Pos(), fmt.Sprintf("cannot box a value of type %q", info.Type))
		return ExprInfo{Type: a.objcPtr("NSNumber"), ValCat: PrValue}
	}
	switch b.Kind() {
	case types.Bool:
		lit.Selector = "numberWithBool:"
	case types.Char, types.SChar:
		lit.Selector = "numberWithChar:"
	case types.UChar:
		lit.Selector = "numberWithUnsignedChar:"
	case types.Short:
		lit.Selector = "numberWithShort:"
	case types.UShort:
		lit.Selector = "numberWithUnsignedShort:"
	case types.Int:
		lit.Selector = "numberWithInt:"
	case types.UInt:
		lit.Selector = "numberWithUnsignedInt:"
	case types.Long:
		lit.Selector = "numberWithLong:"
	case types.ULong:
		lit.Selector = "numberWithUnsignedLong:"
	case types.LongLong:
		lit.Selector = "numberWithLongLong:"
	case types.ULongLong:
		lit.Selector = "numberWithUnsignedLongLong:"
	case types.Float:
		lit.Selector = "numberWithFloat:"
	case types.Double:
		lit.Selector = "numberWithDouble:"
	default:
		a.errorAt(e.Pos(), fmt.Sprintf("cannot box a value of type %q", info.Type))
	}
	a.objcInfo().Literals[e] = lit
	return ExprInfo{Type: a.objcPtr("NSNumber"), ValCat: PrValue}
}

// objcMessageSelector is a message's selector.
func (a *Analyzer) objcMessageSelector(e *ast.ObjCMessageExpr) string {
	var b strings.Builder
	for _, p := range e.Parts {
		if p.Name != nil {
			b.WriteString(p.Name.Text(a.unit))
		}
		if p.Colon.IsValid() {
			b.WriteByte(':')
		}
	}
	return b.String()
}

// checkObjCMessage resolves a message: its receiver, the method it
// reaches, and what its arguments convert to.
func (a *Analyzer) checkObjCMessage(e *ast.ObjCMessageExpr) ExprInfo {
	sel := a.objcMessageSelector(e)
	send := &ObjCSend{Selector: sel}
	var recvClass *types.ObjCInterface
	classMsg := false
	var recvType types.Type

	switch r := e.Recv.(type) {
	case *ast.ObjCSuperExpr:
		if a.objcMethod == nil || a.objcMethod.class.Super == nil {
			a.errorAt(r.Pos(), "super is only a receiver in a method of a class that has a superclass")
			return ExprInfo{Type: types.ObjCId, ValCat: PrValue}
		}
		send.Super = true
		send.Class = a.objcMethod.class
		a.objcCaptureSelf()
		classMsg = a.objcMethod.isCls
		recvClass = a.objcMethod.class.Super
		recvType = &types.Pointer{Elem: a.objcMethod.class}
	case *ast.ObjCClassRecv:
		name := r.Name.Text(a.unit)
		c := a.objcTables().Classes[name]
		if c == nil {
			// A generic parameter, or a typedef of id: a message to an
			// unknown class.
			c = types.ObjCIdObject
		}
		send.ClassRecv = true
		send.Class = c
		classMsg = true
		recvClass = c
		recvType = &types.Pointer{Elem: c}
		for _, t := range r.TypeArgs {
			a.noteTypeId(t)
		}
	default:
		info := a.CheckExpr(e.Recv)
		recvType = types.Decay(types.RemoveReference(info.Type))
		c := types.ObjCClassOf(recvType)
		_, isBlock := types.Unqualify(recvType).(*types.BlockPointer)
		switch {
		case types.ObjCBase(c) == types.ObjCClassObject:
			classMsg = true
		case c != nil:
			recvClass = c
		case isBlock:
		case isDependentExpr(info):
			return dependentExpr()
		default:
			a.errorAt(e.Recv.Pos(), fmt.Sprintf("a message's receiver is an object, not %q", info.Type))
			return ExprInfo{Type: types.ObjCId, ValCat: PrValue}
		}
	}

	var m *types.ObjCMethod
	if recvClass != nil && !recvClass.Builtin {
		m = recvClass.LookupMethod(sel, classMsg)
	}
	if m == nil {
		t := a.objcTables()
		table := t.Instance
		if classMsg {
			table = t.Class
		}
		if ms := table[sel]; len(ms) > 0 {
			m = ms[0]
		} else if classMsg {
			// A class object answers its root class's instance methods.
			if ms := t.Instance[sel]; len(ms) > 0 {
				m = ms[0]
			}
		}
	}
	if m == nil {
		a.errorAt(e.Pos(), fmt.Sprintf("no known method for selector %q", sel))
	}
	send.Method = m

	var args []ast.Expr
	for _, p := range e.Parts {
		args = append(args, p.Args...)
	}
	if m != nil {
		if len(args) < len(m.Params) || len(args) > len(m.Params) && !m.Variadic {
			a.errorAt(e.Pos(), fmt.Sprintf("%q takes %d arguments, not %d", sel, len(m.Params), len(args)))
		}
		send.ParamTypes = m.Params
	}
	for i, arg := range args {
		info := a.CheckExpr(arg)
		if m == nil || i >= len(m.Params) {
			continue
		}
		pt := m.Params[i]
		cs := ClassifyConversion(info.Type, pt, info.ValCat == LValue)
		if !cs.Valid && !(isNullConstant(arg, info) && isPointerLike(pt)) && !isDependentType(pt) && !isDependentExpr(info) {
			a.errorAt(arg.Pos(), fmt.Sprintf("cannot pass %q as %q to %q", info.Type, pt, sel))
		}
	}

	result := types.Type(types.ObjCId)
	if m != nil {
		result = m.Result
		if m.Instancetype {
			switch {
			case classMsg && recvClass != nil && !recvClass.Builtin && !send.Super:
				result = &types.Pointer{Elem: recvClass}
			case !classMsg && recvType != nil && types.IsObjCObjectPointer(recvType):
				result = types.Unqualify(recvType)
			default:
				result = types.ObjCId
			}
		}
		if types.ObjCClassOf(result) == types.ObjCInstancetypeObject {
			result = types.ObjCId
		}
	}
	send.Result = result
	a.objcInfo().Sends[e] = send
	return ExprInfo{Type: result, ValCat: PrValue}
}

// objcClassNamed is the class an expression names when it is a class name
// and nothing in scope shadows it: the receiver of a class property,
// `NSColor.redColor`.
func (a *Analyzer) objcClassNamed(x ast.Expr) *types.ObjCInterface {
	id, ok := x.(*ast.Ident)
	if !ok || a.globalScope.ObjC == nil {
		return nil
	}
	name := id.Text(a.unit)
	if len(LookupUnqualified(a.curScope, name)) > 0 {
		return nil
	}
	return a.objcTables().Classes[name]
}

// objcMemberOf is `obj.prop` -- a property, through its accessors -- or
// `obj->ivar`, on an object; false for a member access that is neither.
func (a *Analyzer) objcMemberOf(m *ast.MemberExpr, info ExprInfo) (ExprInfo, bool) {
	c := types.ObjCClassOf(types.Decay(types.RemoveReference(info.Type)))
	if c == nil {
		return info, false
	}
	name := NameString(m.Sel, a.unit)
	if m.Op == token.ARROW {
		iv, owner := c.LookupIvar(name)
		if iv == nil {
			a.errorAt(m.Pos(), fmt.Sprintf("%s has no instance variable %q", c.Name, name))
			return ExprInfo{Type: types.Typ(types.Int), ValCat: LValue}, true
		}
		a.objcInfo().Ivars[m] = &ObjCIvarRef{Class: owner, Ivar: iv}
		return ExprInfo{Type: iv.Type, ValCat: LValue}, true
	}
	return a.objcPropRef(m, c, name, types.ObjCBase(c) == types.ObjCClassObject), true
}

// objcPropRef resolves a dot-syntax reference to its accessors.
func (a *Analyzer) objcPropRef(m *ast.MemberExpr, c *types.ObjCInterface, name string, class bool) ExprInfo {
	ref := &ObjCPropRef{Class: c, ClassProp: class, Getter: name}
	t := types.Type(types.ObjCId)
	if !c.Builtin {
		if p := c.LookupProperty(name); p != nil && p.Class == class {
			ref.Prop, ref.Getter, t = p, p.Getter, p.Type
			if !p.Readonly {
				ref.Setter = p.Setter
			}
		}
	}
	if ref.Prop == nil {
		// Dot syntax on a method that is no property: `s.length`,
		// `NSColor.redColor`, and its set...: for an assignment.
		var getter *types.ObjCMethod
		if !c.Builtin {
			getter = c.LookupMethod(name, class)
		}
		if getter == nil {
			if ms := a.objcMethodsNamed(name, class); len(ms) > 0 {
				getter = ms[0]
			}
		}
		if getter == nil || len(getter.Params) > 0 {
			a.errorAt(m.Pos(), fmt.Sprintf("%s has no property %q", c.Name, name))
			return ExprInfo{Type: types.Typ(types.Int), ValCat: LValue}
		}
		t = getter.Result
		setter := "set" + strings.ToUpper(name[:1]) + name[1:] + ":"
		if c.Builtin || c.LookupMethod(setter, class) != nil || len(a.objcMethodsNamed(setter, class)) > 0 {
			ref.Setter = setter
		}
	}
	a.objcInfo().Props[m] = ref
	return ExprInfo{Type: t, ValCat: LValue}
}

func (a *Analyzer) objcMethodsNamed(sel string, class bool) []*types.ObjCMethod {
	t := a.objcTables()
	if class {
		return t.Class[sel]
	}
	return t.Instance[sel]
}

// checkObjCSubscript is `obj[i]` or `obj[key]` on an object: the indexed
// or keyed subscripting methods.
func (a *Analyzer) checkObjCSubscript(idx *ast.IndexExpr, base, arg ExprInfo) (ExprInfo, bool) {
	c := types.ObjCClassOf(types.Decay(types.RemoveReference(base.Type)))
	if c == nil || len(idx.Args) != 1 {
		return ExprInfo{}, false
	}
	s := &ObjCSubscript{Getter: "objectAtIndexedSubscript:", Setter: "setObject:atIndexedSubscript:"}
	at := types.Unqualify(types.Decay(types.RemoveReference(arg.Type)))
	if types.IsObjCRetainable(at) {
		s.Keyed = true
		s.Getter, s.Setter = "objectForKeyedSubscript:", "setObject:forKeyedSubscript:"
	}
	result := types.Type(types.ObjCId)
	if !c.Builtin {
		if m := c.LookupMethod(s.Getter, false); m != nil {
			result = m.Result
		}
	}
	a.objcInfo().Subs[idx] = s
	return ExprInfo{Type: result, ValCat: LValue}, true
}

// ---- blocks ----

// BlockInfo is what lowering needs of a block literal: the function its
// body is compiled to, and what it captures.
type BlockInfo struct {
	Expr   *ast.BlockExpr
	Invoke *FuncSymbol
	Type   *types.BlockPointer
	// Self is the self of the method the block is written in, nil
	// outside one: what an instance variable or super in the body means.
	Self *VarSymbol
	// Captures are the enclosing function's variables the body names, in
	// the order first named; ByRef marks the __block ones, which the
	// block shares rather than copies.
	Captures []*VarSymbol
	ByRef    map[*VarSymbol]bool
	scope    *Scope
}

// checkBlockExpr checks a block literal as the function its body is: the
// block itself, then its parameters.
func (a *Analyzer) checkBlockExpr(e *ast.BlockExpr) ExprInfo {
	ret := types.Type(types.Typ(types.AutoKind))
	if e.Result != nil {
		ret = a.noteTypeId(e.Result)
	}
	params := []types.Param{{Name: "", Type: &types.Pointer{Elem: types.Typ(types.Void)}}}
	for _, p := range e.Params {
		info := BuildDeclSpecs(p.Specs, a.curScope, a.unit)
		t := a.objcParamType(BuildDeclarator(p.Decl, info.Type, a.curScope, a.unit))
		name := ""
		if p.Decl != nil && p.Decl.DeclName() != nil {
			name = NameString(p.Decl.DeclName(), a.unit)
		}
		params = append(params, types.Param{Name: name, Type: t})
	}
	a.objcBlockCount++
	host := "block"
	if a.curFunc != nil {
		host = strings.NewReplacer("[", "", "]", "", " ", "_", ":", "_", "-", "", "+", "").Replace(a.curFunc.SymName)
	}
	fn := &FuncSymbol{
		SymName:  fmt.Sprintf("__%s_block_invoke_%d", host, a.objcBlockCount),
		FuncType: &types.Func{Ret: ret, Params: params, Variadic: e.Vararg.IsValid()},
		SymPos:   e.Pos(),
		SymScope: a.globalScope,
		Body:     e.Body,
		Internal: true,
	}
	fn.AsmLabel = fn.SymName
	a.functions = append(a.functions, fn)

	fnScope := NewScope(a.curScope, FunctionScope, fn)
	info := &BlockInfo{Expr: e, Invoke: fn, ByRef: map[*VarSymbol]bool{}, scope: fnScope}
	if a.objcMethod != nil {
		info.Self = a.objcMethod.self
	}
	oldScope, oldFunc := a.curScope, a.curFunc
	a.curScope, a.curFunc = fnScope, fn
	a.objcBlocks = append(a.objcBlocks, info)
	fn.Params = a.declareParams(fn.FuncType, fnScope)
	a.CheckStmt(e.Body)
	a.objcBlocks = a.objcBlocks[:len(a.objcBlocks)-1]
	a.curScope, a.curFunc = oldScope, oldFunc
	if fn.FuncType.Ret.Kind() == types.AutoKind {
		fn.FuncType.Ret = types.Typ(types.Void)
	}

	// The block's type is the function's, less the block itself.
	sig := &types.Func{Ret: fn.FuncType.Ret, Variadic: fn.FuncType.Variadic}
	for _, p := range fn.FuncType.Params[1:] {
		sig.Params = append(sig.Params, types.Param{Type: p.Type})
	}
	info.Type = &types.BlockPointer{Func: sig}
	if a.info.Blocks == nil {
		a.info.Blocks = map[*ast.BlockExpr]*BlockInfo{}
	}
	a.info.Blocks[e] = info
	return ExprInfo{Type: info.Type, ValCat: PrValue}
}

// objcBlockCapture records a variable of an enclosing function that a
// block's body names: each block between the use and the variable's
// declaration captures it.
func (a *Analyzer) objcBlockCapture(v *VarSymbol) {
	if len(a.objcBlocks) == 0 || v == nil || v.SymScope == nil {
		return
	}
	if v.Storage == StorageStatic || v.Storage == StorageExtern || v.InClass != nil {
		return
	}
	switch v.SymScope.Kind {
	case BlockScope, FunctionScope:
	default:
		return
	}
	for i := len(a.objcBlocks) - 1; i >= 0; i-- {
		b := a.objcBlocks[i]
		if scopeEncloses(b.scope, v.SymScope) {
			return
		}
		seen := false
		for _, c := range b.Captures {
			seen = seen || c == v
		}
		if !seen {
			b.Captures = append(b.Captures, v)
			if v.ByRefBlock {
				b.ByRef[v] = true
			}
		}
	}
}

// checkBlockCall types a call through a block pointer, as a call through
// a function pointer is.
func (a *Analyzer) checkBlockCall(c *ast.CallExpr, bp *types.BlockPointer) ExprInfo {
	ft := bp.Func
	if len(c.Args) < len(ft.Params) || len(c.Args) > len(ft.Params) && !ft.Variadic {
		a.errorAt(c.Pos(), fmt.Sprintf("the block takes %d arguments, not %d", len(ft.Params), len(c.Args)))
	}
	for i, arg := range c.Args {
		info := a.CheckExpr(arg)
		if i >= len(ft.Params) {
			continue
		}
		cs := ClassifyConversion(info.Type, ft.Params[i].Type, info.ValCat == LValue)
		if !cs.Valid && !(isNullConstant(arg, info) && isPointerLike(ft.Params[i].Type)) {
			a.errorAt(arg.Pos(), fmt.Sprintf("cannot pass %q as %q to the block", info.Type, ft.Params[i].Type))
		}
	}
	return ExprInfo{Type: ft.Ret, ValCat: PrValue}
}

// objcCaptureSelf captures self into the blocks between here and the
// method: an instance variable or a super message in a block reaches the
// object through it.
func (a *Analyzer) objcCaptureSelf() {
	if len(a.objcBlocks) > 0 && a.objcMethod != nil && a.objcMethod.self != nil {
		a.objcBlockCapture(a.objcMethod.self)
	}
}
