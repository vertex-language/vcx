package sema

import (
	"github.com/vertex-language/vcx/types"
)

// Type traits evaluated via overload resolution and semantic analysis
// (constructibility, assignability, convertibility, etc.).

// resolveTrait answers a trait by name for the evaluator.
func (a *Analyzer) resolveTrait(name string, args []types.Type) (answer bool, known bool, err error) {
	switch name {
	case "__is_constructible", "__is_nothrow_constructible", "__is_trivially_constructible":
		if len(args) == 0 {
			return false, true, nil
		}
		chosen, ok := a.constructible(args[0], args[1:])
		switch name {
		case "__is_nothrow_constructible":
			return ok && (chosen == nil || chosen.Defaulted || chosen.FuncType.Noexcept), true, nil
		case "__is_trivially_constructible":
			return ok && (chosen == nil || chosen.Defaulted && a.triviallyCopyable(args[0])), true, nil
		}
		return ok, true, nil

	case "__is_assignable", "__is_nothrow_assignable", "__is_trivially_assignable", "__is_assignable_no_precondition_check":
		if len(args) != 2 {
			return false, true, nil
		}
		chosen, ok := a.assignable(args[0], args[1])
		switch name {
		case "__is_nothrow_assignable":
			return ok && (chosen == nil || chosen.Defaulted || chosen.FuncType.Noexcept), true, nil
		case "__is_trivially_assignable":
			return ok && (chosen == nil || chosen.Defaulted && a.triviallyCopyable(types.RemoveReference(args[0]))), true, nil
		}
		return ok, true, nil

	case "__is_destructible", "__is_nothrow_destructible", "__is_trivially_destructible":
		if len(args) != 1 {
			return false, true, nil
		}
		t := types.Unqualify(args[0])
		if types.IsVoid(t) || types.IsFunc(t) {
			return false, true, nil
		}
		if arr, isArr := t.(*types.Array); isArr {
			if arr.Incomplete {
				return false, true, nil
			}
			return a.resolveTrait(name, []types.Type{arr.Elem})
		}
		if types.IsReference(args[0]) {
			return true, true, nil
		}
		rec := types.AsRecord(t)
		if rec == nil {
			return true, true, nil
		}
		dtor := a.destructorOf(rec)
		if dtor != nil && dtor.Deleted {
			return false, true, nil
		}
		if name == "__is_trivially_destructible" {
			return (dtor == nil || dtor.Defaulted) && a.triviallyDestructible(rec), true, nil
		}
		return true, true, nil // a destructor is noexcept unless declared otherwise

	case "__is_convertible_to", "__is_convertible", "__is_nothrow_convertible":
		if len(args) != 2 {
			return false, true, nil
		}
		from, to := args[0], args[1]
		if types.IsVoid(types.Unqualify(from)) || types.IsVoid(types.Unqualify(to)) {
			return types.IsVoid(types.Unqualify(from)) && types.IsVoid(types.Unqualify(to)), true, nil
		}
		return a.convertible(from, to), true, nil

	case "__is_empty":
		return oneClass(args, func(rec *types.Record) bool { return types.IsEmptyRecord(rec) })
	case "__is_polymorphic":
		return oneClass(args, func(rec *types.Record) bool { return types.IsPolymorphic(rec) })
	case "__is_abstract":
		return oneClass(args, func(rec *types.Record) bool { return types.IsAbstract(rec) })
	case "__is_final":
		return oneClass(args, func(rec *types.Record) bool { return rec.Final })
	case "__has_virtual_destructor":
		return oneClass(args, func(rec *types.Record) bool {
			for _, m := range rec.Methods {
				if m.Name == "~"+rec.Name && m.Virtual {
					return true
				}
			}
			return false
		})
	case "__is_trivially_copyable":
		if len(args) != 1 {
			return false, true, nil
		}
		return a.triviallyCopyable(args[0]), true, nil
	case "__is_trivial":
		if len(args) != 1 {
			return false, true, nil
		}
		t := types.Unqualify(args[0])
		rec := types.AsRecord(t)
		if rec == nil {
			return a.triviallyCopyable(args[0]) && !types.IsReference(args[0]), true, nil
		}
		_, defaultOK := a.constructible(t, nil)
		return a.triviallyCopyable(t) && defaultOK && a.trivialDefault(rec), true, nil
	case "__is_standard_layout":
		if len(args) != 1 {
			return false, true, nil
		}
		return a.standardLayout(args[0]), true, nil
	case "__is_pod":
		if len(args) != 1 {
			return false, true, nil
		}
		trivial, _, _ := a.resolveTrait("__is_trivial", args)
		return trivial && a.standardLayout(args[0]), true, nil
	case "__is_aggregate":
		if len(args) != 1 {
			return false, true, nil
		}
		t := types.Unqualify(args[0])
		if _, isArr := t.(*types.Array); isArr {
			return true, true, nil
		}
		rec := types.AsRecord(t)
		if rec == nil {
			return false, true, nil
		}
		return !hasUserConstructor(rec) && !types.IsPolymorphic(rec) && a.allPublic(rec), true, nil
	case "__is_literal_type":
		if len(args) != 1 {
			return false, true, nil
		}
		t := types.Unqualify(args[0])
		if types.IsVoid(t) || types.IsReference(args[0]) || types.IsScalar(t) {
			return true, true, nil
		}
		if arr, isArr := t.(*types.Array); isArr {
			return a.resolveTrait(name, []types.Type{arr.Elem})
		}
		rec := types.AsRecord(t)
		if rec == nil {
			return false, true, nil
		}
		destructible, _, _ := a.resolveTrait("__is_trivially_destructible", args)
		return destructible && (!hasUserConstructor(rec) || a.hasConstexprConstructor(rec)), true, nil
	case "__has_unique_object_representations":
		if len(args) != 1 {
			return false, true, nil
		}
		return a.uniqueRepresentations(args[0]), true, nil
	}
	return false, false, nil
}

