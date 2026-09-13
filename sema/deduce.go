package sema

import (
	"github.com/vertex-language/vcx/types"
)

// Template argument deduction and substitution.

// Binding maps a template parameter's name to what it was deduced as.
type Binding map[string]types.Type

// valueBound records a deduced non-type template parameter value.
type valueBound struct {
	types.Type
	Val int64
}

// isDependent reports whether t mentions a template parameter anywhere.
func isDependent(t types.Type) bool {
	switch x := t.(type) {
	case nil:
		return false
	case *types.TemplateParam, *types.DependentType:
		return true
	case *types.Transform:
		return true
	case *types.Qualified:
		return isDependent(x.T)
	case *types.Pointer:
		return isDependent(x.Elem)
	case *types.LValueReference:
		return isDependent(x.Elem)
	case *types.RValueReference:
		return isDependent(x.Elem)
	case *types.Array:
		return x.DepLen != "" || isDependent(x.Elem)
	case *types.Func:
		if isDependent(x.Ret) {
			return true
		}
		for _, p := range x.Params {
			if isDependent(p.Type) {
				return true
			}
		}
	case *types.TemplateSpecialization:
		for _, a := range x.Args {
			if a.IsType && isDependent(a.Type) {
				return true
			}
		}
	}
	return false
}

// deduce matches parameter against argument type, recording deduced template parameters.
func deduce(param, arg types.Type, out Binding) bool {
	if param == nil || arg == nil {
		return false
	}

	// A parameter that mentions nothing to deduce imposes no constraint.
	// Whether the argument converts to it is overload resolution's question,
	// asked later against the substituted signature.
	if !isDependent(param) {
		return true
	}

	switch p := param.(type) {
	case *types.TemplateParam:
		if !p.IsType || p.Name == "" {
			return false
		}
		a := types.Unqualify(types.RemoveReference(arg))
		if prev, seen := out[p.Name]; seen {
			// Multiple deductions for the same parameter must agree.
			return prev != nil && prev.Equal(a)
		}
		out[p.Name] = a
		return true

	case *types.Qualified:
		// `const T` against `const int`: the qualification on the parameter
		// is matched by the argument's and removed from both.
		return deduce(p.T, types.Unqualify(arg), out)

	case *types.LValueReference:
		// Match referenced type against argument, preserving qualifiers.
		return deduceReferenced(p.Elem, types.RemoveReference(arg), out)

	case *types.RValueReference:
		return deduceReferenced(p.Elem, types.RemoveReference(arg), out)

	case *types.Pointer:
		bare := types.Unqualify(types.RemoveReference(arg))
		if a, ok := bare.(*types.Pointer); ok {
			return deduce(p.Elem, a.Elem, out)
		}
		// Array argument decays to element pointer.
		if arr, ok := bare.(*types.Array); ok {
			return deduce(p.Elem, arr.Elem, out)
		}
		return false

	case *types.Array:
		a, ok := types.Unqualify(types.RemoveReference(arg)).(*types.Array)
		if !ok {
			return false
		}
		if p.DepLen != "" {
			// `T[N]` binds N to the argument's bound, which an array of
			// unknown bound does not have.
			if a.Incomplete || a.DepLen != "" {
				return false
			}
			if prev, seen := out[p.DepLen]; seen {
				vb, isVal := prev.(*valueBound)
				if !isVal || vb.Val != a.Len {
					return false
				}
			} else {
				out[p.DepLen] = &valueBound{Type: a.Elem, Val: a.Len}
			}
		} else if a.Incomplete != p.Incomplete || !p.Incomplete && a.Len != p.Len {
			return false
		}
		return deduce(p.Elem, a.Elem, out)

	case *types.Func:
		a, ok := types.Unqualify(types.RemoveReference(arg)).(*types.Func)
		if !ok || len(a.Params) != len(p.Params) {
			return false
		}
		if !deduce(p.Ret, a.Ret, out) {
			return false
		}
		for i := range p.Params {
			if !deduce(p.Params[i].Type, a.Params[i].Type, out) {
				return false
			}
		}
		return true

	case *types.TemplateSpecialization:
		// `Box<T>` against `Box<int>`: the names have to agree and the
		// arguments are matched pairwise. The argument is the
		// specialization's class, instantiated, which carries the
		// arguments it was made for; or the same dependent spelling.
		if _, isClass := p.Type.(*types.Record); !isClass {
			// Alias template specializations are non-deduced contexts.
			return true
		}
		var name string
		var args []types.TemplateArg
		switch a := types.Unqualify(types.RemoveReference(arg)).(type) {
		case *types.TemplateSpecialization:
			name, args = a.Name, a.Args
		case *types.Record:
			if a.TemplateArgs == nil {
				return false
			}
			name, args = a.Name, a.TemplateArgs
		default:
			return false
		}
		if name != p.Name || len(args) != len(p.Args) {
			return false
		}
		for i := range p.Args {
			if !p.Args[i].IsType || !args[i].IsType {
				continue
			}
			if !deduce(p.Args[i].Type, args[i].Type, out) {
				return false
			}
		}
		return true

	case *types.DependentType:
		// Nested dependent type names are non-deduced contexts.
		return true
	}

	return false
}

