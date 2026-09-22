package sema

// Generic lambdas ([expr.prim.lambda.closure]/7): a lambda with an auto
// parameter, or a template parameter list, has an operator() that is a
// member function template. A lambda has no function declaration for
// instantiateWithArgs to read again, so its specializations are made
// here, from the lambda: each call deduces the template arguments from
// its arguments, and the first call with a given list checks a copy of
// the body with the parameters' types put in. The copy is checked in a
// scope inside the lambda's own, so captures -- explicit and implicit --
// are what the generic body found.

import (
	"fmt"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/types"
)

// genericLambda is the lambda whose generic operator() fn is, or nil.
func (a *Analyzer) genericLambda(fn *FuncSymbol) *LambdaInfo {
	if a.generics == nil {
		return nil
	}
	return a.generics[fn]
}

// instantiateLambdaCall is the specialization of a generic lambda's
// operator() for a call's arguments.
func (a *Analyzer) instantiateLambdaCall(li *LambdaInfo, args []Argument, at ast.Tok) *FuncSymbol {
	gen := li.Call
	b := Binding{}
	for i, p := range gen.FuncType.Params {
		if i >= len(args) {
			break
		}
		if !deduceArg(p.Type, args[i], b) {
			a.errorAt(at, fmt.Sprintf("no deduction for the generic lambda's parameter %d from %s", i+1, args[i].Type))
			return nil
		}
	}
	var targs []types.TemplateArg
	for _, name := range li.TemplateNames {
		t, ok := b[name]
		if !ok || t == nil {
			a.errorAt(at, fmt.Sprintf("the generic lambda's template parameter %s is not deduced", name))
			return nil
		}
		targs = append(targs, types.TemplateArg{IsType: true, Type: t})
	}
	key := argsKey(targs)
	if inst, done := li.Instances[key]; done {
		return inst
	}
	call := &types.Func{Ret: types.Typ(types.AutoKind), Variadic: gen.FuncType.Variadic, Quals: gen.FuncType.Quals, Noexcept: gen.FuncType.Noexcept}
	for i, p := range gen.FuncType.Params {
		pt := substitute(p.Type, b)
		if isDependent(pt) {
			a.errorAt(at, fmt.Sprintf("the generic lambda's parameter %d does not substitute: %s", i+1, pt))
			return nil
		}
		p.Type = pt
		call.Params = append(call.Params, p)
	}
	if li.Expr.Trailing != nil {
		call.Ret = substitute(gen.FuncType.Ret, b)
	}
	inst := &FuncSymbol{SymName: gen.SymName, FuncType: call, InClass: gen.InClass, SymPos: gen.SymPos,
		SymScope: gen.SymScope, Inline: true, Access: gen.Access, Space: gen.Space,
		TemplateArgs: targs, TemplateOf: gen}
	if li.Instances == nil {
		li.Instances = map[string]*FuncSymbol{}
	}
	li.Instances[key] = inst

	if li.Expr.Body != nil {
		body := ast.Clone(li.Expr.Body)
		scope := NewScope(li.Inner, FunctionScope, inst)
		oldScope, oldFunc, oldParams, oldLambdas := a.curScope, a.curFunc, a.curTemplateParams, a.lambdas
		a.curScope, a.curFunc, a.curTemplateParams = scope, inst, nil
		a.lambdas = append(append([]*LambdaInfo(nil), oldLambdas...), li)
		inst.Params = a.declareParams(call, scope)
		a.CheckStmt(body)
		a.curScope, a.curFunc, a.curTemplateParams, a.lambdas = oldScope, oldFunc, oldParams, oldLambdas
		if call.Ret != nil && call.Ret.Kind() == types.AutoKind {
			call.Ret = types.Typ(types.Void)
		}
		inst.Body = body
		a.noteDeclared(inst)
	}
	return inst
}

// callArgsDependent reports whether any argument's type is still
// dependent, as in a template's body, where a generic lambda is not
// instantiated yet.
func callArgsDependent(args []Argument) bool {
	for _, arg := range args {
		if isDependent(arg.Type) {
			return true
		}
	}
	return false
}