// oneClass answers a trait of a class type, false for anything else.
func oneClass(args []types.Type, f func(*types.Record) bool) (bool, bool, error) {
	if len(args) != 1 {
		return false, true, nil
	}
	rec := types.AsRecord(types.Unqualify(args[0]))
	if rec == nil || !rec.Complete {
		return false, true, nil
	}
	return f(rec), true, nil
}

// asArguments turns the trait's argument types into call arguments: a
// `T &` is an lvalue of T, anything else a prvalue -- an rvalue, which is
// what declval<T>() gives for T and T &&.
func asArguments(ts []types.Type) []Argument {
	out := make([]Argument, len(ts))
	for i, t := range ts {
		out[i] = Argument{Type: types.RemoveReference(t), IsLValue: types.IsLValueReference(t)}
	}
	return out
}

// constructible checks whether `T t(declval<Args>()...)` is well-formed.
func (a *Analyzer) constructible(t types.Type, argTypes []types.Type) (*FuncSymbol, bool) {
	if types.IsReference(t) {
		// Reference initialization requires exactly one argument.
		if len(argTypes) != 1 {
			return nil, false
		}
		arg := asArguments(argTypes)[0]
		return nil, ClassifyConversion(arg.Type, t, arg.IsLValue).Valid
	}
	bare := types.Unqualify(t)
	if types.IsVoid(bare) || types.IsFunc(bare) {
		return nil, false
	}
	if arr, isArr := bare.(*types.Array); isArr {
		// An array is constructible only by value-initialization of
		// its elements, and only when its bound is known.
		if arr.Incomplete || len(argTypes) != 0 {
			return nil, false
		}
		return a.constructible(arr.Elem, nil)
	}
	rec := types.AsRecord(bare)
	if rec == nil {
		// A scalar: value-initialized from nothing, or converted from one.
		switch len(argTypes) {
		case 0:
			return nil, true
		case 1:
			arg := asArguments(argTypes)[0]
			return nil, ClassifyConversion(arg.Type, bare, arg.IsLValue).Valid
		}
		return nil, false
	}
	if !rec.Complete {
		return nil, false
	}
	if types.IsAbstract(rec) {
		return nil, false
	}
	chosen, err := a.chooseConstructor(rec, asArguments(argTypes))
	if err != nil || chosen.Deleted {
		return nil, false
	}
	return chosen, true
}