// substitute replaces template parameters in t with their bound types.
func substitute(t types.Type, b Binding) types.Type {
	if t == nil || len(b) == 0 || !isDependent(t) {
		return t
	}

	switch x := t.(type) {
	case *types.Transform:
		arg := substitute(x.Arg, b)
		if r, ok := types.ApplyTransform(x.Op, arg, isDependent(arg)); ok && r != nil {
			return r
		}
		return t

	case *types.TemplateParam:
		if sub, ok := b[x.Name]; ok && sub != nil {
			if _, isPack := sub.(*types.Pack); isPack && !x.IsPack {
				return t
			}
			return sub
		}
		return t

	case *types.Qualified:
		return types.Qualify(substitute(x.T, b), x.Q)

	case *types.Pointer:
		return &types.Pointer{Elem: substitute(x.Elem, b)}

	case *types.LValueReference:
		return types.AddLValueReference(substitute(x.Elem, b))

	case *types.RValueReference:
		return types.AddRValueReference(substitute(x.Elem, b))

	case *types.Array:
		if x.DepLen != "" {
			if vb, isVal := b[x.DepLen].(*valueBound); isVal {
				return &types.Array{Elem: substitute(x.Elem, b), Len: vb.Val}
			}
		}
		return &types.Array{Elem: substitute(x.Elem, b), Len: x.Len, Incomplete: x.Incomplete, DepLen: x.DepLen}

	case *types.Func:
		out := &types.Func{
			Ret:      substitute(x.Ret, b),
			Variadic: x.Variadic,
			Quals:    x.Quals,
			RefQual:  x.RefQual,
			Noexcept: x.Noexcept,
		}
		for _, p := range x.Params {
			if p.Pack {
				// The pattern, once per element of the bound pack.
				if pack, isPack := b[packParamName(p.Type)].(*types.Pack); isPack {
					for _, elem := range pack.Elems {
						one := Binding{packParamName(p.Type): elem.Type}
						out.Params = append(out.Params, types.Param{Name: p.Name, Type: substitute(p.Type, one), PackOf: p.Name})
					}
					continue
				}
			}
			out.Params = append(out.Params, types.Param{
				Name:       p.Name,
				Type:       substitute(p.Type, b),
				HasDefault: p.HasDefault,
				Pack:       p.Pack,
			})
		}
		return out

	case *types.TemplateSpecialization:
		out := &types.TemplateSpecialization{Name: x.Name, Type: x.Type}
		for _, a := range x.Args {
			if a.IsType {
				out.Args = append(out.Args, types.TemplateArg{IsType: true, Type: substitute(a.Type, b)})
				continue
			}
			out.Args = append(out.Args, a)
		}
		return out
	}

	return t
}

// specialize deduces fn's template parameters from call arguments and substitutes them.
func specialize(fn *types.Func, explicit []types.Type, args []Argument) (*types.Func, bool) {
	sig, _, ok := specializeWith(fn, explicit, args)
	return sig, ok
}

// specializeWith performs specialize returning the deduced parameter bindings.
func specializeWith(fn *types.Func, explicit []types.Type, args []Argument) (*types.Func, Binding, bool) {
	return specializeNamed(fn, nil, explicit, args)
}

// specializeNamed performs template argument deduction and substitution with explicit parameters.
func specializeNamed(fn *types.Func, paramNames []string, explicit []types.Type, args []Argument) (*types.Func, Binding, bool) {
	if fn == nil {
		return fn, nil, true
	}

	// Explicit arguments bind their parameters before deduction starts.
	b := Binding{}
	if paramNames == nil {
		paramNames = templateParamNames(fn)
	}
	for i, name := range paramNames {
		if i < len(explicit) && explicit[i] != nil {
			b[name] = explicit[i]
		}
	}
	if !isDependent(fn) {
		return fn, b, true
	}

	n := len(fn.Params)
	if n > 0 && fn.Params[n-1].Pack {
		// Pack parameter deduces against remaining arguments.
		n--
		if len(args) < n {
			return fn, nil, false
		}
		pat := fn.Params[n].Type
		var elems []types.TemplateArg
		for _, arg := range args[n:] {
			each := Binding{}
			for k, v := range b {
				each[k] = v
			}
			if !deduceArg(pat, arg, each) {
				return fn, nil, false
			}
			name := packParamName(pat)
			if name == "" {
				return fn, nil, false
			}
			elems = append(elems, types.TemplateArg{IsType: true, Type: each[name]})
		}
		if name := packParamName(pat); name != "" {
			b[name] = &types.Pack{Elems: elems}
		}
	}
	if len(args) < n {
		n = len(args)
	}
	for i := 0; i < n; i++ {
		if !deduceArg(fn.Params[i].Type, args[i], b) {
			return fn, nil, false
		}
	}

	out, ok := substitute(fn, b).(*types.Func)
	if !ok {
		return fn, nil, false
	}
	return out, b, true
}

