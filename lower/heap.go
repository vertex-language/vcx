package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/mangle"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// Lowering of new and delete expressions.
// Allocates storage via operator new / new[], invokes constructors,
// and invokes destructors followed by operator delete / delete[].

// newExpr allocates and constructs.
func (fl *fn) newExpr(e *ast.NewExpr) ir.Value {
	ptrT, isPtr := types.Unqualify(fl.typeOf(e)).(*types.Pointer)
	if !isPtr {
		fl.u.errorf(e.Pos(), "lowering: a new-expression that is not a pointer")
		return nil
	}
	elem := ptrT.Elem
	if count := sema.NewArrayCount(e); count != nil {
		return fl.newArray(e, elem, count)
	}
	if _, isArr := types.Unqualify(elem).(*types.Array); isArr {
		fl.u.errorf(e.Pos(), "lowering: a new-expression of array type with no bound")
		return nil
	}

	size, _ := fl.u.sizeAlign(elem)
	var res ir.Results
	if alloc := fl.u.res.Info.Allocs[e]; alloc != nil {
		// Placement new: invoke chosen allocation function with size and placement arguments.
		target := fl.u.callee(alloc)
		if target == nil {
			return nil
		}
		args := []ir.Value{fl.convert(fl.blk.I64.Const(size), fl.u.sizeT(), alloc.FuncType.Params[0].Type)}
		for i, p := range e.Placement {
			var want types.Type
			if i+1 < len(alloc.FuncType.Params) {
				want = alloc.FuncType.Params[i+1].Type
			}
			if isReference(want) {
				addr, ok := fl.bind(p, want)
				if !ok {
					return nil
				}
				args = append(args, addr)
				continue
			}
			v := fl.expr(p)
			if v == nil {
				return nil
			}
			args = append(args, fl.convert(v, fl.typeOf(p), want))
		}
		res = fl.blk.Call(target, args...)
	} else {
		if len(e.Placement) > 0 {
			fl.u.errorf(e.Pos(), "lowering has no allocation function for this placement new")
			return nil
		}
		if align := fl.u.overAlignment(elem); align != 0 {
			res = fl.blk.Call(fl.u.operatorNewAligned(), fl.blk.I64.Const(size), fl.blk.I64.Const(align))
		} else {
			res = fl.blk.Call(fl.u.operatorNew(), fl.blk.I64.Const(size))
		}
	}
	if res.Len() == 0 {
		return nil
	}
	obj, isP := res.Value(0).(ir.Ptr)
	if !isP {
		return nil
	}

	// Initialize object according to constructor or value-initialization.
	if ctor := fl.u.res.Info.News[e]; ctor != nil {
		fl.constructObject(obj, classOf(elem), ctor, e.Args, e.Pos())
		return obj
	}
	if rec := classOf(elem); rec != nil {
		if list, isList := e.Init.(*ast.InitList); isList {
			fl.initList(obj, elem, list)
			return obj
		}
		if e.Init != nil || len(e.Args) > 0 {
			// `new T()` -- value-initialized: zeroed, then constructed.
			size, _ := fl.u.sizeAlign(rec)
			fl.blk.MemSet(obj, fl.blk.I32.Const(0), fl.blk.I64.Const(size))
		}
		fl.defaultConstruct(obj, rec, e.Pos())
		return obj
	}
	// A scalar: `new int(5)`, `new int{5}`, or `new int` left alone.
	var initExpr ast.Expr
	switch {
	case len(e.Args) == 1:
		initExpr = e.Args[0]
	case e.Init != nil:
		if list, isList := e.Init.(*ast.InitList); isList {
			if len(list.Items) == 1 {
				initExpr = list.Items[0]
			} else if len(list.Items) == 0 {
				fl.store(obj, fl.zeroOf(elem), elem)
			}
		}
	}
	if initExpr != nil {
		if v := fl.expr(initExpr); v != nil {
			fl.store(obj, fl.convert(v, fl.typeOf(initExpr), elem), elem)
		}
	}
	return obj
}

func isArrayNew(e *ast.NewExpr) bool {
	for d := e.Type.Decl; d != nil; {
		switch node := d.(type) {
		case *ast.ArrayDeclarator:
			return true
		case *ast.PointerDeclarator:
			d = node.Inner
		case *ast.ParenDeclarator:
			d = node.Inner
		default:
			return false
		}
	}
	return false
}