// assignable is `declval<T>() = declval<U>()`.
func (a *Analyzer) assignable(t, u types.Type) (*FuncSymbol, bool) {
	// The left operand: an lvalue when T is an lvalue reference, an
	// rvalue otherwise -- and an rvalue of scalar type is not assignable
	// to, where a class rvalue is (through operator=).
	lhs := types.RemoveReference(t)
	rec := types.AsRecord(types.Unqualify(lhs))
	if rec == nil {
		if !types.IsLValueReference(t) || types.IsConst(lhs) {
			return nil, false
		}
		bare := types.Unqualify(lhs)
		if types.IsVoid(bare) || types.IsFunc(bare) {
			return nil, false
		}
		if _, isArr := bare.(*types.Array); isArr {
			return nil, false
		}
		arg := asArguments([]types.Type{u})[0]
		return nil, ClassifyConversion(arg.Type, bare, arg.IsLValue).Valid
	}
	if !rec.Complete {
		return nil, false
	}
	if types.IsConst(lhs) {
		return nil, false
	}
	ops := a.memberFuncs(rec, "operator=")
	if len(ops) == 0 {
		return nil, false
	}
	chosen, err := a.resolveAmong(ops, nil, asArguments([]types.Type{u}), 0)
	if err != nil || chosen.Deleted {
		return nil, false
	}
	return chosen, true
}

// convertible tests whether an implicit conversion from From to To is valid.
func (a *Analyzer) convertible(from, to types.Type) bool {
	arg := asArguments([]types.Type{from})[0]
	if ClassifyConversion(arg.Type, to, arg.IsLValue).Valid {
		return true
	}
	// A class To with a converting constructor from From, which the
	// conversion table does not see.
	if rec := types.AsRecord(types.Unqualify(types.RemoveReference(to))); rec != nil && !types.IsReference(to) {
		chosen, ok := a.constructible(to, []types.Type{from})
		return ok && (chosen == nil || !chosen.Explicit)
	}
	return false
}

// destructorOf is the class's destructor as declared, or nil.
func (a *Analyzer) destructorOf(rec *types.Record) *FuncSymbol {
	for _, fn := range a.memberFuncs(rec, "~"+rec.Name) {
		return fn
	}
	return nil
}

// triviallyCopyable checks for trivial copy/move operations, trivial destructor,
// and no virtual bases or methods.
func (a *Analyzer) triviallyCopyable(t types.Type) bool {
	if types.IsReference(t) {
		return false
	}
	bare := types.Unqualify(t)
	if arr, isArr := bare.(*types.Array); isArr {
		return a.triviallyCopyable(arr.Elem)
	}
	rec := types.AsRecord(bare)
	if rec == nil {
		return types.IsScalar(bare)
	}
	if !rec.Complete || types.IsPolymorphic(rec) {
		return false
	}
	for _, b := range rec.Bases {
		if b.Virtual || !a.triviallyCopyable(b.Type) {
			return false
		}
	}
	for _, m := range rec.Methods {
		special := m.Name == "~"+rec.Name || m.Name == "operator="
		if m.Name == rec.Name && len(m.Func.Params) == 1 {
			if p := types.RemoveReference(m.Func.Params[0].Type); types.AsRecord(types.Unqualify(p)) == rec {
				special = true
			}
		}
		if special && !m.Defaulted && !m.Deleted {
			return false
		}
	}
	for _, f := range rec.Fields {
		if !a.triviallyCopyable(f.Type) {
			return false
		}
	}
	return true
}

