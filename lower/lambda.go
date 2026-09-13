package lower

import (
	"strings"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// Lowering of lambda-expressions into closure objects and operator() invocations.

// isClosure is whether a class is a lambda's, by the name cl gives one.
func isClosure(rec *types.Record) bool {
	return rec != nil && strings.HasPrefix(rec.Name, "<lambda_")
}

// lambda makes the closure object of a lambda-expression, a temporary of
// the full-expression, and is its address.
func (fl *fn) lambda(e *ast.LambdaExpr) ir.Value {
	info := fl.u.res.Info.Lambdas[e]
	if info == nil {
		fl.u.errorf(e.Pos(), "lowering has no closure for this lambda")
		return nil
	}
	rec := info.Closure
	obj := fl.alloc(rec, "")
	fl.temporary(obj, rec)

	fieldOffs := make([]int64, len(rec.Fields))
	fl.u.model.LayoutWithBases(rec, fieldOffs, make([]int64, len(rec.Bases)))
	for _, c := range info.Captures {
		field := rec.Fields[c.Field]
		at := obj
		if fieldOffs[c.Field] != 0 {
			at = fl.blk.Ptr.Add(obj, fl.blk.I64.Const(fieldOffs[c.Field]))
		}
		switch {
		case c.Init != nil:
			// An init-capture: the member from its initializer, as a
			// declaration's would be.
			if c.ByRef {
				if addr, ok := fl.bind(c.Init, field.Type); ok {
					fl.blk.Ptr.Store(addr, at)
				}
				continue
			}
			if cr := classOf(field.Type); cr != nil {
				fl.exprInto(at, c.Init, cr)
				continue
			}
			if v := fl.expr(c.Init); v != nil {
				fl.store(at, fl.convert(v, fl.typeOf(c.Init), field.Type), field.Type)
			}
		case c.ByRef:
			// Reference capture binds directly to the entity.
			addr, ok := fl.captured(e, c)
			if !ok {
				return nil
			}
			fl.blk.Ptr.Store(addr, at)
		default:
			// By-value capture: copy according to type.
			src, ok := fl.captured(e, c)
			if !ok {
				return nil
			}
			if cr := classOf(field.Type); cr != nil {
				if !fl.copyObject(at, src, cr, nil) {
					return nil
				}
				continue
			}
			fl.store(at, fl.load(src, field.Type), field.Type)
		}
	}
	return obj
}

// captured is the address of the entity a capture names, where the
// lambda is written: a local of this function, or -- inside another
// lambda's body -- that closure's own capture of it, a member reached
// through this.
func (fl *fn) captured(e *ast.LambdaExpr, c sema.Capture) (ir.Ptr, bool) {
	if slot, ok := fl.slotOf(c.Sym); ok {
		addr, _, _ := fl.throughRef(slot, c.Sym.SymType)
		return addr, true
	}
	if fl.hasThis && fl.sym.InClass != nil {
		if off, t, ok := fl.u.fieldOffset(fl.sym.InClass, c.Name); ok {
			base := fl.this
			if off != 0 {
				base = fl.blk.Ptr.Add(base, fl.blk.I64.Const(off))
			}
			addr, _, _ := fl.throughRef(base, t)
			return addr, true
		}
	}
	fl.u.errorf(e.Pos(), "lowering has no storage for captured %q", c.Name)
	return ir.Ptr{}, false
}

// lambdaInvoker returns the static function pointer conversion invoker for a captureless closure.
func (u *unit) lambdaInvoker(rec *types.Record) ir.Callee {
	if !isClosure(rec) {
		return nil
	}
	if f, done := u.invokers[rec]; done {
		return f
	}
	var call *sema.FuncSymbol
	for _, m := range rec.Methods {
		if m.Name == "operator()" {
			call = u.declaredFor(&sema.FuncSymbol{SymName: m.Name, FuncType: m.Func, InClass: rec})
		}
	}
	if call == nil {
		u.errorf(ast.NoTok, "lowering has no operator() for %s", rec.Name)
		return nil
	}
	invoker := &sema.FuncSymbol{
		SymName:  "<lambda_invoker_cdecl>",
		SymScope: u.res.GlobalScope,
		FuncType: &types.Func{Ret: call.FuncType.Ret, Params: call.FuncType.Params, Variadic: call.FuncType.Variadic},
		InClass:  rec,
		Static:   true,
		Access:   types.AccessPublic,
	}
	for i, p := range call.FuncType.Params {
		invoker.Params = append(invoker.Params, &sema.VarSymbol{SymName: p.Name, SymType: p.Type, IsParam: true})
		if p.Name == "" {
			invoker.Params[i].SymName = "unnamed"
		}
	}
	f := u.declareFunc(invoker)
	u.funcs[invoker] = f
	u.invokers[rec] = f

	fl := &fn{u: u, sym: invoker, f: f, slots: map[sema.Symbol]ir.Ptr{}}
	fl.entry = f.Entry()
	body := f.Block("body")
	fl.blk = body
	if p, has := u.srets[invoker]; has {
		fl.sret, fl.hasSRet = p, true
	}
	args := u.params[invoker]
	var into ir.Ptr
	if fl.hasSRet {
		into = fl.sret
	}
	v := fl.invoke(call, fl.blk.Ptr.Const(), args, into, ast.NoTok)
	switch {
	case fl.hasSRet:
		fl.blk.Return()
	case v == nil || types.IsVoid(types.Unqualify(call.FuncType.Ret)):
		fl.blk.Return()
	default:
		fl.blk.Return(v)
	}
	fl.entry.Br(body.To())
	return f
}