// packParamName is the name of the parameter pack a pattern expands:
// the TemplateParam marked as a pack inside `Ts &&`.
func packParamName(t types.Type) string {
	switch x := t.(type) {
	case *types.TemplateParam:
		if x.IsPack {
			return x.Name
		}
	case *types.Qualified:
		return packParamName(x.T)
	case *types.Pointer:
		return packParamName(x.Elem)
	case *types.LValueReference:
		return packParamName(x.Elem)
	case *types.RValueReference:
		return packParamName(x.Elem)
	case *types.Array:
		return packParamName(x.Elem)
	}
	return ""
}

// templateParamNames returns the unique template parameter names in appearance order.
func templateParamNames(fn *types.Func) []string {
	var names []string
	seen := map[string]bool{}

	var walk func(types.Type)
	walk = func(t types.Type) {
		switch x := t.(type) {
		case *types.TemplateParam:
			if x.Name != "" && !seen[x.Name] {
				seen[x.Name] = true
				names = append(names, x.Name)
			}
		case *types.Qualified:
			walk(x.T)
		case *types.Pointer:
			walk(x.Elem)
		case *types.LValueReference:
			walk(x.Elem)
		case *types.RValueReference:
			walk(x.Elem)
		case *types.Array:
			walk(x.Elem)
		case *types.Func:
			walk(x.Ret)
			for _, p := range x.Params {
				walk(p.Type)
			}
		case *types.TemplateSpecialization:
			for _, a := range x.Args {
				if a.IsType {
					walk(a.Type)
				}
			}
		}
	}

	walk(fn.Ret)
	for _, p := range fn.Params {
		walk(p.Type)
	}
	return names
}

// dependentExpr returns ExprInfo for an expression whose type is dependent on template parameters.
func dependentExpr() ExprInfo {
	return ExprInfo{Type: &types.DependentType{}, ValCat: PrValue}
}

// isDependentExpr reports whether an expression's type is dependent.
func isDependentExpr(info ExprInfo) bool {
	return isDependentType(info.Type)
}

func isDependentType(t types.Type) bool {
	if t == nil {
		return false
	}
	if _, ok := types.Unqualify(t).(*types.DependentType); ok {
		return true
	}
	return isDependent(t)
}

// deduceArg performs argument deduction considering value categories and forwarding references.
func deduceArg(param types.Type, arg Argument, out Binding) bool {
	if rr, isRRef := param.(*types.RValueReference); isRRef && arg.IsLValue {
		if tp, isParam := rr.Elem.(*types.TemplateParam); isParam && tp.IsType && tp.Name != "" {
			bound := types.AddLValueReference(arg.Type)
			if prev, seen := out[tp.Name]; seen {
				return prev != nil && prev.Equal(bound)
			}
			out[tp.Name] = bound
			return true
		}
	}
	return deduce(param, arg.Type, out)
}