// triviallyDestructible is whether nothing runs when an object of the
// class dies: no user-provided destructor anywhere in it.
func (a *Analyzer) triviallyDestructible(rec *types.Record) bool {
	for _, m := range rec.Methods {
		if m.Name == "~"+rec.Name && !m.Defaulted {
			return false
		}
	}
	for _, b := range rec.Bases {
		if br := types.AsRecord(types.Unqualify(b.Type)); br != nil && !a.triviallyDestructible(br) {
			return false
		}
	}
	for _, f := range rec.Fields {
		if fr := types.AsRecord(types.Unqualify(f.Type)); fr != nil && !a.triviallyDestructible(fr) {
			return false
		}
	}
	return true
}

// trivialDefault is whether the class's default constructor is the
// implicit one and does nothing: no user-provided default constructor,
// no member initializers, and the same of every base and member.
func (a *Analyzer) trivialDefault(rec *types.Record) bool {
	for _, m := range rec.Methods {
		if m.Name == rec.Name && len(m.Func.Params) == 0 && !m.Defaulted {
			return false
		}
	}
	for _, f := range rec.Fields {
		if f.HasInit {
			return false
		}
		if fr := types.AsRecord(types.Unqualify(f.Type)); fr != nil && !a.trivialDefault(fr) {
			return false
		}
	}
	for _, b := range rec.Bases {
		if br := types.AsRecord(types.Unqualify(b.Type)); br != nil && !a.trivialDefault(br) {
			return false
		}
	}
	return true
}

// standardLayout checks whether a type satisfies standard-layout requirements.
func (a *Analyzer) standardLayout(t types.Type) bool {
	if types.IsReference(t) {
		return false
	}
	bare := types.Unqualify(t)
	if arr, isArr := bare.(*types.Array); isArr {
		return a.standardLayout(arr.Elem)
	}
	rec := types.AsRecord(bare)
	if rec == nil {
		return types.IsScalar(bare)
	}
	if !rec.Complete || types.IsPolymorphic(rec) {
		return false
	}
	for _, b := range rec.Bases {
		if b.Virtual || !a.standardLayout(b.Type) {
			return false
		}
	}
	var access types.Access
	for i, f := range rec.Fields {
		if i == 0 {
			access = f.Access
		} else if f.Access != access {
			return false
		}
		if !a.standardLayout(f.Type) {
			return false
		}
	}
	return true
}

// allPublic reports whether all non-static data members and bases are public.
func (a *Analyzer) allPublic(rec *types.Record) bool {
	for _, f := range rec.Fields {
		if f.Access != types.AccessPublic {
			return false
		}
	}
	for _, b := range rec.Bases {
		if b.Access != types.AccessPublic {
			return false
		}
	}
	return true
}

// hasConstexprConstructor is whether any constructor of the class is
// constexpr, which is what makes a class with constructors a literal type.
func (a *Analyzer) hasConstexprConstructor(rec *types.Record) bool {
	for _, fn := range a.memberFuncs(rec, rec.Name) {
		if fn.Constexpr || fn.Defaulted {
			return true
		}
	}
	return false
}

// uniqueRepresentations reports whether all bits in the object contribute to its value.
func (a *Analyzer) uniqueRepresentations(t types.Type) bool {
	bare := types.Unqualify(t)
	if arr, isArr := bare.(*types.Array); isArr {
		return !arr.Incomplete && a.uniqueRepresentations(arr.Elem)
	}
	if types.IsInteger(bare) || types.IsEnum(bare) || types.IsPointer(bare) {
		return true
	}
	rec := types.AsRecord(bare)
	if rec == nil || !rec.Complete || !a.triviallyCopyable(bare) {
		return false
	}
	var sum int64
	for _, f := range rec.Fields {
		if f.BitField || !a.uniqueRepresentations(f.Type) {
			return false
		}
		sz, ok := a.model.Sizeof(f.Type)
		if !ok {
			return false
		}
		sum += sz
	}
	for _, b := range rec.Bases {
		if !a.uniqueRepresentations(b.Type) {
			return false
		}
		sz, ok := a.model.Sizeof(b.Type)
		if !ok {
			return false
		}
		sum += sz
	}
	size, ok := a.model.Sizeof(bare)
	return ok && size == sum
}
