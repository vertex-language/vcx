package lower

// C++ exceptions, the Itanium way.
//
// A throw puts the object in storage the runtime owns and hands it to
// __cxa_throw, which unwinds. Unwinding runs in two phases: the
// personality routine walks the frames looking for a handler, and then the
// stack is unwound to it, entering each frame's landing pad on the way so
// that it can destroy what it owns. A pad is a block the IR marks with the
// clauses it answers -- a catch for each handler's type, or a cleanup for a
// frame that only has destructors to run.
//
// What decides which handler runs is the pad's selector: the backend
// delivers the position of the matching clause among the pad's catch
// clauses, counting from one, and zero when the pad was entered only to
// clean up. So a handler is chosen by comparing against a constant, and a
// pad that matched nothing resumes.

import (
	"fmt"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// The runtime the Itanium ABI defines for C++ exceptions.
func (u *unit) cxaAllocate() ir.Callee {
	return u.rtImport("__cxa_allocate_exception", ir.NewSig().Param(u.regType(u.sizeT())).Ret(ir.TypePtr))
}

func (u *unit) cxaFree() ir.Callee {
	return u.rtImport("__cxa_free_exception", ir.NewSig().Param(ir.TypePtr))
}

func (u *unit) cxaThrow() ir.Callee {
	return u.rtImport("__cxa_throw", ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypePtr))
}

func (u *unit) cxaRethrow() ir.Callee {
	return u.rtImport("__cxa_rethrow", ir.NewSig())
}

func (u *unit) cxaBeginCatch() ir.Callee {
	return u.rtImport("__cxa_begin_catch", ir.NewSig().Param(ir.TypePtr).Ret(ir.TypePtr))
}

func (u *unit) cxaEndCatch() ir.Callee {
	return u.rtImport("__cxa_end_catch", ir.NewSig())
}

// personality is the routine the unwinder asks about this frame. Naming it
// is what makes the backend write the tables at all.
func (u *unit) personality() ir.Callee {
	return u.rtImport("__gxx_personality_v0", ir.NewSig().Ret(ir.TypeI32))
}

// needsEH marks the function as one with exception tables, and names the
// personality that reads them.
func (fl *fn) needsEH() {
	if fl.ehDeclared {
		return
	}
	fl.ehDeclared = true
	fl.f.Personality(fl.u.personality())
}

// call emits a call, or an invoke where the exception would have somewhere
// to go. What it returns are the callee's results.
//
// An invoke has no results of its own: §G3 binds them to the trailing
// parameters of the block its normal edge names, so the continuation is
// made here and becomes the current block. Every call in this package goes
// through this, because any of them can be the one that throws.
func (fl *fn) emitCall(target ir.Callee, args ...ir.Value) []ir.Value {
	if fl.blk == nil || target == nil {
		return nil
	}
	pad := fl.unwindPad()
	if pad == nil {
		res := fl.blk.Call(target, args...)
		out := make([]ir.Value, res.Len())
		for i := range out {
			out[i] = res.Value(i)
		}
		return out
	}
	fl.needsEH()
	rets := target.Signature().Rets()
	cont := fl.block("resume")
	out := make([]ir.Value, 0, len(rets))
	for i, r := range rets {
		out = append(out, cont.Param(r.Type, fmt.Sprintf("r%d", i)))
	}
	fl.blk.Invoke(target, args, cont.To(), pad)
	fl.blk = cont
	return out
}

// call1 is call where one result is wanted and nothing else will do.
func (fl *fn) emitCall1(target ir.Callee, args ...ir.Value) ir.Value {
	res := fl.emitCall(target, args...)
	if len(res) == 0 {
		return nil
	}
	return res[0]
}

// unwindPad is where a call in the current region unwinds to: the handler
// of the try it sits in, or a pad that destroys what is live and lets the
// unwinding go on. Nil when there is neither, and then a call is a call.
func (fl *fn) unwindPad() *ir.Block {
	if fl.u.devicePass() {
		// A GPU has no unwinder, and the device pass has no tables to
		// write. Code there is compiled as though nothing throws.
		return nil
	}
	if fl.inEH {
		// Inside a pad's own code. A destructor that throws while
		// unwinding is std::terminate's business, not another pad's.
		return nil
	}
	if fl.ehPad != nil {
		return fl.ehPad
	}
	if !fl.hasLiveObjects() {
		return nil
	}
	return fl.cleanupPad()
}