// arrayCookie is what an array of objects with a destructor keeps before
// its first element: the count, in eight bytes -- sixteen under ARM's
// convention, which puts the element size in front of it -- or the
// element's alignment when that is larger, so the elements stay aligned.
// Zero for an element with nothing to destroy, whose count nobody needs.
func (u *unit) arrayCookie(elem types.Type) int64 {
	rec := classOf(elem)
	if rec == nil || !u.needsDestructor(rec) {
		return 0
	}
	_, align := u.sizeAlign(elem)
	cookie := int64(8)
	if u.model.ABI.ArrayCookieHasElementSize() {
		cookie = 16
	}
	if align > cookie {
		return align
	}
	return cookie
}

// newArray lowers `new T[n]`: calls operator new[] for n elements and cookie,
// stores cookie count when T has a destructor, and constructs elements in turn.
func (fl *fn) newArray(e *ast.NewExpr, elem types.Type, count ast.Expr) ir.Value {
	if len(e.Placement) > 0 {
		fl.u.errorf(e.Pos(), "lowering does not handle placement new[] yet")
		return nil
	}
	n := fl.expr(count)
	if n == nil {
		return nil
	}
	n64, isI64 := fl.convert(n, fl.typeOf(count), types.Typ(types.LongLong)).(ir.I64)
	if !isI64 {
		fl.u.errorf(count.Pos(), "lowering: the array bound is not an integer")
		return nil
	}
	size, _ := fl.u.sizeAlign(elem)
	cookie := fl.u.arrayCookie(elem)
	b := fl.blk
	total := b.I64.Mul(n64, b.I64.Const(size))
	if cookie != 0 {
		total = b.I64.Add(total, b.I64.Const(cookie))
	}
	var res ir.Results
	if align := fl.u.overAlignment(elem); align != 0 {
		res = b.Call(fl.u.operatorNewArrayAligned(), total, b.I64.Const(align))
	} else {
		res = b.Call(fl.u.operatorNewArray(), total)
	}
	if res.Len() == 0 {
		return nil
	}
	raw, isP := res.Value(0).(ir.Ptr)
	if !isP {
		return nil
	}
	obj := raw
	if cookie != 0 {
		obj = b.Ptr.Add(raw, b.I64.Const(cookie))
		if fl.u.model.ABI.IsMicrosoft() {
			b.I64.Store(n64, raw)
		} else {
			// Itanium keeps the count immediately before the first
			// element, whatever padding the element's alignment put in
			// front of it; ARM keeps the element size before that.
			b.I64.Store(n64, b.Ptr.Add(obj, b.I64.Const(-8)))
			if fl.u.model.ABI.ArrayCookieHasElementSize() {
				b.I64.Store(b.I64.Const(size), b.Ptr.Add(obj, b.I64.Const(-16)))
			}
		}
	}

	// Keep the count and the array in slots: the loop below is blocks.
	countSlot := fl.alloc(types.Typ(types.LongLong), "__n")
	b.I64.Store(n64, countSlot)
	rec := classOf(elem)
	ctor := fl.u.res.Info.News[e]
	zero := false
	switch init := e.Init.(type) {
	case *ast.InitList:
		zero = len(init.Items) == 0
	case *ast.ParenExpr:
		zero = len(e.Args) == 0
	}
	if ctor == nil && rec == nil && !zero {
		return obj // Default-initialized scalars are left uninitialized.
	}
	if ctor == nil && rec != nil && fl.u.vtablesOf(rec) == nil && !zero && !fl.u.hasFieldInits(rec) {
		return obj
	}

	// for (i = 0; i < n; ++i) construct(obj + i*size)
	idx := fl.alloc(types.Typ(types.LongLong), "__i")
	b.I64.Store(b.I64.Const(0), idx)
	head := fl.block("newarr_head")
	body := fl.block("newarr_body")
	exit := fl.block("newarr_exit")
	b.Br(head.To())

	fl.blk = head
	i := fl.blk.I64.Load(idx)
	fl.blk.BrIf(fl.blk.I64.SLt(i, fl.blk.I64.Load(countSlot)), body.To(), exit.To())

	fl.blk = body
	i = fl.blk.I64.Load(idx)
	at := fl.blk.Ptr.Add(obj, fl.blk.I64.Mul(i, fl.blk.I64.Const(size)))
	switch {
	case ctor != nil:
		fl.constructWith(at, ctor, e.Args, e.Pos())
	case rec != nil:
		if zero {
			fl.blk.MemSet(at, fl.blk.I32.Const(0), fl.blk.I64.Const(size))
		}
		fl.defaultConstruct(at, rec, e.Pos())
	default:
		fl.store(at, fl.zeroOf(elem), elem)
	}
	fl.blk.I64.Store(fl.blk.I64.Add(fl.blk.I64.Load(idx), fl.blk.I64.Const(1)), idx)
	fl.blk.Br(head.To())

	fl.blk = exit
	return obj
}