// matchType deduces template arguments against a partial specialization pattern.
func matchType(pat, arg types.Type, out Binding) bool {
	if pat == nil || arg == nil {
		return false
	}
	if !isDependent(pat) {
		return pat.Equal(arg)
	}
	switch p := pat.(type) {
	case *types.TemplateParam:
		if !p.IsType || p.Name == "" {
			return false
		}
		if prev, seen := out[p.Name]; seen {
			return prev != nil && prev.Equal(arg)
		}
		out[p.Name] = arg
		return true

	case *types.Qualified:
		q, isQual := arg.(*types.Qualified)
		if !isQual || q.Q&p.Q != p.Q {
			return false
		}
		rest := q.Q &^ p.Q
		var inner types.Type = q.T
		if rest != 0 {
			inner = types.Qualify(q.T, rest)
		}
		return matchType(p.T, inner, out)

	case *types.LValueReference:
		a, isRef := arg.(*types.LValueReference)
		return isRef && matchType(p.Elem, a.Elem, out)

	case *types.RValueReference:
		a, isRef := arg.(*types.RValueReference)
		return isRef && matchType(p.Elem, a.Elem, out)

	case *types.Pointer:
		a, isPtr := types.Unqualify(arg).(*types.Pointer)
		if !isPtr || types.IsConst(arg) || types.IsVolatile(arg) {
			return false
		}
		return matchType(p.Elem, a.Elem, out)

	case *types.MemberPointer:
		a, isMP := types.Unqualify(arg).(*types.MemberPointer)
		return isMP && matchType(p.Class, a.Class, out) && matchType(p.Elem, a.Elem, out)

	case *types.Array:
		a, isArr := types.Unqualify(arg).(*types.Array)
		if !isArr {
			return false
		}
		if p.DepLen != "" {
			if a.Incomplete || a.DepLen != "" {
				return false
			}
			if prev, seen := out[p.DepLen]; seen {
				vb, isVal := prev.(*valueBound)
				if !isVal || vb.Val != a.Len {
					return false
				}
			} else {
				out[p.DepLen] = &valueBound{Type: a.Elem, Val: a.Len}
			}
		} else if a.Incomplete != p.Incomplete || !p.Incomplete && a.Len != p.Len {
			return false
		}
		return matchType(p.Elem, a.Elem, out)

	case *types.Func:
		a, isFunc := types.Unqualify(arg).(*types.Func)
		if !isFunc || len(a.Params) != len(p.Params) || a.Variadic != p.Variadic {
			return false
		}
		if !matchType(p.Ret, a.Ret, out) {
			return false
		}
		for i := range p.Params {
			if !matchType(p.Params[i].Type, a.Params[i].Type, out) {
				return false
			}
		}
		return true

	case *types.TemplateSpecialization:
		if isAliasSpelling(p) {
			// An alias template specialization is a non-deduced context.
			return true
		}
		var name string
		var args []types.TemplateArg
		var primary *types.Record
		switch a := types.Unqualify(arg).(type) {
		case *types.TemplateSpecialization:
			name, args = a.Name, a.Args
			primary, _ = a.Type.(*types.Record)
		case *types.Record:
			if a.TemplateArgs == nil {
				return false
			}
			name, args = a.Name, a.TemplateArgs
			primary = a
		default:
			return false
		}
		args = flattenPacks(args)
		if tp, isParam := p.Type.(*types.TemplateParam); isParam {
			// `_Ty<_First, _Rest...>` with _Ty a template template
			// parameter: _Ty is bound to the argument's template, and
			// the arguments are matched pairwise as with any
			// specialization.
			if primary == nil {
				return false
			}
			ref := &types.TemplateRef{Name: name, Primary: primaryOf(primary)}
			if prev, seen := out[tp.Name]; seen {
				if !prev.Equal(ref) {
					return false
				}
			} else {
				out[tp.Name] = ref
			}
		} else if name != p.Name {
			return false
		}
		if n := len(p.Args); n > 0 && isPackParam(p.Args[n-1]) {
			// The pattern's own pack takes the rest of the arguments.
			if len(args) < n-1 {
				return false
			}
			tp := p.Args[n-1].Type.(*types.TemplateParam)
			rest := &types.Pack{Elems: append([]types.TemplateArg(nil), args[n-1:]...)}
			if prev, seen := out[tp.Name]; seen {
				if !prev.Equal(rest) {
					return false
				}
			} else {
				out[tp.Name] = rest
			}
			args = args[:n-1]
			p = &types.TemplateSpecialization{Name: p.Name, Args: p.Args[:n-1], Type: p.Type}
		}
		if len(args) != len(p.Args) {
			return false
		}
		for i := range p.Args {
			if !p.Args[i].IsType || !args[i].IsType {
				if p.Args[i].IsType == args[i].IsType && p.Args[i].Val != args[i].Val {
					return false
				}
				continue
			}
			if !matchType(p.Args[i].Type, args[i].Type, out) {
				return false
			}
		}
		return true

	case *types.DependentType:
		return true
	}
	return false
}

// deduceReferenced is deduce for what a reference parameter refers to:
// deduceReferenced performs deduction for a reference parameter target.
func deduceReferenced(param, arg types.Type, out Binding) bool {
	if tp, isParam := param.(*types.TemplateParam); isParam && tp.IsType && tp.Name != "" {
		if prev, seen := out[tp.Name]; seen {
			return prev != nil && prev.Equal(arg)
		}
		out[tp.Name] = arg
		return true
	}
	return deduce(param, arg, out)
}

// primaryOf returns the primary class template record for a specialization.
func primaryOf(rec *types.Record) *types.Record {
	if p, known := primaryRecords[rec]; known {
		return p
	}
	return rec
}

// primaryRecords maps specialization records to their primary template record.
var primaryRecords = map[*types.Record]*types.Record{}