// hasLiveObjects reports whether anything with a destructor is live.
func (fl *fn) hasLiveObjects() bool {
	if len(fl.temps) > 0 || len(fl.partial) > 0 {
		return true
	}
	for _, s := range fl.scopes {
		if len(s.objs) > 0 {
			return true
		}
	}
	return false
}

// cleanupPad builds a pad that destroys what is live here and lets the
// exception go on to the next frame.
//
// One per site, because what is live changes as a block proceeds: the pad
// for a call before a local is constructed must not destroy it. They are
// blocks and nothing else, so the cost is code size in a function that
// throws through, which is the path already paying for an unwind.
func (fl *fn) cleanupPad() *ir.Block {
	fl.needsEH()
	pad := fl.f.Pad(fmt.Sprintf("cleanup_%d", fl.nextPad()), ir.Cleanup)
	if pad == nil {
		return nil
	}
	saved, savedIn := fl.blk, fl.inEH
	fl.blk, fl.inEH = pad, true
	fl.destroyLive()
	fl.blk.Resume(pad.Exn())
	fl.blk, fl.inEH = saved, savedIn
	return pad
}

// destroyLive runs every live destructor, innermost and latest first: the
// temporaries of the full-expression being evaluated, then each scope.
func (fl *fn) destroyLive() {
	for i := len(fl.temps) - 1; i >= 0; i-- {
		fl.destroy(fl.temps[i].addr, fl.temps[i].rec)
	}
	fl.destroyFrom(0)

	// Then what a constructor has built of the object itself, members
	// before bases, each in reverse order of construction.
	for i := len(fl.partial) - 1; i >= 0; i-- {
		fl.destroy(fl.partial[i].addr, fl.partial[i].rec)
	}
}

// nextPad numbers the pads of a function apart.
func (fl *fn) nextPad() int {
	fl.npads++
	return fl.npads
}

// throwExpr lowers `throw x` and the bare `throw`.
//
// The object goes in storage the runtime owns, because it outlives this
// frame: __cxa_allocate_exception hands it over, the object is built
// there, and __cxa_throw takes it with the type-info that says what it is
// and the destructor that ends it. A bare throw re-raises the exception
// the handler it sits in is holding.
func (fl *fn) throwExpr(e *ast.ThrowExpr) ir.Value {
	if e.X == nil {
		fl.emitCall(fl.u.cxaRethrow())
		fl.unreachable(e.Pos())
		return nil
	}
	t := types.Unqualify(types.RemoveReference(fl.typeOf(e.X)))
	size, _ := fl.u.sizeAlign(t)
	obj, isPtr := fl.emitCall1(fl.u.cxaAllocate(), fl.blk.I64.Const(size)).(ir.Ptr)
	if !isPtr {
		fl.u.errorf(e.Pos(), "lowering: the exception storage is not an address")
		return nil
	}

	// The object is built where it will be thrown from, so that a throw of
	// a class is one construction and not a copy of a temporary.
	if rec := classOf(t); rec != nil {
		if !fl.exprInto(obj, e.X, rec) {
			return nil
		}
	} else {
		v := fl.expr(e.X)
		if v == nil {
			return nil
		}
		fl.store(obj, fl.convert(v, fl.typeOf(e.X), t), t)
	}

	ti := fl.u.typeInfoSymbol(t)
	if ti == nil {
		fl.u.errorf(e.Pos(), "lowering: no type information for the thrown %s", t)
		return nil
	}
	dtor := fl.blk.Ptr.Const()
	if rec := classOf(t); rec != nil && fl.u.needsDestructor(rec) {
		if d := fl.u.destructor(rec); d != nil {
			dtor = fl.blk.Ptr.GetAddr(d)
		}
	}
	fl.emitCall(fl.u.cxaThrow(), obj, fl.blk.Ptr.GetAddr(ti), dtor)
	fl.unreachable(e.Pos())
	return nil
}

// unreachable ends the block after a call that does not return.
// __cxa_throw and __cxa_rethrow both unwind, and the block still needs a
// terminator: trap is the one that says control does not arrive here.
func (fl *fn) unreachable(at ast.Tok) {
	if fl.blk == nil {
		return
	}
	fl.blk.Trap()
	fl.blk = nil
}