// hasFieldInits reports a class some member of which has a default
// member initializer, so that default-initializing it is not nothing.
func (u *unit) hasFieldInits(rec *types.Record) bool {
	for _, f := range rec.Fields {
		if f.HasInit {
			return true
		}
		if fr := classOf(f.Type); fr != nil && u.hasFieldInits(fr) {
			return true
		}
	}
	return false
}

// deleteExpr destroys and deallocates.
func (fl *fn) deleteExpr(e *ast.DeleteExpr) {
	if e.Lbrack != ast.NoTok {
		fl.deleteArray(e)
		return
	}
	v := fl.expr(e.X)
	if v == nil {
		return
	}
	p, isP := v.(ir.Ptr)
	if !isP {
		fl.u.errorf(e.Pos(), "lowering: the operand of delete is not a pointer")
		return
	}
	// Deleting a null pointer is a no-op; guard destructor and deallocation behind a null check.
	isNull := fl.blk.Ptr.Eq(p, fl.blk.Ptr.Const())
	do := fl.block("delete_do")
	done := fl.block("delete_done")
	fl.blk.BrIf(isNull, done.To(), do.To())

	fl.blk = do
	// The destructor: the one the analysis recorded for a class that
	// declares one, or the implicit one of a class that does not.
	dtor := fl.u.res.Info.Deletes[e]
	if dtor == nil {
		if ptr, isPtr := types.Unqualify(types.RemoveReference(fl.typeOf(e.X))).(*types.Pointer); isPtr {
			if rec := classOf(ptr.Elem); rec != nil {
				dtor = fl.u.destructorSymbol(rec)
			}
		}
	}
	if dtor != nil {
		// Virtual destructor: static type may be a base of the dynamic one.
		// The slot holds the deleting destructor, which frees as well when instructed.
		if tableOff, slot, virtual := fl.u.vslot(dtor); virtual {
			b := fl.blk
			sub := p
			if tableOff != 0 {
				sub = b.Ptr.Add(p, b.I64.Const(tableOff))
			}
			if fl.u.model.ABI.IsItanium() {
				// Itanium's D0 is the slot after D1, and takes no flags.
				entry := b.Ptr.Add(b.Ptr.Load(sub), b.I64.Const(int64(slot+1)*fl.u.model.SizePtr))
				b.CallInd(b.Ptr.Load(entry), fl.u.deletingDtorItaniumType(), sub)
				b.Br(done.To())
				fl.blk = done
				return
			}
			entry := b.Ptr.Load(sub)
			if slot > 0 {
				entry = b.Ptr.Add(entry, b.I64.Const(int64(slot)*fl.u.model.SizePtr))
			}
			target := b.Ptr.Load(entry)
			b.CallInd(target, fl.u.deletingDtorType(), sub, b.I32.Const(1))
			b.Br(done.To())
			fl.blk = done
			return
		}
		if d := fl.u.callee(dtor); d != nil {
			fl.blk.Call(d, p)
		}
	}
	fl.deallocate(p, fl.typeOf(e.X), false)
	fl.blk.Br(done.To())
	fl.blk = done
}

// deallocate calls operator delete or operator delete[], aligned when the type is over-aligned.
func (fl *fn) deallocate(p ir.Ptr, ptrType types.Type, array bool) {
	var elem types.Type
	if ptr, isPtr := types.Unqualify(types.RemoveReference(ptrType)).(*types.Pointer); isPtr {
		elem = ptr.Elem
	}
	align := int64(0)
	if elem != nil {
		align = fl.u.overAlignment(elem)
	}
	switch {
	case array && align != 0:
		fl.blk.Call(fl.u.operatorDeleteArrayAligned(), p, fl.blk.I64.Const(align))
	case array:
		fl.blk.Call(fl.u.operatorDeleteArray(), p)
	case align != 0:
		fl.blk.Call(fl.u.operatorDeleteAligned(), p, fl.blk.I64.Const(align))
	default:
		fl.blk.Call(fl.u.operatorDelete(), p)
	}
}

