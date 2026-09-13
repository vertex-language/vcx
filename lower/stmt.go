package lower

import (
	"fmt"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// stmt lowers one statement. Unreachable statements after terminators emit nothing.
func (fl *fn) stmt(s ast.Stmt) {
	if s == nil || fl.blk == nil {
		return
	}

	switch s := s.(type) {
	case *ast.AttrStmt:
		fl.stmt(s.Stmt)

	case *ast.CompoundStmt:
		fl.pushScope()
		for _, child := range s.Stmts {
			fl.stmt(child)
		}
		fl.popScope()

	case *ast.EmptyStmt:

	case *ast.DeclStmt:
		fl.declStmt(s)

	case *ast.ExprStmt:
		// Value is computed and dropped.
		fl.expr(s.X)
		fl.endFullExpr()

	case *ast.ReturnStmt:
		fl.returnStmt(s)

	case *ast.IfStmt:
		fl.ifStmt(s)

	case *ast.WhileStmt:
		fl.whileStmt(s)

	case *ast.DoStmt:
		fl.doStmt(s)

	case *ast.ForStmt:
		fl.forStmt(s)

	case *ast.RangeForStmt:
		fl.rangeForStmt(s)

	case *ast.SwitchStmt:
		fl.switchStmt(s)

	case *ast.BreakStmt:
		fl.destroyFrom(fl.loopDepth)
		fl.jump(fl.breakTo, "break outside any loop")

	case *ast.ContinueStmt:
		fl.destroyFrom(fl.loopDepth)
		fl.jump(fl.continueTo, "continue outside any loop")

	default:
		fl.u.errorf(s.Pos(), "lowering does not handle %T yet", s)
	}
}

// jump ends the current block with a branch to a loop's exit or header.
func (fl *fn) jump(target *ir.Block, what string) {
	if target == nil {
		// The analysis refuses this, so reaching here is an ordering bug.
		fl.u.errorf(ast.NoTok, "internal: %s", what)
		return
	}
	fl.blk.Br(target.To())
	fl.blk = nil
}

func (fl *fn) declStmt(s *ast.DeclStmt) {
	switch d := s.Decl.(type) {
	case *ast.StructuredBinding:
		fl.structuredBinding(d)
	case *ast.SimpleDecl:
		for _, init := range d.Inits {
			fl.declareLocal(init)
		}
	case *ast.UsingDirectiveDecl, *ast.UsingDecl, *ast.NamespaceAliasDecl, *ast.AliasDecl, *ast.EmptyDecl, *ast.StaticAssertDecl:
		// Declarations that do not generate runtime code in a block.
	default:
		fl.u.errorf(s.Pos(), "lowering does not handle %T in a block yet", s.Decl)
	}
}

// structuredBinding lowers a structured binding declaration.
func (fl *fn) structuredBinding(sb *ast.StructuredBinding) {
	hidden, _ := fl.u.res.Info.Defs[sb].(*sema.VarSymbol)
	if hidden == nil {
		fl.u.errorf(sb.Pos(), "lowering has no variable for this structured binding")
		return
	}
	init := fl.u.res.Info.BindingInits[sb]
	if init == nil {
		fl.u.errorf(sb.Pos(), "lowering has no initializer for this structured binding")
		return
	}
	fl.declareLocal(init)

	for _, name := range sb.Names {
		sym, _ := fl.u.res.Info.Defs[name].(*sema.VarSymbol)
		if sym == nil || sym.Binding == nil || sym.Binding.Get == nil {
			continue
		}
		// Tuple-like element: bind get<i>(hidden) result as a reference.
		slot := fl.alloc(sym.SymType, sym.SymName)
		fl.slots[sym] = slot
		get := sym.Binding.Get
		target := fl.u.callee(get)
		if target == nil {
			return
		}
		obj, ok := fl.slotOf(hidden)
		if !ok {
			return
		}
		want := get.FuncType.Params[0].Type
		var arg ir.Value = obj
		if isReference(hidden.SymType) {
			arg = fl.blk.Ptr.Load(obj)
		}
		if !isReference(want) {
			fl.u.errorf(sb.Pos(), "lowering: get by value is not handled")
			return
		}
		res := fl.blk.Call(target, arg)
		if res.Len() == 0 {
			return
		}
		fl.blk.Ptr.Store(res.Value(0).(ir.Ptr), slot)
	}
}

// slotOf is the frame slot of a local the analysis defined.
func (fl *fn) slotOf(v *sema.VarSymbol) (ir.Ptr, bool) {
	slot, ok := fl.slots[v]
	return slot, ok
}

// declareLocal gives one declarator a frame slot and runs its initializer.
func (fl *fn) declareLocal(init *ast.InitDeclarator) {
	sym := fl.localSymbol(init)
	if sym == nil {
		return
	}
	if sym.Storage == sema.StorageStatic {
		fl.declareStaticLocal(init, sym)
		return
	}

	slot := fl.alloc(sym.SymType, sym.SymName)
	fl.slots[sym] = slot
	defer fl.endFullExpr()

	// Bind reference to the initializer address.
	if isReference(sym.SymType) {
		if init.Value == nil {
			return
		}
		if addr, ok := fl.bind(init.Value, sym.SymType); ok {
			fl.blk.Ptr.Store(addr, slot)
			fl.extendTemporary(addr)
		}
		return
	}

	// Class object constructed via selected constructor.
	if ctor := fl.u.res.Info.Ctors[init]; ctor != nil {
		fl.construct(slot, classOf(sym.SymType), ctor, init)
		fl.track(slot, sym.SymType)
		return
	}
	fl.track(slot, sym.SymType)

	if init.Braced != nil {
		fl.initList(slot, sym.SymType, init.Braced)
		return
	}
	if init.Value == nil {
		// Default-initialize class objects or class element arrays.
		if rec := classOf(sym.SymType); rec != nil {
			fl.defaultConstruct(slot, rec, init.Pos())
		} else if arr, isArr := types.Unqualify(sym.SymType).(*types.Array); isArr && classOf(elementOf(sym.SymType)) != nil {
			fl.defaultElements(slot, arr, init.Pos())
		}
		return
	}
	if rec := classOf(sym.SymType); rec != nil {
		fl.installVPtr(slot, rec)
	}
	if rec := classOf(sym.SymType); rec != nil {
		// Direct prvalue initialization or plain copy.
		fl.exprInto(slot, init.Value, rec)
		return
	}
	if arr, isArr := types.Unqualify(sym.SymType).(*types.Array); isArr {
		if lit, isLit := unparen(init.Value).(*ast.StringLit); isLit {
			fl.initArrayFromString(slot, arr, lit)
			return
		}
		if src, _, ok := fl.lvalue(init.Value); ok {
			size, _ := fl.u.sizeAlign(arr)
			fl.blk.MemCpy(slot, src, fl.blk.I64.Const(size))
			return
		}
	}
	v, converted := fl.convertedScalar(init.Value)
	if !converted {
		v = fl.expr(init.Value)
	}
	if v == nil {
		return
	}
	fl.store(slot, fl.convert(v, fl.typeOf(init.Value), sym.SymType), sym.SymType)
}

// declareStaticLocal lowers a block-scope static object. Constant initializers
// become the global's initial image; non-constant initializers run once guarded
// by a flag.
func (fl *fn) declareStaticLocal(init *ast.InitDeclarator, sym *sema.VarSymbol) {
	u := fl.u
	_, align := u.sizeAlign(sym.SymType)
	fl.nstatics++
	base := fmt.Sprintf("?%s@?%d?%s@4", sym.SymName, fl.nstatics, u.funcSymbol(fl.sym))
	g := u.mod.Global(u.symbolName(base+"A"), ir.RW, u.storageType(sym.SymType))
	g.Internal()
	g.Align(uint64(align))
	u.globals[sym] = g
	obj := fl.blk.Ptr.GetAddr(g)

	// Constant initialization, or none at all: the image is the value.
	if init.Value == nil && init.Braced == nil && u.res.Info.Ctors[init] == nil && classOf(sym.SymType) == nil {
		return
	}
	if init.Value != nil && classOf(sym.SymType) == nil && !isReference(sym.SymType) && !types.IsArray(types.Unqualify(sym.SymType)) {
		if n, err := u.evalInt(init.Value); err == nil {
			g.Init(ir.Lit(ir.Int(n)))
			return
		}
	}

	guard := u.mod.Global(u.symbolName(base+"guard"), ir.RW, ir.StoreI8.FType())
	guard.Internal()
	guard.Align(1)
	gp := fl.blk.Ptr.GetAddr(guard)
	done := fl.blk.I32.Ne(fl.blk.I32.ULoad8(gp), fl.blk.I32.Const(0))
	initBlk := fl.block("static_init")
	join := fl.block("static_join")
	fl.blk.BrIf(done, join.To(), initBlk.To())

	fl.blk = initBlk
	fl.blk.I32.Store8(fl.blk.I32.Const(1), gp)
	fl.initializeObject(obj, sym, init)
	if fl.blk != nil {
		fl.blk.Br(join.To())
	}
	fl.blk = join
}

// initializeObject runs a declarator's initializer on storage that is
// already reserved: the frame slot of a local, or the object of a static.
func (fl *fn) initializeObject(slot ir.Ptr, sym *sema.VarSymbol, init *ast.InitDeclarator) {
	initVal := init.Value
	if initVal == nil && len(init.Args) == 1 && classOf(sym.SymType) == nil {
		initVal = init.Args[0]
	}
	if isReference(sym.SymType) {
		if initVal == nil {
			return
		}
		if addr, ok := fl.bind(initVal, sym.SymType); ok {
			fl.blk.Ptr.Store(addr, slot)
		}
		return
	}
	if ctor := fl.u.res.Info.Ctors[init]; ctor != nil {
		fl.construct(slot, classOf(sym.SymType), ctor, init)
		return
	}
	if init.Braced != nil {
		fl.initList(slot, sym.SymType, init.Braced)
		return
	}
	if initVal == nil {
		if rec := classOf(sym.SymType); rec != nil {
			fl.defaultConstruct(slot, rec, init.Pos())
		}
		return
	}
	if rec := classOf(sym.SymType); rec != nil {
		fl.installVPtr(slot, rec)
	}
	if rec := classOf(sym.SymType); rec != nil {
		fl.exprInto(slot, initVal, rec)
		return
	}
	if arr, isArr := types.Unqualify(sym.SymType).(*types.Array); isArr {
		if lit, isLit := unparen(initVal).(*ast.StringLit); isLit {
			fl.initArrayFromString(slot, arr, lit)
			return
		}
		if src, _, ok := fl.lvalue(initVal); ok {
			size, _ := fl.u.sizeAlign(arr)
			fl.blk.MemCpy(slot, src, fl.blk.I64.Const(size))
			return
		}
	}
	v := fl.expr(initVal)
	if v == nil {
		return
	}
	fl.store(slot, fl.convert(v, fl.typeOf(initVal), sym.SymType), sym.SymType)
}

// construct calls a constructor on freshly reserved storage.
func (fl *fn) construct(obj ir.Ptr, rec *types.Record, ctor *sema.FuncSymbol, init *ast.InitDeclarator) {
	var argExprs []ast.Expr
	switch {
	case len(init.Args) > 0:
		argExprs = init.Args
	case init.Braced != nil:
		for _, item := range init.Braced.Items {
			if e, isExpr := item.(ast.Expr); isExpr {
				argExprs = append(argExprs, e)
			}
		}
	case init.Value != nil:
		// `T t = u;` -- the copy constructor's one argument.
		argExprs = []ast.Expr{init.Value}
	}
	fl.constructObject(obj, rec, ctor, argExprs, init.Pos())
}

// constructObject constructs an object using its own constructor or an
// inherited base constructor.
func (fl *fn) constructObject(obj ir.Ptr, rec *types.Record, ctor *sema.FuncSymbol, argExprs []ast.Expr, at ast.Tok) {
	if rec == nil || ctor.InClass == rec {
		fl.constructWith(obj, ctor, argExprs, at)
		return
	}
	size, _ := fl.u.sizeAlign(rec)
	fl.blk.MemSet(obj, fl.blk.I32.Const(0), fl.blk.I64.Const(size))
	baseOffs := make([]int64, len(rec.Bases))
	fl.u.model.LayoutWithBases(rec, make([]int64, len(rec.Fields)), baseOffs)
	for i, b := range rec.Bases {
		br := classOf(b.Type)
		if br == nil {
			continue
		}
		sub := obj
		if baseOffs[i] != 0 {
			sub = fl.blk.Ptr.Add(obj, fl.blk.I64.Const(baseOffs[i]))
		}
		if br == ctor.InClass {
			fl.constructWith(sub, ctor, argExprs, at)
			continue
		}
		fl.defaultConstruct(sub, br, at)
	}
	fl.installVPtr(obj, rec)
	fl.defaultMembers(obj, rec, nil, at)
}

// constructWith calls a constructor on an object with the given arguments,
// each brought to its parameter's type the way a call's are.
func (fl *fn) constructWith(obj ir.Ptr, ctor *sema.FuncSymbol, argExprs []ast.Expr, at ast.Tok) {
	if ctor.Consteval {
		// Immediate functions evaluate at compile time; empty classes need no runtime call.
		if rec := ctor.InClass; rec != nil && len(rec.Fields) == 0 && len(rec.Bases) == 0 {
			for _, a := range argExprs {
				fl.expr(a)
			}
			return
		}
		fl.u.errorf(at, "lowering: a consteval constructor of a class with members is not evaluated at compile time yet")
		return
	}
	target := fl.u.callee(ctor)
	if target == nil {
		fl.u.errorf(at, "lowering has no symbol for the constructor of %s", ctor.InClass.Name)
		return
	}
	args := []ir.Value{obj}
	for i, a := range argExprs {
		var want types.Type
		if i < len(ctor.FuncType.Params) {
			want = ctor.FuncType.Params[i].Type
		}
		if isReference(want) {
			addr, ok := fl.bind(a, want)
			if !ok {
				return
			}
			args = append(args, addr)
			continue
		}
		v := fl.expr(a)
		if v == nil {
			return
		}
		args = append(args, fl.convert(v, fl.typeOf(a), want))
	}
	fl.blk.Call(target, args...)
}

// initList writes a braced-init-list into storage. Trailing elements are value-initialized.
func (fl *fn) initList(dst ir.Ptr, t types.Type, list *ast.InitList) {
	if rec := classOf(t); rec != nil {
		// Aggregate initialization of bases and members in declaration order.
		fieldOffs := make([]int64, len(rec.Fields))
		baseOffs := make([]int64, len(rec.Bases))
		fl.u.model.LayoutWithBases(rec, fieldOffs, baseOffs)
		items := list.Items
		for i, b := range rec.Bases {
			at := dst
			if baseOffs[i] != 0 {
				at = fl.blk.Ptr.Add(dst, fl.blk.I64.Const(baseOffs[i]))
			}
			if len(items) > 0 {
				if inner, isList := items[0].(*ast.InitList); isList {
					fl.initList(at, b.Type, inner)
				} else if x, isExpr := items[0].(ast.Expr); isExpr {
					fl.exprInto(at, x, classOf(b.Type))
				}
				items = items[1:]
				continue
			}
			fl.initList(at, b.Type, &ast.InitList{})
		}
		for i, f := range rec.Fields {
			at := dst
			if fieldOffs[i] != 0 {
				at = fl.blk.Ptr.Add(dst, fl.blk.I64.Const(fieldOffs[i]))
			}
			if i < len(items) {
				switch item := items[i].(type) {
				case *ast.InitList:
					fl.initList(at, f.Type, item)
					continue
				case ast.Expr:
					if fr := classOf(f.Type); fr != nil {
						fl.exprInto(at, item, fr)
						continue
					}
					if val := fl.expr(item); val != nil {
						fl.store(at, fl.convert(val, fl.typeOf(item), f.Type), f.Type)
						continue
					}
				}
			}
			// Trailing member without item: default member initializer or value-initialization.
			if decl := fl.u.res.Info.MemberInits[rec][f.Name]; decl != nil && f.Name != "" {
				fl.memberDefault(at, f.Type, decl)
				continue
			}
			if fr := classOf(f.Type); fr != nil {
				size, _ := fl.u.sizeAlign(fr)
				fl.blk.MemSet(at, fl.blk.I32.Const(0), fl.blk.I64.Const(size))
				if fl.u.needsConstruction(fr) {
					fl.defaultConstruct(at, fr, list.Pos())
				}
				continue
			}
			if _, isArr := types.Unqualify(f.Type).(*types.Array); isArr {
				fl.initList(at, f.Type, &ast.InitList{})
				continue
			}
			fl.store(at, fl.zeroOf(f.Type), f.Type)
		}
		return
	}

	arr, isArr := types.Unqualify(t).(*types.Array)
	if !isArr {
		// A scalar in braces is just the value.
		if len(list.Items) == 1 {
			if v, isExpr := list.Items[0].(ast.Expr); isExpr {
				if val := fl.expr(v); val != nil {
					fl.store(dst, fl.convert(val, fl.typeOf(v), t), t)
				}
			}
		}
		return
	}

	elemSize, _ := fl.u.sizeAlign(arr.Elem)
	for i := int64(0); i < arr.Len; i++ {
		at := dst
		if i > 0 {
			at = fl.blk.Ptr.Add(dst, fl.blk.I64.Const(i*elemSize))
		}
		if i < int64(len(list.Items)) {
			switch item := list.Items[i].(type) {
			case *ast.InitList:
				fl.initList(at, arr.Elem, item)
				continue
			case ast.Expr:
				if rec := classOf(arr.Elem); rec != nil {
					if fl.exprInto(at, item, rec) {
						continue
					}
				}
				if val := fl.expr(item); val != nil {
					fl.store(at, fl.convert(val, fl.typeOf(item), arr.Elem), arr.Elem)
					continue
				}
			}
		}
		if rec := classOf(arr.Elem); rec != nil {
			size, _ := fl.u.sizeAlign(rec)
			fl.blk.MemSet(at, fl.blk.I32.Const(0), fl.blk.I64.Const(size))
			continue
		}
		fl.store(at, fl.zeroOf(arr.Elem), arr.Elem)
	}
}

// localSymbol returns the declaration symbol recorded by sema for this declarator.
func (fl *fn) localSymbol(init *ast.InitDeclarator) *sema.VarSymbol {
	if fl.u.res.Info == nil {
		return nil
	}
	v, _ := fl.u.res.Info.Defs[init].(*sema.VarSymbol)
	return v
}

func (fl *fn) returnStmt(s *ast.ReturnStmt) {
	if s.X == nil {
		fl.destroyFrom(0)
		if fl.u.ctorReturnsThis(fl.sym) {
			fl.blk.Return(fl.this)
		} else {
			fl.blk.Return()
		}
		fl.blk = nil
		return
	}
	ret := fl.sym.FuncType.Ret
	if rec := classOf(ret); rec != nil && fl.hasSRet {
		// Return in caller-provided sret storage directly or via copy.
		if !fl.exprInto(fl.sret, s.X, rec) {
			fl.returnDefault()
			return
		}
		fl.endFullExpr()
		fl.destroyFrom(0)
		fl.blk.Return()
		fl.blk = nil
		return
	}
	if isReference(ret) {
		// Return reference address.
		addr, ok := fl.bind(s.X, ret)
		if !ok {
			fl.returnDefault()
			return
		}
		fl.destroyFrom(0)
		fl.blk.Return(addr)
		fl.blk = nil
		return
	}
	v := fl.expr(s.X)
	if v == nil {
		fl.returnDefault()
		return
	}
	// Evaluate return expression before destroying scope locals.
	v = fl.convert(v, fl.typeOf(s.X), ret)
	fl.endFullExpr()
	fl.destroyFrom(0)
	fl.blk.Return(v)
	fl.blk = nil
}

func (fl *fn) ifStmt(s *ast.IfStmt) {
	// Init-statement scope spans the entire if statement.
	fl.pushScope()
	defer fl.popScope()
	if s.Init != nil {
		fl.stmt(s.Init)
		if fl.blk == nil {
			return
		}
	}

	cond, ok := s.Cond.(ast.Expr)
	if !ok {
		fl.u.errorf(s.Pos(), "lowering does not handle a declaration as a condition yet")
		return
	}
	if s.Constexpr.IsValid() {
		// Lower only the active branch of if constexpr.
		n, known := fl.u.res.Info.Consts[cond]
		if !known {
			fl.u.errorf(s.Pos(), "lowering has no decision for this if constexpr")
			return
		}
		if n != 0 {
			fl.stmt(s.Then)
		} else if s.Else != nil {
			fl.stmt(s.Else)
		}
		return
	}
	c := fl.truth(cond)
	if c == nil {
		return
	}
	fl.endFullExpr()

	then := fl.block("if_then")
	join := fl.block("if_join")
	els := join
	if s.Else != nil {
		els = fl.block("if_else")
	}

	fl.blk.BrIf(*c, then.To(), els.To())

	fl.blk = then
	fl.stmt(s.Then)
	if fl.blk != nil {
		fl.blk.Br(join.To())
	}

	if s.Else != nil {
		fl.blk = els
		fl.stmt(s.Else)
		if fl.blk != nil {
			fl.blk.Br(join.To())
		}
	}

	fl.blk = join
}

func (fl *fn) whileStmt(s *ast.WhileStmt) {
	header := fl.block("while_head")
	body := fl.block("while_body")
	exit := fl.block("while_exit")

	fl.blk.Br(header.To())

	fl.blk = header
	cond, ok := s.Cond.(ast.Expr)
	if !ok {
		fl.u.errorf(s.Pos(), "lowering does not handle a declaration as a condition yet")
		return
	}
	c := fl.truth(cond)
	if c == nil {
		return
	}
	fl.endFullExpr()
	fl.blk.BrIf(*c, body.To(), exit.To())

	fl.loop(body, header, exit, s.Body, header)
}

func (fl *fn) doStmt(s *ast.DoStmt) {
	body := fl.block("do_body")
	cont := fl.block("do_cond")
	exit := fl.block("do_exit")

	// Do-while body executes before condition evaluation.
	fl.blk.Br(body.To())

	fl.loop(body, cont, exit, s.Body, cont)

	fl.blk = cont
	c := fl.truth(s.Cond)
	if c == nil {
		return
	}
	fl.endFullExpr()
	fl.blk.BrIf(*c, body.To(), exit.To())
	fl.blk = exit
}

func (fl *fn) forStmt(s *ast.ForStmt) {
	// Init-statement scope spans the for statement.
	fl.pushScope()
	defer fl.popScope()
	if s.Init != nil {
		fl.stmt(s.Init)
		if fl.blk == nil {
			return
		}
	}

	header := fl.block("for_head")
	body := fl.block("for_body")
	step := fl.block("for_step")
	exit := fl.block("for_exit")

	fl.blk.Br(header.To())
	fl.blk = header

	// Omitted condition defaults to true.
	if cond, ok := s.Cond.(ast.Expr); ok && cond != nil {
		c := fl.truth(cond)
		if c == nil {
			return
		}
		fl.endFullExpr()
		fl.blk.BrIf(*c, body.To(), exit.To())
	} else {
		fl.blk.Br(body.To())
	}

	fl.loop(body, step, exit, s.Body, step)

	fl.blk = step
	if s.Post != nil {
		fl.expr(s.Post)
		fl.endFullExpr()
	}
	fl.blk.Br(header.To())
	fl.blk = exit
}

// loop walks a loop body with break and continue pointing where they should,
// and joins the body's fall-through to next.
func (fl *fn) loop(body, continueTo, exit *ir.Block, stmt ast.Stmt, next *ir.Block) {
	oldBreak, oldContinue, oldDepth := fl.breakTo, fl.continueTo, fl.loopDepth
	fl.breakTo, fl.continueTo, fl.loopDepth = exit, continueTo, len(fl.scopes)

	fl.blk = body
	fl.stmt(stmt)
	if fl.blk != nil {
		fl.blk.Br(next.To())
	}

	fl.breakTo, fl.continueTo, fl.loopDepth = oldBreak, oldContinue, oldDepth
	fl.blk = exit
}

// rangeForStmt lowers a range-based for loop over an array.
func (fl *fn) rangeForStmt(s *ast.RangeForStmt) {
	// Range-for statement scope spans the whole statement.
	fl.pushScope()
	defer fl.popScope()
	if s.Init != nil {
		fl.stmt(s.Init)
		if fl.blk == nil {
			return
		}
	}

	if proto := fl.u.res.Info.Ranges[s]; proto != nil {
		fl.rangeForClass(s, proto)
		return
	}
	rangeType := fl.typeOf(s.Range)
	arr, isArr := types.Unqualify(types.RemoveReference(rangeType)).(*types.Array)
	if !isArr {
		fl.u.errorf(s.Range.Pos(), "lowering: ranged loop over non-array type is not supported yet")
		return
	}

	var base ir.Ptr
	if list, isList := s.Range.(*ast.InitList); isList {
		base = fl.alloc(arr, "__range")
		fl.initList(base, arr, list)
	} else {
		var ok bool
		base, _, ok = fl.arrayBase(s.Range)
		if !ok {
			fl.u.errorf(s.Range.Pos(), "lowering: could not determine array base for ranged loop")
			return
		}
	}

	sd, ok := s.Decl.(*ast.SimpleDecl)
	if !ok || len(sd.Inits) == 0 {
		fl.u.errorf(s.Pos(), "lowering: unsupported ranged loop declaration")
		return
	}
	init := sd.Inits[0]
	sym := fl.localSymbol(init)
	if sym == nil {
		fl.u.errorf(s.Pos(), "lowering: no symbol for ranged loop variable")
		return
	}

	elemSize, _ := fl.u.sizeAlign(arr.Elem)
	idxSlot := fl.alloc(types.Typ(types.LongLong), "__i")
	fl.blk.I64.Store(fl.blk.I64.Const(0), idxSlot)

	varSlot := fl.alloc(sym.SymType, sym.SymName)
	fl.slots[sym] = varSlot

	head := fl.block("range_head")
	body := fl.block("range_body")
	step := fl.block("range_step")
	exit := fl.block("range_exit")

	fl.blk.Br(head.To())

	fl.blk = head
	curI := fl.blk.I64.Load(idxSlot)
	cond := fl.blk.I64.SLt(curI, fl.blk.I64.Const(arr.Len))
	fl.blk.BrIf(cond, body.To(), exit.To())

	oldBreak, oldContinue, oldDepth := fl.breakTo, fl.continueTo, fl.loopDepth
	fl.breakTo, fl.continueTo, fl.loopDepth = exit, step, len(fl.scopes)

	fl.blk = body
	fl.pushScope()

	iterI := fl.blk.I64.Load(idxSlot)
	offset := fl.blk.I64.Mul(iterI, fl.blk.I64.Const(elemSize))
	elemAddr := fl.blk.Ptr.Add(base, offset)

	if isReference(sym.SymType) {
		fl.blk.Ptr.Store(elemAddr, varSlot)
	} else {
		if rec := classOf(sym.SymType); rec != nil {
			fl.copyObject(varSlot, elemAddr, rec, nil)
			fl.track(varSlot, sym.SymType)
		} else {
			elemVal := fl.load(elemAddr, arr.Elem)
			conv := fl.convert(elemVal, arr.Elem, sym.SymType)
			fl.store(varSlot, conv, sym.SymType)
		}
	}

	fl.stmt(s.Body)
	fl.popScope()
	if fl.blk != nil {
		fl.blk.Br(step.To())
	}

	fl.breakTo, fl.continueTo, fl.loopDepth = oldBreak, oldContinue, oldDepth

	fl.blk = step
	stepI := fl.blk.I64.Load(idxSlot)
	nextI := fl.blk.I64.Add(stepI, fl.blk.I64.Const(1))
	fl.blk.I64.Store(nextI, idxSlot)
	fl.blk.Br(head.To())

	fl.blk = exit
}