// tryStmt lowers try/catch.
//
// The body runs with its calls aimed at one pad, which names a clause per
// handler in order -- a catch of the handler's type, or a catch of nothing
// at all for `catch (...)`. The pad compares the selector the personality
// left against each clause's position and enters the handler that matched;
// what matched nothing resumes, which is what a cleanup-only entry does.
func (fl *fn) tryStmt(s *ast.TryStmt) {
	if s.Body == nil {
		return
	}
	fl.needsEH()
	clauses := make([]ir.PadClause, 0, len(s.Handlers))
	for _, h := range s.Handlers {
		clauses = append(clauses, ir.Catch(fl.handlerTypeInfo(h)))
	}
	if len(clauses) == 0 {
		fl.stmt(s.Body)
		return
	}
	pad := fl.f.Pad(fmt.Sprintf("catch_%d", fl.nextPad()), clauses...)
	if pad == nil {
		return
	}
	done := fl.block("try_done")

	// The body. Its own destructors run on the normal path as ever; on the
	// unwinding path the pad is the one that was named.
	savedPad := fl.ehPad
	fl.ehPad = pad
	fl.pushScope()
	fl.stmt(s.Body)
	fl.popScope()
	fl.ehPad = savedPad
	if fl.blk != nil {
		fl.blk.Br(done.To())
	}

	// The pad: the selector says which clause matched, counting from one.
	fl.blk = pad
	exn, sel := pad.Exn(), pad.Sel()
	for i, h := range s.Handlers {
		hit := fl.block(fmt.Sprintf("catch_hit_%d", fl.nextPad()))
		next := fl.block(fmt.Sprintf("catch_next_%d", fl.nextPad()))
		fl.blk.BrIf(fl.blk.I32.Eq(sel, fl.blk.I32.Const(int64(i+1))), hit.To(), next.To())
		fl.blk = hit
		fl.handler(h, exn, done)
		fl.blk = next
	}
	// Nothing matched: keep unwinding.
	fl.blk.Resume(exn)
	fl.blk = done
}

// handlerTypeInfo is the type-info a handler catches by, or nil for
// `catch (...)`, which the tables spell as a clause with no type at all.
func (fl *fn) handlerTypeInfo(h *ast.CatchClause) ir.Symbol {
	if h.Param == nil || h.Param.Decl == nil {
		return nil
	}
	sym, _ := fl.u.res.Info.Defs[h.Param].(*sema.VarSymbol)
	if sym == nil {
		return nil
	}
	// A handler catches by the type named, with the reference and the
	// qualifiers taken off: `catch (const Error&)` catches an Error.
	return fl.u.typeInfoSymbol(types.Unqualify(types.RemoveReference(sym.SymType)))
}

// handler runs one catch clause: the runtime hands over the object, the
// parameter is bound to it, the body runs, and the catch ends however the
// body leaves.
func (fl *fn) handler(h *ast.CatchClause, exn ir.Ptr, done *ir.Block) {
	caught, isPtr := fl.emitCall1(fl.u.cxaBeginCatch(), exn).(ir.Ptr)
	if !isPtr {
		fl.u.errorf(h.Pos(), "lowering: __cxa_begin_catch did not answer with an address")
		return
	}
	fl.pushScope()
	if sym, _ := fl.u.res.Info.Defs[h.Param].(*sema.VarSymbol); sym != nil && sym.SymName != "" {
		fl.bindCaught(sym, caught)
	}
	if h.Body != nil {
		fl.stmt(h.Body)
	}
	fl.popScope()
	if fl.blk != nil {
		fl.emitCall(fl.u.cxaEndCatch())
		fl.blk.Br(done.To())
	}
}

// bindCaught gives the handler's parameter the object the runtime handed
// over: a reference names it, and anything else is a copy of it.
func (fl *fn) bindCaught(sym *sema.VarSymbol, caught ir.Ptr) {
	if isReference(sym.SymType) {
		slot := fl.alloc(&types.Pointer{Elem: types.RemoveReference(sym.SymType)}, sym.SymName)
		fl.blk.Ptr.Store(caught, slot)
		fl.slots[sym] = slot
		return
	}
	slot := fl.allocVar(sym)
	fl.slots[sym] = slot
	if rec := classOf(sym.SymType); rec != nil {
		fl.copyObject(slot, caught, rec, nil)
		fl.track(slot, sym.SymType)
		return
	}
	fl.store(slot, fl.load(caught, sym.SymType), sym.SymType)
}