// overAlignment is a type's alignment when it exceeds what the plain
// allocation functions guarantee -- __STDCPP_DEFAULT_NEW_ALIGNMENT__,
// sixteen on every target here -- and zero otherwise.
func (u *unit) overAlignment(t types.Type) int64 {
	_, align := u.sizeAlign(t)
	if align > 16 {
		return align
	}
	return 0
}

// deleteArray lowers `delete[] p`: runs destructors in reverse order using the
// cookie count, then frees storage via operator delete[].
func (fl *fn) deleteArray(e *ast.DeleteExpr) {
	v := fl.expr(e.X)
	if v == nil {
		return
	}
	p, isP := v.(ir.Ptr)
	if !isP {
		fl.u.errorf(e.Pos(), "lowering: the operand of delete[] is not a pointer")
		return
	}
	ptr, isPtrType := types.Unqualify(types.RemoveReference(fl.typeOf(e.X))).(*types.Pointer)
	if !isPtrType {
		fl.u.errorf(e.Pos(), "lowering: the operand of delete[] has no pointee type")
		return
	}
	isNull := fl.blk.Ptr.Eq(p, fl.blk.Ptr.Const())
	do := fl.block("delete_do")
	done := fl.block("delete_done")
	fl.blk.BrIf(isNull, done.To(), do.To())

	fl.blk = do
	if rec := classOf(ptr.Elem); rec != nil && fl.u.needsDestructor(rec) {
		if fl.u.model.ABI.IsItanium() {
			fl.deleteArrayItanium(p, rec, fl.typeOf(e.X))
		} else if d := fl.u.deletingDtor(rec); d != nil {
			fl.blk.Call(d, p, fl.blk.I32.Const(3))
		}
	} else {
		fl.deallocate(p, fl.typeOf(e.X), true)
	}
	fl.blk.Br(done.To())
	fl.blk = done
}

// alignValT returns std::align_val_t for aligned allocation functions.
func (u *unit) alignValT() types.Type {
	return &types.Enum{Name: "align_val_t", Scopes: []string{"std"}, Scoped: true, Underlying: u.sizeT(), Complete: true}
}

// allocationFunction imports an allocation or deallocation runtime function.
func (u *unit) allocationFunction(name string, aligned bool) ir.Callee {
	var params []types.Param
	var ret types.Type
	sig := ir.NewSig()
	if name == "operator new" || name == "operator new[]" {
		params = []types.Param{{Type: u.sizeT()}}
		ret = &types.Pointer{Elem: types.Typ(types.Void)}
		sig = sig.Param(u.regType(u.sizeT()))
	} else {
		params = []types.Param{{Type: &types.Pointer{Elem: types.Typ(types.Void)}}}
		ret = types.Typ(types.Void)
		sig = sig.Param(ir.TypePtr)
	}
	if aligned {
		params = append(params, types.Param{Type: u.alignValT()})
		sig = sig.Param(u.regType(u.sizeT()))
	}
	if ret != nil && !types.IsVoid(ret) {
		sig = sig.Ret(ir.TypePtr)
	}
	mangled, err := mangle.FunctionName(u.opt.ABI, &mangle.Function{Name: name, Type: &types.Func{Ret: ret, Params: params}})
	if err != nil {
		u.errorf(ast.NoTok, "%v", err)
		return nil
	}
	return u.mod.ImportFunc(u.symbolName(mangled), sig)
}

func (u *unit) operatorNewAligned() ir.Callee {
	if u.opNewAligned == nil {
		u.opNewAligned = u.allocationFunction("operator new", true)
	}
	return u.opNewAligned
}

func (u *unit) operatorDeleteAligned() ir.Callee {
	if u.opDeleteAligned == nil {
		u.opDeleteAligned = u.allocationFunction("operator delete", true)
	}
	return u.opDeleteAligned
}

func (u *unit) operatorNewArrayAligned() ir.Callee {
	if u.opNewArrayAligned == nil {
		u.opNewArrayAligned = u.allocationFunction("operator new[]", true)
	}
	return u.opNewArrayAligned
}

func (u *unit) operatorDeleteArrayAligned() ir.Callee {
	if u.opDeleteArrayAligned == nil {
		u.opDeleteArrayAligned = u.allocationFunction("operator delete[]", true)
	}
	return u.opDeleteArrayAligned
}

// operatorNewArray is the import of `void *operator new[](std::size_t)`.
func (u *unit) operatorNewArray() ir.Callee {
	if u.opNewArray != nil {
		return u.opNewArray
	}
	name, err := mangle.FunctionName(u.opt.ABI, &mangle.Function{
		Name: "operator new[]",
		Type: &types.Func{Ret: &types.Pointer{Elem: types.Typ(types.Void)}, Params: []types.Param{{Type: u.sizeT()}}},
	})
	if err != nil {
		u.errorf(ast.NoTok, "%v", err)
		return nil
	}
	u.opNewArray = u.mod.ImportFunc(u.symbolName(name), ir.NewSig().Param(u.regType(u.sizeT())).Ret(ir.TypePtr))
	return u.opNewArray
}

// operatorDeleteArray is the import of `void operator delete[](void *)`.
func (u *unit) operatorDeleteArray() ir.Callee {
	if u.opDeleteArray != nil {
		return u.opDeleteArray
	}
	name, err := mangle.FunctionName(u.opt.ABI, &mangle.Function{
		Name: "operator delete[]",
		Type: &types.Func{Ret: types.Typ(types.Void), Params: []types.Param{{Type: &types.Pointer{Elem: types.Typ(types.Void)}}}},
	})
	if err != nil {
		u.errorf(ast.NoTok, "%v", err)
		return nil
	}
	u.opDeleteArray = u.mod.ImportFunc(u.symbolName(name), ir.NewSig().Param(ir.TypePtr))
	return u.opDeleteArray
}

// deletingDtorType is the signature an indirect call to a deleting
// destructor carries: the object and the flags, returning the object.
func (u *unit) deletingDtorType() *ir.Type {
	if u.deletingDtorSig == nil {
		u.deletingDtorSig = u.mod.FuncType("sig_deleting_dtor", ir.NewSig().Param(ir.TypePtr).Param(ir.TypeI32).Ret(ir.TypePtr))
	}
	return u.deletingDtorSig
}

// operatorNew is the import of `void *operator new(std::size_t)`.
func (u *unit) operatorNew() ir.Callee {
	if u.opNew != nil {
		return u.opNew
	}
	name, err := mangle.FunctionName(u.opt.ABI, &mangle.Function{
		Name: "operator new",
		Type: &types.Func{Ret: &types.Pointer{Elem: types.Typ(types.Void)}, Params: []types.Param{{Type: u.sizeT()}}},
	})
	if err != nil {
		u.errorf(ast.NoTok, "%v", err)
		return nil
	}
	u.opNew = u.mod.ImportFunc(u.symbolName(name), ir.NewSig().Param(u.regType(u.sizeT())).Ret(ir.TypePtr))
	return u.opNew
}

// operatorDelete is the import of `void operator delete(void *)`.
func (u *unit) operatorDelete() ir.Callee {
	if u.opDelete != nil {
		return u.opDelete
	}
	name, err := mangle.FunctionName(u.opt.ABI, &mangle.Function{
		Name: "operator delete",
		Type: &types.Func{Ret: types.Typ(types.Void), Params: []types.Param{{Type: &types.Pointer{Elem: types.Typ(types.Void)}}}},
	})
	if err != nil {
		u.errorf(ast.NoTok, "%v", err)
		return nil
	}
	u.opDelete = u.mod.ImportFunc(u.symbolName(name), ir.NewSig().Param(ir.TypePtr))
	return u.opDelete
}

// sizeT is std::size_t on this target: the unsigned integer the pointer
// is the width of, which is unsigned long long under Microsoft and
// unsigned long everywhere else this compiles for.
func (u *unit) sizeT() types.Type {
	if u.model.SizeLong < u.model.SizePtr {
		return types.Typ(types.ULongLong)
	}
	return types.Typ(types.ULong)
}
