package sema

import (
	"fmt"
	"os"
	"strings"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/constexpr"
	"github.com/vertex-language/vcx/literal"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// ValueCategory defines the C++ value category of an expression.
type ValueCategory uint8

const (
	PrValue ValueCategory = iota
	LValue
	XValue
)

func (v ValueCategory) String() string {
	switch v {
	case PrValue:
		return "prvalue"
	case LValue:
		return "lvalue"
	case XValue:
		return "xvalue"
	}
	return "prvalue"
}

// ExprInfo records the type and value category of an evaluated expression.
type ExprInfo struct {
	Type     types.Type
	ValCat   ValueCategory
	ConstVal int64
	IsConst  bool
}

// CheckExpr type checks an expression and records its type and resolution info.
func (a *Analyzer) CheckExpr(expr ast.Expr) ExprInfo {
	if expr == nil {
		return ExprInfo{Type: types.Typ(types.Void), ValCat: PrValue}
	}
	if info, done := a.prechecked[expr]; done {
		// Prechecked pack expansion element.
		return info
	}
	info := a.checkExpr(expr)
	if a.info != nil && info.Type != nil {
		a.info.Types[expr] = info.Type
	}
	if info.IsConst {
		a.noteConst(expr, info.ConstVal)
	}
	return info
}

func (a *Analyzer) checkExpr(expr ast.Expr) ExprInfo {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return a.checkBasicLit(e)

	case *ast.StringLit:
		// Lvalue array of const char, null terminated.
		s, err := literal.Decode(a.unit, e)
		if err != nil {
			a.errorAt(e.Pos(), err.Error())
			return ExprInfo{Type: &types.Array{Elem: types.Qualify(types.Typ(types.Char), types.QConst), Incomplete: true}, ValCat: LValue}
		}
		return ExprInfo{
			Type:   &types.Array{Elem: types.Qualify(types.Typ(s.ElemKind()), types.QConst), Len: int64(len(s.Units) + 1)},
			ValCat: LValue,
		}

	case *ast.ThisExpr:
		rec := a.curScope.InnermostRecord()
		if rec == nil {
			a.errorAt(e.Pos(), "'this' is only valid inside member functions")
			return ExprInfo{Type: types.Typ(types.Void), ValCat: PrValue}
		}
		ptr := &types.Pointer{Elem: rec}
		if a.curFunc != nil && a.curFunc.FuncType != nil && a.curFunc.FuncType.Quals&types.QConst != 0 {
			ptr = &types.Pointer{Elem: types.Qualify(rec, types.QConst)}
		}
		return ExprInfo{Type: ptr, ValCat: PrValue}

	case *ast.Ident:
		name := e.Text(a.unit)
		syms := LookupUnqualified(a.curScope, name)
		if len(syms) == 0 {
			a.errorAt(e.Lo, fmt.Sprintf("use of undeclared identifier %q", name))
			return ExprInfo{Type: types.Typ(types.Int), ValCat: LValue}
		}
		a.record(e, syms[0])
		if v, isVar := syms[0].(*VarSymbol); isVar {
			a.implicitCapture(e, v)
		}
		return a.foldConstant(e, a.symToExprInfo(syms[0]), syms[0])

	case *ast.QualifiedName:
		// Qualified template-id.
		if tn, isTemplate := e.Name.(*ast.TemplateName); isTemplate {
			return a.checkTemplateIdExpr(e, tn, ResolveQualifiedName(e, a.curScope, a.globalScope, a.unit))
		}
		syms := ResolveQualifiedName(e, a.curScope, a.globalScope, a.unit)
		if len(syms) == 0 {
			a.errorAt(e.Pos(), fmt.Sprintf("no symbol named %q", NameString(e, a.unit)))
			return ExprInfo{Type: types.Typ(types.Int), ValCat: LValue}
		}
		a.record(e, syms[0])
		return a.foldConstant(e, a.symToExprInfo(syms[0]), syms[0])

	case *ast.FoldExpr:
		// Fold expression evaluation.
		if a.dependentContext() {
			return dependentExpr()
		}
		v, err := a.expandFold(a.NewConstContext(), e)
		if err != nil {
			a.errorAt(e.Pos(), fmt.Sprintf("fold expression is not a constant expression: %v", err))
			return ExprInfo{Type: types.Typ(types.Int), ValCat: PrValue}
		}
		info := ExprInfo{Type: v.Type(), ValCat: PrValue}
		switch v := v.(type) {
		case constexpr.IntValue:
			a.noteConst(e, v.Int64())
			info.IsConst, info.ConstVal = true, v.Int64()
		case constexpr.BoolValue:
			n := int64(0)
			if v.Val {
				n = 1
			}
			a.noteConst(e, n)
			info.IsConst, info.ConstVal = true, n
		}
		return info

	case *ast.PackExpansion:
		// Unexpanded pack expansion in template.
		a.CheckExpr(e.X)
		return dependentExpr()

	case *ast.TemplateName:
		var syms []Symbol
		switch n := e.Name.(type) {
		case *ast.QualifiedName:
			syms = ResolveQualifiedName(n, a.curScope, a.globalScope, a.unit)
		default:
			syms = LookupUnqualified(a.curScope, NameString(e.Name, a.unit))
		}
		return a.checkTemplateIdExpr(e, e, syms)

	case *ast.ParenExpr:
		return a.CheckExpr(e.X)

	case *ast.UnaryExpr:
		return a.checkUnaryExpr(e)

	case *ast.BinaryExpr:
		return a.checkBinaryExpr(e)

	case *ast.AssignExpr:
		return a.checkAssignExpr(e)

	case *ast.IncDecExpr:
		sub := a.CheckExpr(e.X)
		if isDependentExpr(sub) {
			return dependentExpr()
		}
		// Postfix operator overload resolution.
		if rec := types.AsRecord(types.Unqualify(types.RemoveReference(sub.Type))); rec != nil {
			opName := "operator++"
			if e.Op == token.DEC {
				opName = "operator--"
			}
			operands := []ExprInfo{sub, {Type: types.Typ(types.Int), ValCat: PrValue}}
			if info, resolved := a.resolveOperator(e, opName, operands, e.Pos()); resolved {
				return info
			}
		}
		if sub.ValCat != LValue {
			a.errorAt(e.Pos(), "operand of ++/-- must be an lvalue")
		}
		if types.IsConst(sub.Type) {
			a.errorAt(e.Pos(), "cannot modify const object with ++/--")
		}
		if !incrementable(sub.Type) {
			a.errorAt(e.Pos(), fmt.Sprintf("no operator%s for an operand of type %q", e.Op, sub.Type))
		}
		return ExprInfo{Type: types.Unqualify(sub.Type), ValCat: PrValue}

	case *ast.CondExpr:
		cond := a.CheckExpr(e.Cond)
		if !isDependentExpr(cond) && !contextuallyConvertibleToBool(cond.Type) {
			a.errorAt(e.Cond.Pos(), "condition expression must be convertible to bool")
		}
		thenInfo := a.CheckExpr(e.Then)
		elseInfo := a.CheckExpr(e.Else)
		if isDependentExpr(cond) || isDependentExpr(thenInfo) || isDependentExpr(elseInfo) {
			return dependentExpr()
		}
		ct := conditionalType(thenInfo, elseInfo, e)
		// Conditional expression value category.
		cat := PrValue
		if thenInfo.ValCat == LValue && elseInfo.ValCat == LValue &&
			types.Unqualify(thenInfo.Type).Equal(types.Unqualify(elseInfo.Type)) {
			cat = LValue
		}
		return ExprInfo{Type: ct, ValCat: cat}

	case *ast.CallExpr:
		return a.checkCallExpr(e)

	case *ast.MemberExpr:
		return a.checkMemberExpr(e)

	case *ast.IndexExpr:
		return a.checkIndexExpr(e)

	case *ast.CastExpr:
		targetT := BuildDeclarator(e.Type.Decl, BuildDeclSpecs(e.Type.Specs, a.curScope, a.unit).Type, a.curScope, a.unit)
		a.CheckExpr(e.X)
		return castResult(targetT)

	case *ast.NamedCastExpr:
		// Named cast value category and result type.
		targetT := BuildDeclarator(e.Type.Decl, BuildDeclSpecs(e.Type.Specs, a.curScope, a.unit).Type, a.curScope, a.unit)
		a.CheckExpr(e.X)
		return castResult(targetT)

	case *ast.FunctionalCastExpr:
		targetT := BuildDeclarator(e.Type.Decl, BuildDeclSpecs(e.Type.Specs, a.curScope, a.unit).Type, a.curScope, a.unit)
		rec := types.AsRecord(types.Unqualify(targetT))
		if e.Args != nil && (rec == nil || !hasUserConstructor(rec)) {
			// Functional cast list-initialization.
			a.checkListInit(e.Args, targetT)
			return ExprInfo{Type: targetT, ValCat: PrValue}
		}
		var args []Argument
		items := e.ArgList
		if e.Args != nil {
			for _, item := range e.Args.Items {
				if x, isExpr := item.(ast.Expr); isExpr {
					items = append(items, x)
				}
			}
		}
		for _, arg := range items {
			info := a.CheckExpr(arg)
			args = append(args, Argument{Type: info.Type, IsLValue: info.ValCat == LValue})
		}
		if rec != nil && hasUserConstructor(rec) && !isDependentType(targetT) && !dependentArguments(args) {
			chosen, err := a.chooseConstructor(rec, args)
			if err != nil {
				a.errorAt(e.Pos(), fmt.Sprintf("no matching constructor for %s: %v", rec.Name, err))
			} else if !chosen.Defaulted && a.info != nil {
				a.info.Casts[e] = chosen
			}
		}
		return ExprInfo{Type: targetT, ValCat: PrValue}

	case *ast.SizeofExpr:
		// Sizeof yields std::size_t.
		if e.Ellipsis.IsValid() {
			// Parameter pack size.
			if n, ok := a.packLength(e.X); ok {
				a.noteConst(e, n)
				return ExprInfo{Type: a.sizeT(), ValCat: PrValue, IsConst: true, ConstVal: n}
			}
			if a.dependentContext() {
				return dependentExpr()
			}
			a.errorAt(e.Pos(), "sizeof... of something that is not a parameter pack")
			return ExprInfo{Type: a.sizeT(), ValCat: PrValue}
		}
		if e.Type != nil {
			a.noteTypeId(e.Type)
		} else if e.X != nil {
			a.unevaluated++
			a.CheckExpr(e.X)
			a.unevaluated--
		}
		return ExprInfo{Type: a.sizeT(), ValCat: PrValue, IsConst: true}

	case *ast.AlignofExpr:
		a.noteTypeId(e.Type)
		return ExprInfo{Type: a.sizeT(), ValCat: PrValue, IsConst: true}

	case *ast.LambdaExpr:
		return a.checkLambdaExpr(e)

	case *ast.InitList:
		// Braced initializer list type deduction for range-for.
		var elem types.Type
		for _, item := range e.Items {
			info := a.CheckExpr(item)
			if elem == nil {
				elem = types.Unqualify(types.RemoveReference(info.Type))
			}
		}
		if elem == nil {
			elem = types.Typ(types.Int)
		}
		return ExprInfo{
			Type:   &types.Array{Elem: elem, Len: int64(len(e.Items))},
			ValCat: PrValue,
		}

	case *ast.NewExpr:
		// Pointer to allocated type.
		allocT := BuildDeclarator(e.Type.Decl, BuildDeclSpecs(e.Type.Specs, a.curScope, a.unit).Type, a.curScope, a.unit)
		if count := newArrayCount(e); count != nil {
			// Array new-declarator bound must be integral.
			if info := a.CheckExpr(count); !isDependentExpr(info) && !types.IsInteger(types.Unqualify(types.RemoveReference(info.Type))) && !types.IsEnum(types.Unqualify(info.Type)) {
				a.errorAt(count.Pos(), fmt.Sprintf("the array bound of a new-expression must be an integer, not %q", info.Type))
			}
		}
		placement := []Argument{{Type: a.sizeT()}}
		for _, arg := range e.Placement {
			info := a.CheckExpr(arg)
			placement = append(placement, Argument{Type: info.Type, IsLValue: info.ValCat == LValue, NullConst: isNullConstant(arg, info)})
		}
		if arr, ok := allocT.(*types.Array); ok {
			allocT = arr.Elem
		}
		if len(e.Placement) > 0 && !a.dependentContext() {
			// Allocation function selection for placement new.
			var candidates []*FuncSymbol
			for _, sym := range LookupUnqualified(a.globalScope, "operator new") {
				if fn, isFn := sym.(*FuncSymbol); isFn {
					candidates = append(candidates, fn)
				}
			}
			if len(candidates) == 0 {
				a.errorAt(e.Pos(), "placement new needs an operator new that takes the placement arguments; <new> declares one")
			} else if chosen, err := ResolveOverload(candidates, placement); err != nil {
				a.errorAt(e.Pos(), fmt.Sprintf("no operator new takes these placement arguments: %v", err))
			} else if a.info != nil {
				a.info.Allocs[e] = chosen
				a.ensureInstantiated(chosen)
			}
		}
		// New-expression initialization.
		var args []Argument
		var items []ast.Expr
		switch init := e.Init.(type) {
		case *ast.ParenExpr:
			e.Args = a.expandPackArgs(e.Args)
			items = e.Args
		case *ast.InitList:
			init.Items = a.expandPackArgs(init.Items)
			items = init.Items
		case nil:
		default:
			items = []ast.Expr{init}
		}
		anyDependent := false
		for _, item := range items {
			info := a.CheckExpr(item)
			if isDependentExpr(info) {
				anyDependent = true
			}
			args = append(args, Argument{Type: info.Type, IsLValue: info.ValCat == LValue})
		}
		if rec := types.AsRecord(types.Unqualify(allocT)); rec != nil && hasUserConstructor(rec) && !isDependentType(allocT) && !(anyDependent && a.dependentContext()) {
			chosen, err := a.chooseConstructor(rec, args)
			if err != nil {
				a.errorAt(e.Pos(), fmt.Sprintf("no matching constructor for %s: %v", rec.Name, err))
			} else if !chosen.Defaulted && a.info != nil {
				a.info.News[e] = chosen
			}
			if chosen != nil {
				for i := len(e.Args); i < len(chosen.Defaults); i++ {
					if def := chosen.Defaults[i]; def != nil {
						e.Args = append(e.Args, def)
						a.CheckExpr(def)
					}
				}
			}
		}
		return ExprInfo{Type: &types.Pointer{Elem: allocT}, ValCat: PrValue}

	case *ast.DeleteExpr:
		// Delete expression destructor invocation.
		if e.X != nil {
			info := a.CheckExpr(e.X)
			if ptr, isPtr := types.Unqualify(info.Type).(*types.Pointer); isPtr {
				if rec := types.AsRecord(types.Unqualify(ptr.Elem)); rec != nil && a.info != nil {
					for _, m := range rec.Methods {
						if m.Name == "~"+rec.Name && !m.Defaulted {
							a.info.Deletes[e] = &FuncSymbol{SymName: m.Name, FuncType: m.Func, InClass: rec}
						}
					}
				}
			}
		}
		return ExprInfo{Type: types.Typ(types.Void), ValCat: PrValue}

	case *ast.TypeTraitExpr:
		// Compiler builtin type traits.
		name := e.Name.Text(a.unit)
		anyDependent := false
		for _, arg := range e.Args {
			if arg != nil {
				if t := a.noteTypeId(arg); t == nil || isDependentType(t) {
					anyDependent = true
				}
			}
		}
		if name == "__builtin_is_constant_evaluated" || name == "__is_constant_evaluated" {
			return ExprInfo{Type: types.Typ(types.Bool), ValCat: PrValue}
		}
		if name == "__builtin_offsetof" {
			if a.dependentContext() {
				return dependentExpr()
			}
			if n, err := a.NewConstContext().EvalInt(e); err == nil {
				a.noteConst(e, n)
				return ExprInfo{Type: a.sizeT(), ValCat: PrValue, IsConst: true, ConstVal: n}
			}
			return ExprInfo{Type: a.sizeT(), ValCat: PrValue}
		}
		// Traits on dependent types are dependent.
		if a.dependentContext() && anyDependent {
			return dependentExpr()
		}
		if v, err := a.NewConstContext().EvalBool(e); err == nil {
			n := int64(0)
			if v {
				n = 1
			}
			a.noteConst(e, n)
			return ExprInfo{Type: types.Typ(types.Bool), ValCat: PrValue, IsConst: true, ConstVal: n}
		}
		return ExprInfo{Type: types.Typ(types.Bool), ValCat: PrValue}
	}

	return ExprInfo{Type: types.Typ(types.Int), ValCat: PrValue}
}

// noteConst records the value of an expression the analysis evaluated.
func (a *Analyzer) noteConst(e ast.Expr, n int64) {
	if a.info != nil {
		a.info.Consts[e] = n
	}
}

func (a *Analyzer) checkBasicLit(lit *ast.BasicLit) ExprInfo {
	switch lit.Kind {
	case token.INT_LIT:
		text := lit.Spelling(a.unit)
		// Evaluate integer literal.
		val, err := a.NewConstContext().EvalInt(lit)
		if err != nil {
			val = 0
		}
		return ExprInfo{
			Type:     integerLiteralType(text, val),
			ValCat:   PrValue,
			ConstVal: val,
			IsConst:  true,
		}
	case token.FLOAT_LIT:
		// Double unless a suffix specifies float or long double.
		text := strings.ToLower(lit.Spelling(a.unit))
		if strings.HasPrefix(text, "0x") {
			// A hexadecimal floating literal's suffix follows its p-exponent.
			if i := strings.IndexByte(text, 'p'); i >= 0 {
				text = text[i:]
			}
		}
		switch {
		case strings.HasSuffix(text, "f") && !strings.HasPrefix(text, "0x"):
			return ExprInfo{Type: types.Typ(types.Float), ValCat: PrValue}
		case strings.HasSuffix(text, "l"):
			return ExprInfo{Type: types.Typ(types.LongDouble), ValCat: PrValue}
		}
		return ExprInfo{Type: types.Typ(types.Double), ValCat: PrValue}
	case token.CHAR_LIT:
		text := lit.Spelling(a.unit)
		switch {
		case strings.HasPrefix(text, "L"):
			return ExprInfo{Type: types.Typ(types.WChar), ValCat: PrValue}
		case strings.HasPrefix(text, "u8"):
			return ExprInfo{Type: types.Typ(types.Char8), ValCat: PrValue}
		case strings.HasPrefix(text, "u"):
			return ExprInfo{Type: types.Typ(types.Char16), ValCat: PrValue}
		case strings.HasPrefix(text, "U"):
			return ExprInfo{Type: types.Typ(types.Char32), ValCat: PrValue}
		}
		return ExprInfo{Type: types.Typ(types.Char), ValCat: PrValue}
	case token.TRUE, token.FALSE:
		val := int64(0)
		if lit.Kind == token.TRUE {
			val = 1
		}
		return ExprInfo{Type: types.Typ(types.Bool), ValCat: PrValue, ConstVal: val, IsConst: true}
	case token.NULLPTR:
		return ExprInfo{Type: types.Typ(types.NullptrKind), ValCat: PrValue}
	}
	return ExprInfo{Type: types.Typ(types.Int), ValCat: PrValue}
}

// integerLiteralType returns the type of an integer literal:
// the first type in the candidate list that can represent the value.
func integerLiteralType(text string, val int64) types.Type {
	lower := strings.ToLower(text)
	unsigned := strings.ContainsRune(suffixOf(lower), 'u')
	longLong := strings.Contains(suffixOf(lower), "ll")
	long := !longLong && strings.Contains(suffixOf(lower), "l")

	// Non-decimal literals may take unsigned types without 'u' suffix.
	decimal := !strings.HasPrefix(lower, "0x") && !strings.HasPrefix(lower, "0b") &&
		!(len(lower) > 1 && lower[0] == '0')

	switch {
	case unsigned && longLong:
		return types.Typ(types.ULongLong)
	case unsigned && long:
		if uint64(val) <= 0xFFFFFFFF {
			return types.Typ(types.ULong)
		}
		return types.Typ(types.ULongLong)
	case unsigned:
		if uint64(val) <= 0xFFFFFFFF {
			return types.Typ(types.UInt)
		}
		return types.Typ(types.ULongLong)
	case longLong:
		return types.Typ(types.LongLong)
	case long:
		if val >= -2147483648 && val <= 2147483647 {
			return types.Typ(types.Long)
		}
		return types.Typ(types.LongLong)
	case val >= -2147483648 && val <= 2147483647:
		return types.Typ(types.Int)
	case !decimal && uint64(val) <= 0xFFFFFFFF:
		return types.Typ(types.UInt)
	default:
		return types.Typ(types.LongLong)
	}
}

// suffixOf returns the trailing suffix letters of an integer literal.
func suffixOf(lower string) string {
	digits := "0123456789'"
	if strings.HasPrefix(lower, "0x") {
		digits = "0123456789abcdef'"
		lower = lower[2:]
	} else if strings.HasPrefix(lower, "0b") {
		digits = "01'"
		lower = lower[2:]
	}
	for i := len(lower) - 1; i >= 0; i-- {
		if strings.ContainsRune(digits, rune(lower[i])) {
			return lower[i+1:]
		}
	}
	return ""
}

func (a *Analyzer) symToExprInfo(sym Symbol) ExprInfo {
	switch s := sym.(type) {
	case *DependentSymbol:
		return dependentExpr()
	case *PackSymbol:
		return dependentExpr()
	case *VarSymbol:
		// Expressions never have reference type.
		return ExprInfo{Type: types.RemoveReference(s.Type()), ValCat: LValue}
	case *FuncSymbol:
		return ExprInfo{Type: s.Type(), ValCat: LValue}
	case *EnumeratorSymbol:
		return ExprInfo{Type: s.Type(), ValCat: PrValue, ConstVal: s.Val, IsConst: true}
	case *TemplateParamSymbol:
		// Non-type template parameters are prvalues.
		if s.IsType || s.SymType == nil {
			return ExprInfo{Type: types.Typ(types.Int), ValCat: PrValue}
		}
		return dependentExpr()
	}
	return ExprInfo{Type: sym.Type(), ValCat: LValue}
}

// incrementable reports whether t can be incremented or decremented.
func incrementable(t types.Type) bool {
	t = types.Unqualify(types.RemoveReference(t))
	if mentionsAuto(t) || isDependentType(t) {
		return true // decided when the type is known
	}
	if types.IsArithmetic(t) {
		return !types.IsBool(t)
	}
	return types.IsPointer(t)
}

// comparable is the operand a built-in relational or equality operator
// accepts: arithmetic, enumeration, pointer, member pointer or nullptr.
func comparable(t types.Type) bool {
	t = types.Unqualify(types.RemoveReference(t))
	if mentionsAuto(t) || isDependentType(t) {
		return true // decided when the type is known
	}
	if types.IsArithmetic(t) || types.IsEnum(t) || types.IsPointer(t) || types.IsNullptr(t) {
		return true
	}
	_, isMemPtr := t.(*types.MemberPointer)
	return isMemPtr
}

func contextuallyConvertibleToBool(t types.Type) bool {
	if types.IsConvertible(t, types.Typ(types.Bool)) {
		return true
	}
	rec := types.AsRecord(types.Unqualify(types.RemoveReference(t)))
	if rec == nil {
		return false
	}
	for _, m := range rec.Methods {
		if m.Name == "operator bool" && len(m.Func.Params) == 0 {
			return true
		}
		// Conversion operator to a type convertible to bool.
		if len(m.Func.Params) == 0 && !m.Explicit && m.Func.Ret != nil &&
			types.IsConvertible(m.Func.Ret, types.Typ(types.Bool)) &&
			len(m.Name) > len("operator ") && m.Name[:len("operator ")] == "operator " {
			return true
		}
	}
	return false
}

func (a *Analyzer) checkUnaryExpr(u *ast.UnaryExpr) ExprInfo {
	sub := a.CheckExpr(u.X)

	if isDependentExpr(sub) {
		return dependentExpr()
	}

	// Prefix operator overload resolution.
	if rec := types.AsRecord(types.Unqualify(types.RemoveReference(sub.Type))); rec != nil {
		if opName := OperatorSpelling(u.Op); opName != "" {
			if info, resolved := a.resolveOperator(u, opName, []ExprInfo{sub}, u.Pos()); resolved {
				return info
			}
		}
	}

	switch u.Op {
	case token.AND:
		// Pointer-to-member expression `&C::m`.
		if qn, isQualified := unparenExpr(u.X).(*ast.QualifiedName); isQualified && len(qn.Qual) > 0 {
			if ref := a.memberRef(qn); ref != nil {
				if a.info != nil {
					a.info.MemberPointers[u] = ref
				}
				return ExprInfo{Type: &types.MemberPointer{Class: ref.Class, Elem: ref.Type}, ValCat: PrValue}
			}
		}
		// Address-of yields pointer to designated object.
		if sub.ValCat != LValue {
			a.errorAt(u.Pos(), "cannot take address of rvalue")
		}
		return ExprInfo{Type: &types.Pointer{Elem: types.RemoveReference(sub.Type)}, ValCat: PrValue}

	case token.MUL:
		// Dereference: operand must be a pointer
		ptr := types.AsPointer(sub.Type)
		if ptr == nil {
			a.errorAt(u.Pos(), fmt.Sprintf("cannot dereference non-pointer type %q", sub.Type))
			return ExprInfo{Type: types.Typ(types.Int), ValCat: LValue}
		}
		return ExprInfo{Type: ptr.Elem, ValCat: LValue}

	case token.NOT:
		if !contextuallyConvertibleToBool(sub.Type) {
			a.errorAt(u.Pos(), "operand of ! must be convertible to bool")
		}
		return ExprInfo{Type: types.Typ(types.Bool), ValCat: PrValue}

	case token.TILDE, token.ADD, token.SUB:
		if !types.IsArithmetic(sub.Type) {
			a.errorAt(u.Pos(), fmt.Sprintf("invalid operand of type %q to unary %s", sub.Type, u.Op))
		}
		return ExprInfo{Type: types.Unqualify(sub.Type), ValCat: PrValue}

	case token.INC, token.DEC:
		if sub.ValCat != LValue {
			a.errorAt(u.Pos(), "operand of prefix ++/-- must be an lvalue")
		}
		if types.IsConst(sub.Type) {
			a.errorAt(u.Pos(), "cannot modify const object with ++/--")
		}
		if !incrementable(sub.Type) {
			a.errorAt(u.Pos(), fmt.Sprintf("no operator%s for an operand of type %q", u.Op, sub.Type))
		}
		return ExprInfo{Type: sub.Type, ValCat: LValue}
	}

	return sub
}

func (a *Analyzer) checkBinaryExpr(b *ast.BinaryExpr) ExprInfo {
	left := a.CheckExpr(b.X)
	right := a.CheckExpr(b.Y)

	// Dependent operands cannot be resolved until instantiation.
	if isDependentExpr(left) || isDependentExpr(right) {
		return dependentExpr()
	}

	lhsRec := types.AsRecord(types.Unqualify(types.RemoveReference(left.Type)))
	rhsRec := types.AsRecord(types.Unqualify(types.RemoveReference(right.Type)))
	if (lhsRec != nil || rhsRec != nil || types.IsEnum(types.Unqualify(types.RemoveReference(left.Type)))) && OperatorSpelling(b.Op) != "" {
		if info, resolved := a.resolveOperator(b, OperatorSpelling(b.Op), []ExprInfo{left, right}, b.Pos()); resolved {
			return info
		}
		if info, rewritten := a.rewrittenEquality(b, left, right); rewritten {
			return info
		}
		if info, rewritten := a.rewrittenRelational(b, left, right); rewritten {
			return info
		}
	}
	if b.Op == token.SPACESHIP {
		return a.builtinSpaceship(b, left, right)
	}

	// Array-to-pointer decay.
	leftT := decayed(left.Type)
	rightT := decayed(right.Type)

	switch b.Op {
	case token.ADD, token.SUB, token.MUL, token.QUO, token.REM:
		if !types.IsArithmetic(leftT) || !types.IsArithmetic(rightT) {
			// Pointer arithmetic (ptr + int, int + ptr, ptr - int, ptr - ptr)
			if b.Op == token.ADD && types.IsPointer(leftT) && types.IsInteger(rightT) {
				return ExprInfo{Type: leftT, ValCat: PrValue}
			}
			if b.Op == token.ADD && types.IsInteger(leftT) && types.IsPointer(rightT) {
				return ExprInfo{Type: rightT, ValCat: PrValue}
			}
			if b.Op == token.SUB && types.IsPointer(leftT) {
				if types.IsInteger(rightT) {
					return ExprInfo{Type: leftT, ValCat: PrValue}
				}
				if types.IsPointer(rightT) {
					return ExprInfo{Type: types.Typ(types.LongLong), ValCat: PrValue} // ptrdiff_t
				}
			}
			a.errorAt(b.Pos(), fmt.Sprintf("invalid operands to binary %s (%q and %q)", b.Op, left.Type, right.Type))
		}
		ct := types.CommonType(left.Type, right.Type)
		return ExprInfo{Type: ct, ValCat: PrValue}

	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		// Built-in comparison operators take arithmetic, enum, or pointer operands.
		if !comparable(leftT) || !comparable(rightT) {
			a.errorAt(b.Pos(), fmt.Sprintf("invalid operands to binary %s (%q and %q)", b.Op, left.Type, right.Type))
		}
		return ExprInfo{Type: types.Typ(types.Bool), ValCat: PrValue}

	case token.LAND, token.LOR:
		if !contextuallyConvertibleToBool(left.Type) {
			a.errorAt(b.X.Pos(), "left operand must be convertible to bool")
		}
		if !contextuallyConvertibleToBool(right.Type) {
			a.errorAt(b.Y.Pos(), "right operand must be convertible to bool")
		}
		return ExprInfo{Type: types.Typ(types.Bool), ValCat: PrValue}

	case token.COMMA:
		return right

	case token.PERIOD_STAR, token.ARROW_STAR:
		// Pointer-to-member operator .* or ->*.
		mp, isMP := types.Unqualify(right.Type).(*types.MemberPointer)
		if !isMP {
			a.errorAt(b.Y.Pos(), fmt.Sprintf("the right operand of %s must be a pointer to member, not %q", b.Op, right.Type))
			return ExprInfo{Type: types.Typ(types.Int), ValCat: LValue}
		}
		objT := types.Unqualify(types.RemoveReference(left.Type))
		if b.Op == token.ARROW_STAR {
			ptr, isPtr := objT.(*types.Pointer)
			if !isPtr {
				a.errorAt(b.X.Pos(), fmt.Sprintf("the left operand of ->* must be a pointer, not %q", left.Type))
				return ExprInfo{Type: types.Typ(types.Int), ValCat: LValue}
			}
			objT = types.Unqualify(ptr.Elem)
		}
		if rec := types.AsRecord(objT); rec == nil || rec != types.AsRecord(types.Unqualify(mp.Class)) && !types.IsBaseOf(mp.Class, rec) {
			a.errorAt(b.X.Pos(), fmt.Sprintf("the left operand of %s is a %q, not an object of %q", b.Op, left.Type, mp.Class))
		}
		if types.IsFunc(mp.Elem) {
			// A bound member function: only callable, and the call sees
			// the function type here.
			return ExprInfo{Type: mp.Elem, ValCat: PrValue}
		}
		return ExprInfo{Type: mp.Elem, ValCat: LValue}

	case token.SHL, token.SHR:
		// Shift operands are promoted and result has the promoted left operand type.
		if !types.IsInteger(leftT) && !types.IsEnum(leftT) || !types.IsInteger(rightT) && !types.IsEnum(rightT) {
			a.errorAt(b.Pos(), fmt.Sprintf("invalid operands to binary %s (%q and %q)", b.Op, left.Type, right.Type))
			return ExprInfo{Type: types.Typ(types.Int), ValCat: PrValue}
		}
		return ExprInfo{Type: types.CommonType(leftT, leftT), ValCat: PrValue}

	case token.AND, token.OR, token.XOR:
		// Bitwise operators perform usual arithmetic conversions.
		if !types.IsInteger(leftT) && !types.IsEnum(leftT) && !types.IsBool(leftT) || !types.IsInteger(rightT) && !types.IsEnum(rightT) && !types.IsBool(rightT) {
			a.errorAt(b.Pos(), fmt.Sprintf("invalid operands to binary %s (%q and %q)", b.Op, left.Type, right.Type))
			return ExprInfo{Type: types.Typ(types.Int), ValCat: PrValue}
		}
		return ExprInfo{Type: types.CommonType(leftT, rightT), ValCat: PrValue}
	}

	return ExprInfo{Type: types.Typ(types.Int), ValCat: PrValue}
}

func (a *Analyzer) checkAssignExpr(as *ast.AssignExpr) ExprInfo {
	left := a.CheckExpr(as.Lhs)
	right := a.CheckExpr(as.Rhs)

	// Assignment is to the dereferenced object type.
	target := types.RemoveReference(left.Type)

	if isDependentExpr(left) || isDependentExpr(right) {
		return dependentExpr()
	}

	// Overloaded operator= resolution.
	if rec := types.AsRecord(types.Unqualify(target)); rec != nil {
		if info, resolved := a.resolveOperator(as, assignSpelling(as.Op), []ExprInfo{left, right}, as.Pos()); resolved {
			return info
		}
	}

	if left.ValCat != LValue {
		a.errorAt(as.Lhs.Pos(), "left-hand side of assignment must be an lvalue")
	}
	if types.IsConst(target) {
		a.errorAt(as.Lhs.Pos(), fmt.Sprintf("cannot assign to const-qualified type %q", target))
	}

	// Pointer arithmetic for += and -=.
	if as.Op == token.ADD_ASSIGN || as.Op == token.SUB_ASSIGN {
		if _, isPtr := types.Unqualify(target).(*types.Pointer); isPtr && types.IsInteger(types.Unqualify(types.RemoveReference(right.Type))) {
			return ExprInfo{Type: left.Type, ValCat: LValue}
		}
	}

	cs := ClassifyConversion(right.Type, target, right.ValCat == LValue)
	if !cs.Valid && !isDependentType(target) {
		a.errorAt(as.Rhs.Pos(), fmt.Sprintf("cannot convert %q to %q in assignment", right.Type, target))
	}

	return ExprInfo{Type: left.Type, ValCat: LValue}
}

func (a *Analyzer) checkCallExpr(c *ast.CallExpr) ExprInfo {
	// Explicit type conversion `T(a, b)` using call syntax.
	c.Args = a.expandPackArgs(c.Args)
	if t, ok := a.calleeNamesAType(c.Fun); ok {
		// What the callee names is recorded, so that lowering sees a
		// conversion and not a call it has no function for.
		if _, recorded := a.info.Uses[c.Fun]; !recorded {
			a.record(c.Fun, &TypeSymbol{SymType: t})
		}
		args := make([]Argument, 0, len(c.Args))
		for _, arg := range c.Args {
			info := a.CheckExpr(arg)
			args = append(args, Argument{Type: info.Type, IsLValue: info.ValCat == LValue})
		}
		// Class type conversion constructs a temporary.
		if rec := types.AsRecord(types.Unqualify(t)); rec != nil && hasUserConstructor(rec) && !isDependentType(t) && !dependentArguments(args) {
			chosen, err := a.chooseConstructor(rec, args)
			if err != nil {
				a.errorAt(c.Pos(), fmt.Sprintf("no matching constructor for %s: %v", rec.Name, err))
			} else if !chosen.Defaulted && a.info != nil {
				a.info.Temporaries[c] = chosen
			}
			if chosen != nil {
				for i := len(c.Args); i < len(chosen.Defaults); i++ {
					if def := chosen.Defaults[i]; def != nil {
						c.Args = append(c.Args, def)
						a.CheckExpr(def)
					}
				}
			}
		}
		return ExprInfo{Type: t, ValCat: PrValue}
	}

	// Compiler builtins in call position.
	if id, isIdent := c.Fun.(*ast.Ident); isIdent {
		if info, isBuiltin := a.builtinCall(id.Text(a.unit), c); isBuiltin {
			return info
		}
	}

	// Call through pointer-to-member function `(obj.*pmf)(args)`.
	if bin, isBin := unparenExpr(c.Fun).(*ast.BinaryExpr); isBin && (bin.Op == token.PERIOD_STAR || bin.Op == token.ARROW_STAR) {
		funInfo := a.CheckExpr(c.Fun)
		if ft, isFunc := types.Unqualify(funInfo.Type).(*types.Func); isFunc {
			for i, arg := range c.Args {
				info := a.CheckExpr(arg)
				if i < len(ft.Params) && !isDependentExpr(info) {
					if cs := ClassifyConversion(info.Type, ft.Params[i].Type, info.ValCat == LValue); !cs.Valid && !isNullConstant(arg, info) {
						a.errorAt(arg.Pos(), fmt.Sprintf("cannot convert argument of type %q to %q", info.Type, ft.Params[i].Type))
					}
				}
			}
			ret := ft.Ret
			cat := PrValue
			if types.IsLValueReference(ret) {
				cat = LValue
			}
			return ExprInfo{Type: types.RemoveReference(ret), ValCat: cat}
		}
	}

	// Collect argument info
	args := make([]Argument, len(c.Args))
	anyDependent := false
	for i, arg := range c.Args {
		argInfo := a.CheckExpr(arg)
		args[i] = Argument{
			Type:      argInfo.Type,
			IsLValue:  argInfo.ValCat == LValue,
			NullConst: isNullConstant(arg, argInfo),
		}
		if isDependentExpr(argInfo) {
			anyDependent = true
		}
	}
	// Dependent call is resolved at instantiation.
	if anyDependent && a.dependentContext() {
		return dependentExpr()
	}

	var candidates []*FuncSymbol
	var explicit []types.Type
	var object *Argument
	if id, ok := c.Fun.(*ast.Ident); ok {
		syms := LookupUnqualified(a.curScope, id.Text(a.unit))
		foundMember := false
		for _, s := range syms {
			if fn, isFn := s.(*FuncSymbol); isFn {
				if reg := a.methodSyms[fn.Method]; reg != nil {
					if reg.TemplateOf != nil {
						continue
					}
					fn = reg
				}
				if fn.InClass != nil {
					foundMember = true
				}
				candidates = append(candidates, fn)
			}
		}
		// Implicit 'this' object argument.
		if len(candidates) > 0 && candidates[0].InClass != nil && a.curFunc != nil && a.curFunc.InClass != nil && !a.curFunc.Static {
			object = &Argument{Type: types.Qualify(a.curFunc.InClass, a.curFunc.FuncType.Quals), IsLValue: true}
		}
		// Argument-dependent lookup (ADL) in associated namespaces.
		if !foundMember {
			candidates = a.addAssociated(candidates, id.Text(a.unit), args)
		}
	} else if qn, ok := c.Fun.(*ast.QualifiedName); ok {
		syms := ResolveQualifiedName(qn, a.curScope, a.globalScope, a.unit)
		if os.Getenv("VCX_DEBUG_CALL") != "" {
			for _, s := range syms {
				fmt.Fprintf(os.Stderr, "callee %s -> %T %s\n", NameString(qn, a.unit), s, s.Type())
			}
		}
		for _, s := range syms {
			if fn, isFn := s.(*FuncSymbol); isFn {
				candidates = append(candidates, fn)
			}
		}
		// Explicit template arguments on qualified call.
		if tn, isTemplate := qn.Name.(*ast.TemplateName); isTemplate {
			var dependent bool
			explicit, dependent = a.explicitTemplateArgs(tn)
			if dependent {
				return dependentExpr()
			}
		}
	} else if tn, ok := c.Fun.(*ast.TemplateName); ok {
		// Explicit template arguments.
		for _, sym := range LookupUnqualified(a.curScope, NameString(tn.Name, a.unit)) {
			if fn, isFn := sym.(*FuncSymbol); isFn {
				candidates = append(candidates, fn)
			}
		}
		var dependent bool
		explicit, dependent = a.explicitTemplateArgs(tn)
		if dependent {
			return dependentExpr()
		}
	} else if mem, ok := c.Fun.(*ast.MemberExpr); ok {
		lhs := a.CheckExpr(mem.X)
		objT := types.Unqualify(types.RemoveReference(lhs.Type))
		object = &Argument{Type: types.RemoveReference(lhs.Type), IsLValue: lhs.ValCat == LValue}
		if mem.Op == token.ARROW {
			// Member access through arrow operator.
			if pointee := a.arrowPointee(mem, lhs); pointee != nil {
				objT = types.Unqualify(pointee)
				object = &Argument{Type: pointee, IsLValue: true}
			}
		}
		rec := types.AsRecord(objT)
		if rec != nil {
			memberName := NameString(mem.Sel, a.unit)
			// Qualified member access c.A::f() starts lookup in base A.
			if qn, qualified := mem.Sel.(*ast.QualifiedName); qualified && len(qn.Qual) > 0 {
				if base := findBaseNamed(rec, NameString(qn.Qual[len(qn.Qual)-1], a.unit)); base != nil {
					rec = base
				}
				memberName = NameString(qn.Name, a.unit)
			}
			syms := lookupRecordMember(rec, memberName, make(map[*types.Record]bool))
			for _, s := range syms {
				if fn, isFn := s.(*FuncSymbol); isFn {
					if reg := a.methodSyms[fn.Method]; reg != nil {
						if reg.TemplateOf != nil {
							continue
						}
						fn = reg
					}
					candidates = append(candidates, fn)
				}
			}
		}
	}

	if len(candidates) > 0 {
		resolved, err := a.resolveAmongOn(candidates, object, explicit, args, c.Pos())
		if err != nil {
			a.errorAt(c.Pos(), err.Error())
			return ExprInfo{Type: types.Typ(types.Int), ValCat: PrValue}
		}
		a.ensureInstantiated(resolved)
		if a.info != nil {
			a.info.Calls[c] = resolved
		}
		a.noteArgConversions(resolved, c.Args, args, c.Pos())
		for i := len(c.Args); i < len(resolved.Defaults); i++ {
			if def := resolved.Defaults[i]; def != nil {
				c.Args = append(c.Args, def)
				a.CheckExpr(def)
			}
		}
		ret := resolved.FuncType.Ret
		valCat := PrValue
		if types.IsLValueReference(ret) {
			valCat = LValue
		} else if types.IsRValueReference(ret) {
			valCat = XValue
		}
		return ExprInfo{Type: types.RemoveReference(ret), ValCat: valCat}
	}

	calleeInfo := a.CheckExpr(c.Fun)

	// Direct call to a function type
	if ft, ok := types.Unqualify(calleeInfo.Type).(*types.Func); ok {
		fnSym := &FuncSymbol{SymName: "callee", FuncType: ft}
		resolved, err := ResolveOverload([]*FuncSymbol{fnSym}, args)
		if err != nil {
			a.errorAt(c.Pos(), err.Error())
			return ExprInfo{Type: ft.Ret, ValCat: PrValue}
		}
		valCat := PrValue
		if types.IsLValueReference(resolved.FuncType.Ret) {
			valCat = LValue
		} else if types.IsRValueReference(resolved.FuncType.Ret) {
			valCat = XValue
		}
		return ExprInfo{Type: types.RemoveReference(resolved.FuncType.Ret), ValCat: valCat}
	}

	// Pointer-to-function call
	if ptr, ok := types.Unqualify(calleeInfo.Type).(*types.Pointer); ok {
		if ft, isFn := ptr.Elem.(*types.Func); isFn {
			fnSym := &FuncSymbol{SymName: "callee", FuncType: ft}
			resolved, err := ResolveOverload([]*FuncSymbol{fnSym}, args)
			if err != nil {
				a.errorAt(c.Pos(), err.Error())
				return ExprInfo{Type: ft.Ret, ValCat: PrValue}
			}
			valCat := PrValue
			if types.IsLValueReference(resolved.FuncType.Ret) {
				valCat = LValue
			} else if types.IsRValueReference(resolved.FuncType.Ret) {
				valCat = XValue
			}
			return ExprInfo{Type: types.RemoveReference(resolved.FuncType.Ret), ValCat: valCat}
		}
	}

	// Call operator() on class objects or lambdas.
	if rec := types.AsRecord(types.Unqualify(types.RemoveReference(calleeInfo.Type))); rec != nil {
		var callable []*FuncSymbol
		for _, m := range rec.Methods {
			if m.Name != "operator()" {
				continue
			}
			sig, ok := specialize(m.Func, nil, args)
			if !ok {
				continue
			}
			callable = append(callable, &FuncSymbol{
				SymName: m.Name, FuncType: sig, InClass: rec,
			})
		}
		if len(callable) > 0 {
			resolved, err := ResolveOverload(callable, args)
			if err != nil {
				a.errorAt(c.Pos(), err.Error())
				return ExprInfo{Type: types.Typ(types.Int), ValCat: PrValue}
			}
			if a.info != nil {
				a.info.Operators[c] = resolved
			}
			ret := resolved.FuncType.Ret
			cat := PrValue
			if types.IsLValueReference(ret) {
				cat = LValue
			}
			return ExprInfo{Type: types.RemoveReference(ret), ValCat: cat}
		}
	}

	// Dependent call.
	if isDependentExpr(calleeInfo) {
		return dependentExpr()
	}

	// Callee must be callable.
	a.errorAt(c.Fun.Pos(), fmt.Sprintf("called object of type %q is not a function or function pointer", calleeInfo.Type))
	return ExprInfo{Type: types.Typ(types.Int), ValCat: PrValue}
}

// findBaseNamed is the class named in a qualified member access: the object's
// own class, or one of its bases at any depth.
func findBaseNamed(r *types.Record, name string) *types.Record {
	if r == nil {
		return nil
	}
	if r.Name == name {
		return r
	}
	for _, b := range r.Bases {
		if br, ok := types.Unqualify(b.Type).(*types.Record); ok {
			if found := findBaseNamed(br, name); found != nil {
				return found
			}
		}
	}
	return nil
}

// calleeNamesAType reports whether the callee of a call expression is the
// name of a type, and yields that type.
func (a *Analyzer) calleeNamesAType(fun ast.Expr) (types.Type, bool) {
	var syms []Symbol
	switch f := fun.(type) {
	case *ast.Ident:
		syms = LookupUnqualified(a.curScope, f.Text(a.unit))
	case *ast.QualifiedName:
		syms = ResolveQualifiedName(f, a.curScope, a.globalScope, a.unit)
	case *ast.TemplateName:
		// Class template specialization used as constructor call.
		isClassTemplate := false
		for _, sym := range LookupUnqualified(a.curScope, NameString(f.Name, a.unit)) {
			if rs, isRec := sym.(*RecordSymbol); isRec && (rs.ClassTemplate != nil || rs.TemplateOf != nil) {
				isClassTemplate = true
			}
		}
		if !isClassTemplate {
			return nil, false
		}
		specs := &ast.DeclSpecs{Span: f.Span, List: []ast.DeclSpec{&ast.NamedTypeSpec{Span: f.Span, Typename: ast.NoTok, Name: f}}}
		info := BuildDeclSpecs(specs, a.curScope, a.unit)
		if info.Type == nil || info.Unresolved != "" {
			return nil, false
		}
		return info.Type, true
	default:
		return nil, false
	}

	for _, sym := range syms {
		switch s := sym.(type) {
		case *RecordSymbol:
			return s.Record, true
		case *EnumSymbol:
			return s.Enum, true
		case *TypeSymbol:
			return s.SymType, true
		case *TemplateParamSymbol:
			if s.IsType && s.SymType != nil {
				return s.SymType, true
			}
		case *FuncSymbol:
			// A function of that name is in scope, so this is a call.
			return nil, false
		}
	}
	return nil, false
}

func (a *Analyzer) checkMemberExpr(m *ast.MemberExpr) ExprInfo {
	lhs := a.CheckExpr(m.X)

	// Dependent member access.
	if isDependentExpr(lhs) {
		return dependentExpr()
	}

	var rec *types.Record

	if m.Op == token.PERIOD {
		rec = types.AsRecord(lhs.Type)
		if rec == nil {
			a.errorAt(m.X.Pos(), fmt.Sprintf("expected class or struct before '.', got %q", lhs.Type))
			return ExprInfo{Type: types.Typ(types.Int), ValCat: LValue}
		}
	} else if m.Op == token.ARROW {
		if pointee := a.arrowPointee(m, lhs); pointee != nil {
			rec = types.AsRecord(pointee)
		}
		if rec == nil {
			a.errorAt(m.X.Pos(), fmt.Sprintf("expected pointer to class before '->', got %q", lhs.Type))
			return ExprInfo{Type: types.Typ(types.Int), ValCat: LValue}
		}
	}

	memberName := NameString(m.Sel, a.unit)
	syms := lookupRecordMember(rec, memberName, make(map[*types.Record]bool))
	if len(syms) == 0 {
		if rs := a.curScope.recordSymbol(rec); rs != nil {
			syms = LookupQualified(rs, memberName)
		}
	}
	if len(syms) == 0 {
		// Dependent base classes may provide the member.
		if hasDependentBase(rec) {
			return dependentExpr()
		}
		a.errorAt(m.Sel.Pos(), fmt.Sprintf("no member named %q in %s", memberName, rec.String()))
		return ExprInfo{Type: types.Typ(types.Int), ValCat: LValue}
	}

	sym := syms[0]
	a.record(m.Sel, sym)

	// Access control checking
	for _, f := range rec.Fields {
		if f.Name == memberName {
			if !CheckAccess(rec, f.Access, a.curScope) {
				a.errorAt(m.Sel.Pos(), fmt.Sprintf("member %q is %s within this context", memberName, f.Access))
			}
			break
		}
	}

	return a.foldConstant(m, a.symToExprInfo(sym), sym)
}

// decayed applies array-to-pointer and function-to-pointer decay.
func decayed(t types.Type) types.Type {
	switch u := types.Unqualify(types.RemoveReference(t)).(type) {
	case *types.Array:
		return &types.Pointer{Elem: u.Elem}
	case *types.Func:
		return &types.Pointer{Elem: u}
	}
	return t
}

func (a *Analyzer) checkIndexExpr(idx *ast.IndexExpr) ExprInfo {
	base := a.CheckExpr(idx.X)
	var arg ExprInfo
	if len(idx.Args) > 0 {
		arg = a.CheckExpr(idx.Args[0])
	}
	if isDependentExpr(base) || isDependentExpr(arg) {
		return dependentExpr()
	}
	// Overloaded operator[].
	if rec := types.AsRecord(types.Unqualify(types.RemoveReference(base.Type))); rec != nil && len(idx.Args) == 1 {
		if info, resolved := a.resolveOperator(idx, "operator[]", []ExprInfo{base, arg}, idx.Pos()); resolved {
			return info
		}
	}

	if arr, ok := types.Unqualify(base.Type).(*types.Array); ok {
		return ExprInfo{Type: arr.Elem, ValCat: LValue}
	}
	if ptr, ok := types.Unqualify(base.Type).(*types.Pointer); ok {
		return ExprInfo{Type: ptr.Elem, ValCat: LValue}
	}

	return ExprInfo{Type: types.Typ(types.Int), ValCat: LValue}
}

// A LambdaInfo is what lowering needs of a lambda-expression: the closure
// class, its operator(), and the captures, each a field of the closure.
type LambdaInfo struct {
	Expr     *ast.LambdaExpr
	Closure  *types.Record
	Call     *FuncSymbol
	Captures []Capture

	// Scope is the scope the lambda was written in, and Inner the one its
	// parameters and body live in: a name found in neither Inner nor a
	// scope inside it, while the body is checked, is a candidate for
	// implicit capture.
	Scope, Inner *Scope

	// Invoker is the function a captureless closure converts to, recorded for lowering.
	Invoker *FuncSymbol
}

// A Capture is one entity a closure holds: by copy or by reference.
type Capture struct {
	Name  string
	Sym   *VarSymbol // the captured variable; nil for an init-capture
	ByRef bool
	Init  ast.Expr // an init-capture's initializer
	Field int      // index in Closure.Fields
}

// checkLambdaExpr builds the closure type and operator() for a lambda expression.
func (a *Analyzer) checkLambdaExpr(l *ast.LambdaExpr) ExprInfo {
	// One closure per lambda-expression, however many times the
	// expression is looked at: a declaration checks its initializer for
	// its type and again for its constructor, and the body's names must
	// resolve to the one closure's parameters.
	if a.info != nil {
		if done := a.info.Lambdas[l]; done != nil {
			return ExprInfo{Type: done.Closure, ValCat: PrValue}
		}
	}
	a.nlambdas++
	closure := &types.Record{
		Tag:      types.TagClass,
		Name:     fmt.Sprintf("<lambda_%d>", a.nlambdas),
		Scopes:   a.curScope.Path(),
		Complete: true,
	}
	info := &LambdaInfo{Expr: l, Closure: closure, Scope: a.curScope}

	// Explicit template parameter list makes the lambda generic.
	var explicitParams []*TemplateParamSymbol
	if l.Templ != nil {
		lambdaScope := a.curScope
		tScope := NewScope(a.curScope, TemplateParamScope, nil)
		explicitParams = a.templateParams(&ast.TemplateDecl{Params: l.Templ}, tScope)
		a.curScope = tScope
		defer func() { a.curScope = lambdaScope }()
	}

	// Lambda parameters.
	var params []types.Param
	generic := false
	for i, p := range l.Params {
		info := BuildDeclSpecs(p.Specs, a.curScope, a.unit)
		pt := BuildDeclarator(p.Decl, info.Type, a.curScope, a.unit)
		pack := isPackParamDecl(p)
		if pt != nil && mentionsAuto(pt) {
			generic = true
			pt = substituteAuto(pt, &types.TemplateParam{Name: fmt.Sprintf("auto#%d", i), Index: i, IsType: true, IsPack: pack})
		}
		name := ""
		if p.Decl != nil && p.Decl.DeclName() != nil {
			name = NameString(p.Decl.DeclName(), a.unit)
		}
		params = append(params, types.Param{Name: name, Type: pt, HasDefault: p.Default != nil, Pack: pack})
	}

	// Deduced return type without trailing return type.
	var ret types.Type = types.Typ(types.AutoKind)
	if l.Trailing != nil && l.Trailing.Type != nil {
		ti := BuildDeclSpecs(l.Trailing.Type.Specs, a.curScope, a.unit)
		ret = BuildDeclarator(l.Trailing.Type.Decl, ti.Type, a.curScope, a.unit)
	}

	var quals types.Qual
	if !hasSpec(l.Specs, token.MUTABLE) {
		// operator() is const unless 'mutable' is specified.
		quals |= types.QConst
	}

	call := &types.Func{
		Ret:      ret,
		Params:   params,
		Variadic: l.Vararg.IsValid(),
		Quals:    quals,
		Noexcept: l.Noexcept != nil,
	}
	sym := &FuncSymbol{SymName: "operator()", FuncType: call, InClass: closure, SymPos: l.Pos(), SymScope: a.curScope, Inline: true, Access: types.AccessPublic}
	method := &types.Method{
		Name:   "operator()",
		Func:   call,
		Access: types.AccessPublic,
	}
	closure.Methods = append(closure.Methods, method)
	sym.Method = method
	if a.methodSyms == nil {
		a.methodSyms = map[*types.Method]*FuncSymbol{}
	}
	a.methodSyms[method] = sym
	info.Call = sym

	// Bind lambda captures and parameters into scope.
	oldScope, oldFunc := a.curScope, a.curFunc
	a.curScope = NewScope(oldScope, FunctionScope, sym)
	a.curFunc = sym

	for _, cap := range l.Captures {
		if cap.This.IsValid() {
			a.errorAt(cap.Pos(), "capturing this is not supported yet")
			continue
		}
		if cap.Name == nil {
			continue
		}
		name := cap.Name.Text(a.unit)
		if cap.Init != nil {
			// An init-capture declares a member of the closure.
			ci := a.CheckExpr(cap.Init)
			t := types.Unqualify(ci.Type)
			if cap.Amp.IsValid() {
				t = types.AddLValueReference(t)
			}
			a.curScope.Insert(&VarSymbol{SymName: name, SymType: t, SymPos: cap.Pos(), Init: cap.Init})
			a.addCapture(info, Capture{Name: name, ByRef: cap.Amp.IsValid(), Init: cap.Init}, t)
			continue
		}
		var v *VarSymbol
		for _, s := range LookupUnqualified(oldScope, name) {
			if vs, isVar := s.(*VarSymbol); isVar {
				v = vs
				break
			}
		}
		if v == nil {
			a.errorAt(cap.Pos(), fmt.Sprintf("%q is not a variable that can be captured", name))
			continue
		}
		a.addCapture(info, Capture{Name: name, Sym: v, ByRef: cap.Amp.IsValid()}, v.SymType)
	}

	sym.Params = a.declareParams(call, a.curScope)
	info.Inner = a.curScope
	a.lambdas = append(a.lambdas, info)
	if a.info != nil {
		a.info.Lambdas[l] = info
	}

	// Generic lambda body checked with template parameters.
	prevParams := a.curTemplateParams
	if l.Templ != nil {
		generic = true
	}
	if generic && a.curTemplateParams == nil {
		a.curTemplateParams = append([]*TemplateParamSymbol{}, explicitParams...)
	}
	if l.Body != nil {
		a.CheckStmt(l.Body)
	}
	a.curTemplateParams = prevParams
	a.lambdas = a.lambdas[:len(a.lambdas)-1]

	a.curScope, a.curFunc = oldScope, oldFunc

	// A body with no return statement returns void.
	if call.Ret != nil && call.Ret.Kind() == types.AutoKind {
		call.Ret = types.Typ(types.Void)
	}
	if l.Body != nil && !generic {
		sym.Body = l.Body
		a.noteDeclared(sym)
	}

	// A closure with no captures converts to a function pointer.
	if len(info.Captures) == 0 && l.Default == ast.NoTok && !generic {
		target := &types.Pointer{Elem: &types.Func{Ret: call.Ret, Params: call.Params, Variadic: call.Variadic}}
		convType := &types.Func{Ret: target, Quals: types.QConst}
		conv := &FuncSymbol{SymName: "operator " + target.String(), FuncType: convType, InClass: closure, SymPos: l.Pos(), SymScope: a.curScope, Access: types.AccessPublic}
		closure.Methods = append(closure.Methods, &types.Method{Name: conv.SymName, Func: convType, Access: types.AccessPublic})
		info.Invoker = conv
	}
	if a.info != nil {
		a.info.Lambdas[l] = info
	}

	return ExprInfo{Type: closure, ValCat: PrValue}
}

// addCapture gives a closure a field for a captured entity.
func (a *Analyzer) addCapture(info *LambdaInfo, c Capture, t types.Type) {
	ft := types.Unqualify(types.RemoveReference(t))
	if c.ByRef {
		ft = types.AddLValueReference(types.RemoveReference(t))
	}
	c.Field = len(info.Closure.Fields)
	info.Closure.Fields = append(info.Closure.Fields, types.Field{Name: c.Name, Type: ft, Access: types.AccessPrivate})
	info.Captures = append(info.Captures, c)
}

// implicitCapture handles implicit capture of enclosing function locals.
func (a *Analyzer) implicitCapture(e ast.Expr, v *VarSymbol) {
	if len(a.lambdas) == 0 || v == nil {
		return
	}
	if v.Storage == StorageStatic || v.Storage == StorageExtern || v.InClass != nil || v.SymScope == nil {
		return
	}
	switch v.SymScope.Kind {
	case BlockScope, FunctionScope:
	default:
		return
	}
	for i := len(a.lambdas) - 1; i >= 0; i-- {
		info := a.lambdas[i]
		if scopeEncloses(info.Inner, v.SymScope) {
			// Declared inside this lambda: its own local or parameter.
			return
		}
		for _, c := range info.Captures {
			if c.Name == v.SymName {
				return
			}
		}
		l := info.Expr
		if l.Default == ast.NoTok {
			a.errorAt(e.Pos(), fmt.Sprintf("%q is not captured: the lambda has no capture-default", v.SymName))
			return
		}
		a.addCapture(info, Capture{Name: v.SymName, Sym: v, ByRef: l.DefKind == token.AND}, v.SymType)
	}
}

// scopeEncloses reports whether outer is inner or one of its ancestors.
func scopeEncloses(outer, inner *Scope) bool {
	for s := inner; s != nil; s = s.Parent {
		if s == outer {
			return true
		}
	}
	return false
}

// hasSpec reports whether one of the lambda's specifiers is the given
// keyword -- `mutable`, `constexpr`, `static`.
func hasSpec(specs []ast.DeclSpec, k token.Kind) bool {
	for _, spec := range specs {
		if b, ok := spec.(*ast.BasicSpec); ok && b.Kind == k {
			return true
		}
	}
	return false
}

// noteTypeId builds a type-id in expression position and records it.
func (a *Analyzer) noteTypeId(id *ast.TypeId) types.Type {
	if id == nil {
		return nil
	}
	info := BuildDeclSpecs(id.Specs, a.curScope, a.unit)
	t := BuildDeclarator(id.Decl, info.Type, a.curScope, a.unit)
	if a.info != nil && t != nil {
		a.info.TypeIds[id] = t
	}
	return t
}

// sizeT returns the std::size_t type for the target model.
func (a *Analyzer) sizeT() types.Type {
	if sz, ok := a.model.Sizeof(types.Typ(types.Long)); ok && sz == a.model.SizePtr {
		return types.Typ(types.ULong)
	}
	return types.Typ(types.ULongLong)
}

// isNullConstant reports whether e is an integer literal with value zero.
func isNullConstant(e ast.Expr, info ExprInfo) bool {
	lit, isLit := unparenExpr(e).(*ast.BasicLit)
	return isLit && lit.Kind == token.INT_LIT && info.IsConst && info.ConstVal == 0
}

func unparenExpr(e ast.Expr) ast.Expr {
	for {
		p, isParen := e.(*ast.ParenExpr)
		if !isParen {
			return e
		}
		e = p.X
	}
}

// NewArrayCount is the bound expression of `new T[n]`, or nil for a
// non-array new-expression.
func NewArrayCount(e *ast.NewExpr) ast.Expr { return newArrayCount(e) }

func newArrayCount(e *ast.NewExpr) ast.Expr {
	if e.Type == nil {
		return nil
	}
	for d := e.Type.Decl; d != nil; {
		switch node := d.(type) {
		case *ast.ArrayDeclarator:
			return node.Size
		case *ast.PointerDeclarator:
			d = node.Inner
		case *ast.ParenDeclarator:
			d = node.Inner
		default:
			return nil
		}
	}
	return nil
}

// A MemberRef is what `&C::m` names: the class, the member's type, and
// the member -- a field by name, or a member function.
type MemberRef struct {
	Class *types.Record
	Type  types.Type
	Field string      // a data member's name
	Func  *FuncSymbol // a member function
}

// memberRef resolves a qualified name to a non-static member of a class,
// or nil when it names anything else.
func (a *Analyzer) memberRef(qn *ast.QualifiedName) *MemberRef {
	var rec *types.Record
	for _, sym := range ResolveQualifiedName(&ast.QualifiedName{Span: qn.Span, Qual: qn.Qual[:len(qn.Qual)-1], Name: qn.Qual[len(qn.Qual)-1]}, a.curScope, a.globalScope, a.unit) {
		if rs, isRec := sym.(*RecordSymbol); isRec {
			rec = rs.Record
			break
		}
	}
	if rec == nil {
		return nil
	}
	name := NameString(qn.Name, a.unit)
	for _, sym := range lookupRecordMember(rec, name, make(map[*types.Record]bool)) {
		switch s := sym.(type) {
		case *VarSymbol:
			if s.InClass != nil || s.Storage == StorageStatic {
				return nil // a static member has an ordinary address
			}
			return &MemberRef{Class: rec, Type: s.SymType, Field: name}
		case *FuncSymbol:
			if s.Static {
				return nil
			}
			if reg := a.methodSyms[s.Method]; reg != nil {
				s = reg
			}
			return &MemberRef{Class: rec, Type: s.FuncType, Func: s}
		}
	}
	return nil
}

// arrowPointee is what `x->` reaches, unwrapping operator->() chains until a pointer is found.
func (a *Analyzer) arrowPointee(m *ast.MemberExpr, lhs ExprInfo) types.Type {
	t, cat := lhs.Type, lhs.ValCat
	var chain []*FuncSymbol
	for depth := 0; depth < 16; depth++ {
		if ptr := types.AsPointer(t); ptr != nil {
			if len(chain) > 0 && a.info != nil {
				a.info.Arrows[m] = chain
			}
			return ptr.Elem
		}
		rec := types.AsRecord(types.Unqualify(types.RemoveReference(t)))
		if rec == nil {
			return nil
		}
		cands := a.memberFuncs(rec, "operator->")
		if len(cands) == 0 {
			return nil
		}
		object := &Argument{Type: types.RemoveReference(t), IsLValue: cat == LValue || types.IsLValueReference(t)}
		ndiags := len(a.diags)
		fn, err := a.resolveAmongOn(cands, object, nil, nil, m.Pos())
		if err != nil {
			a.diags = a.diags[:ndiags]
			return nil
		}
		a.ensureInstantiated(fn)
		chain = append(chain, fn)
		t, cat = fn.FuncType.Ret, PrValue
	}
	return nil
}

// checkTemplateIdExpr checks a template-id in expression position (variable template or concept).
func (a *Analyzer) checkTemplateIdExpr(e ast.Expr, tn *ast.TemplateName, syms []Symbol) ExprInfo {
	name := NameString(tn.Name, a.unit)
	for _, sym := range syms {
		switch s := sym.(type) {
		case *DependentSymbol:
			return dependentExpr()
		case *ConceptSymbol:
			// Concept-id evaluates to bool.
			args := templateArgs(tn, a.curScope, a.unit)
			if a.dependentContext() && argsDependent(args) {
				return dependentExpr()
			}
			v, err := a.NewConstContext().EvalBool(e)
			if err != nil {
				a.errorAt(e.Pos(), fmt.Sprintf("the concept-id %s cannot be decided: %v", name, err))
				return ExprInfo{Type: types.Typ(types.Bool), ValCat: PrValue}
			}
			n := int64(0)
			if v {
				n = 1
			}
			a.noteConst(e, n)
			return ExprInfo{Type: types.Typ(types.Bool), ValCat: PrValue, IsConst: true, ConstVal: n}
		case *VarSymbol:
			if s.Template == nil {
				continue
			}
			args := templateArgs(tn, a.curScope, a.unit)
			if args == nil || a.dependentContext() && argsDependent(args) {
				return dependentExpr()
			}
			inst := a.instantiateVar(s, args, tn.Pos())
			if inst == nil {
				return dependentExpr()
			}
			a.record(e, inst)
			return a.foldConstant(e, a.symToExprInfo(inst), inst)
		case *FuncSymbol:
			return a.symToExprInfo(s)
		}
	}
	if len(syms) == 0 {
		a.errorAt(e.Pos(), fmt.Sprintf("use of undeclared identifier %q", name))
	} else {
		a.errorAt(e.Pos(), fmt.Sprintf("%q is not a variable template", name))
	}
	return ExprInfo{Type: types.Typ(types.Int), ValCat: LValue}
}

// argsDependent reports whether any argument still depends on a template parameter.
func argsDependent(args []types.TemplateArg) bool {
	for _, arg := range args {
		if arg.IsType && (arg.Type == nil || isDependentType(arg.Type)) {
			return true
		}
	}
	return false
}

// castResult is what a cast to t yields: the referenced object as an
// lvalue or xvalue when t is a reference, a prvalue of t otherwise.
func castResult(t types.Type) ExprInfo {
	switch r := t.(type) {
	case *types.LValueReference:
		return ExprInfo{Type: r.Elem, ValCat: LValue}
	case *types.RValueReference:
		return ExprInfo{Type: r.Elem, ValCat: XValue}
	}
	return ExprInfo{Type: t, ValCat: PrValue}
}

// foldConstant evaluates and records the value of a constant variable.
func (a *Analyzer) foldConstant(e ast.Expr, info ExprInfo, sym Symbol) ExprInfo {
	if es, isEnum := sym.(*EnumeratorSymbol); isEnum {
		a.noteConst(e, es.Val)
		return info
	}
	v, isVar := sym.(*VarSymbol)
	if !isVar {
		return info
	}
	// Non-type template parameter with known bound value.
	if v.HasKnownValue {
		a.noteConst(e, v.KnownValue)
		info.IsConst, info.ConstVal = true, v.KnownValue
		return info
	}
	if v.Init == nil || classOfType(v.SymType) != nil {
		return info
	}
	if !v.Constexpr && !(types.IsConst(v.SymType) && types.IsInteger(types.Unqualify(v.SymType))) {
		return info
	}
	if _, isRef := v.SymType.(*types.LValueReference); isRef {
		return info
	}
	if isDependentType(v.SymType) {
		return info
	}
	val, err := a.evalInScope(a.NewConstContext(), v.Init, v.SymScope)
	if err != nil {
		return info
	}
	switch val := val.(type) {
	case constexpr.IntValue:
		a.noteConst(e, val.Int64())
		info.IsConst, info.ConstVal = true, val.Int64()
	case constexpr.BoolValue:
		n := int64(0)
		if val.Val {
			n = 1
		}
		a.noteConst(e, n)
		info.IsConst, info.ConstVal = true, n
	}
	return info
}

// classOfType is the class a type is, or nil.
func classOfType(t types.Type) *types.Record {
	return types.AsRecord(types.Unqualify(t))
}

// conditionalType computes the result type of a conditional expression.
func conditionalType(thenInfo, elseInfo ExprInfo, e *ast.CondExpr) types.Type {
	t1, t2 := types.Decay(types.RemoveReference(thenInfo.Type)), types.Decay(types.RemoveReference(elseInfo.Type))
	_, p1 := types.Unqualify(t1).(*types.Pointer)
	_, p2 := types.Unqualify(t2).(*types.Pointer)
	switch {
	case p1 && p2:
		if types.Unqualify(t1).Equal(types.Unqualify(t2)) {
			return t1
		}
		if isNullConstant(e.Else, elseInfo) {
			return t1
		}
		if isNullConstant(e.Then, thenInfo) {
			return t2
		}
		return t1
	case p1 && (isNullConstant(e.Else, elseInfo) || types.Unqualify(t2).Kind() == types.NullptrKind):
		return t1
	case p2 && (isNullConstant(e.Then, thenInfo) || types.Unqualify(t1).Kind() == types.NullptrKind):
		return t2
	}
	if ct := types.CommonType(thenInfo.Type, elseInfo.Type); ct != nil {
		return ct
	}
	return thenInfo.Type
}

// builtinCall answers a call to one of the compiler's expression builtins.
func (a *Analyzer) builtinCall(name string, c *ast.CallExpr) (ExprInfo, bool) {
	if info, ok := a.gnuBuiltinCall(name, c); ok {
		return info, true
	}
	switch name {
	case "__assume", "__builtin_assume", "__builtin_unreachable", "__debugbreak", "__noop", "__fastfail":
		for _, arg := range c.Args {
			a.CheckExpr(arg)
		}
		return ExprInfo{Type: types.Typ(types.Void), ValCat: PrValue}, true
	case "__builtin_addressof":
		if len(c.Args) != 1 {
			a.errorAt(c.Pos(), "__builtin_addressof takes one argument")
			return ExprInfo{Type: types.Typ(types.Void), ValCat: PrValue}, true
		}
		info := a.CheckExpr(c.Args[0])
		if isDependentExpr(info) {
			return dependentExpr(), true
		}
		if info.ValCat != LValue {
			a.errorAt(c.Args[0].Pos(), "__builtin_addressof needs an lvalue")
		}
		return ExprInfo{Type: &types.Pointer{Elem: types.RemoveReference(info.Type)}, ValCat: PrValue}, true
	}
	return ExprInfo{}, false
}

// resolveAmong performs overload resolution with template argument deduction.
func (a *Analyzer) resolveAmong(candidates []*FuncSymbol, explicit []types.Type, args []Argument, at ast.Tok) (*FuncSymbol, error) {
	return a.resolveAmongOn(candidates, nil, explicit, args, at)
}

// resolveAmongOn performs overload resolution including member function candidates.
func (a *Analyzer) resolveAmongOn(candidates []*FuncSymbol, object *Argument, explicit []types.Type, args []Argument, at ast.Tok) (*FuncSymbol, error) {
	instanceArgs := make(map[*FuncSymbol][]types.TemplateArg)
	templateOf := make(map[*FuncSymbol]*FuncSymbol)
	viable := candidates[:0:0]
	var whyNot []string
	for _, cand := range candidates {
		var paramNames []string
		if cand.Template != nil {
			for _, p := range cand.Template.Params {
				paramNames = append(paramNames, p.SymName)
			}
		}
		sig, b, ok := specializeNamed(cand.FuncType, paramNames, explicit, args)
		if !ok {
			continue
		}
		if cand.Template == nil {
			if sig != cand.FuncType {
				sub := *cand
				sub.FuncType = sig
				templateOf[&sub] = cand
				viable = append(viable, &sub)
				continue
			}
			viable = append(viable, cand)
			continue
		}
		if a.dependentContext() {
			sub := *cand
			sub.FuncType = sig
			templateOf[&sub] = cand
			viable = append(viable, &sub)
			continue
		}
		if b == nil {
			b = Binding{}
		}
		if !arityAdmits(cand.FuncType, len(args)) {
			continue
		}
		ndiags := len(a.diags)
		targs, err := a.templateArgsFor(cand, b)
		if err != nil {
			a.diags = a.diags[:ndiags]
			whyNot = append(whyNot, err.Error())
			continue
		}
		realSig, ok := a.instanceSignature(cand, targs)
		if !ok || len(a.diags) > ndiags {
			a.diags = a.diags[:ndiags]
			whyNot = append(whyNot, fmt.Sprintf("%s: the signature does not substitute", cand.SymName))
			continue
		}
		sub := *cand
		sub.FuncType = realSig
		instanceArgs[&sub] = targs
		templateOf[&sub] = cand
		viable = append(viable, &sub)
	}
	if len(viable) == 0 {
		if len(whyNot) > 0 {
			return nil, fmt.Errorf("no candidate matches: %s", strings.Join(whyNot, "; "))
		}
		return nil, fmt.Errorf("no candidate matches: template argument deduction failed for every one")
	}

	// Constraint checking and ordering.
	viable = a.satisfiedOnly(viable, instanceArgs, templateOf)
	if len(viable) == 0 {
		return nil, fmt.Errorf("no candidate matches: every one's constraints are unsatisfied")
	}
	resolved, err := ResolveOverloadOn(viable, object, args)
	if err != nil && len(viable) > 1 {
		if best := a.moreConstrained(viable, object, args); best != nil {
			resolved, err = best, nil
		}
	}
	if err != nil {
		return nil, err
	}
	if tmpl := templateOf[resolved]; tmpl != nil {
		if targs, has := instanceArgs[resolved]; has {
			if inst := a.instantiateWithArgs(tmpl, targs, at); inst != nil {
				a.ensureInstantiated(inst)
				return inst, nil
			}
			return nil, fmt.Errorf("%s<%s> could not be instantiated", tmpl.SymName, argsKey(targs))
		}
		return tmpl, nil
	}
	a.ensureInstantiated(resolved)
	return resolved, nil
}

// memberFuncs returns candidate member function symbols matching the given name.
func (a *Analyzer) memberFuncs(rec *types.Record, name string) []*FuncSymbol {
	var out []*FuncSymbol
	if name == rec.Name {
		// Inherited constructors exclude copy and move constructors.
		for _, base := range rec.InheritedCtors {
			for _, c := range a.memberFuncs(base, base.Name) {
				if len(c.FuncType.Params) == 1 && isReferenceTo(c.FuncType.Params[0].Type, base) {
					continue
				}
				out = append(out, c)
			}
		}
	}
	for _, m := range rec.Methods {
		if m.Name != name {
			continue
		}
		if reg := a.methodSyms[m]; reg != nil {
			if reg.TemplateOf != nil {
				continue
			}
			out = append(out, reg)
			continue
		}
		out = append(out, &FuncSymbol{
			SymName: m.Name, FuncType: m.Func, InClass: rec, Method: m,
			Explicit: m.Explicit, Defaulted: m.Defaulted, Deleted: m.Deleted,
			Static: m.Static, Virtual: m.Virtual, Access: m.Access,
		})
	}
	return out
}

// isReferenceTo is whether t is a reference to rec, cv-qualified or not.
func isReferenceTo(t types.Type, rec *types.Record) bool {
	if !types.IsReference(t) {
		return false
	}
	return types.AsRecord(types.Unqualify(types.RemoveReference(t))) == rec
}

// explicitTemplateArgs builds explicit template arguments for a call.
func (a *Analyzer) explicitTemplateArgs(tn *ast.TemplateName) ([]types.Type, bool) {
	var explicit []types.Type
	for _, argNode := range tn.Args {
		if typeId, isType := argNode.(*ast.TypeId); isType {
			if name := bareTypeName(typeId); name != nil {
				if e, isExpr := name.(ast.Expr); isExpr && namesAValue(lookupName(name, a.curScope, a.unit)) {
					if n, err := a.NewConstContext().EvalInt(e); err == nil {
						explicit = append(explicit, &valueBound{Type: types.Typ(types.Int), Val: n})
						continue
					}
				}
			}
			info := BuildDeclSpecs(typeId.Specs, a.curScope, a.unit)
			explicit = append(explicit, BuildDeclarator(typeId.Decl, info.Type, a.curScope, a.unit))
			continue
		}
		if expr, isExpr := argNode.(ast.Expr); isExpr {
			if n, err := a.NewConstContext().EvalInt(expr); err == nil {
				explicit = append(explicit, &valueBound{Type: types.Typ(types.Int), Val: n})
				continue
			}
		}
		explicit = append(explicit, nil)
	}
	if a.dependentContext() {
		for _, t := range explicit {
			if t == nil || isDependentType(t) {
				return explicit, true
			}
		}
	}
	return explicit, false
}

// hasDependentBase reports whether rec has a base class that depends on a template parameter.
func hasDependentBase(rec *types.Record) bool {
	for _, b := range rec.Bases {
		if isDependentType(b.Type) {
			return true
		}
		if br := types.AsRecord(types.Unqualify(b.Type)); br != nil && hasDependentBase(br) {
			return true
		}
	}
	return false
}

// satisfiedOnly filters candidates by evaluating their constraint clauses.
func (a *Analyzer) satisfiedOnly(cands []*FuncSymbol, instanceArgs map[*FuncSymbol][]types.TemplateArg, templateOf map[*FuncSymbol]*FuncSymbol) []*FuncSymbol {
	out := cands[:0:0]
	for _, c := range cands {
		if a.satisfied(c, instanceArgs[c], templateOf[c]) {
			out = append(out, c)
		}
	}
	return out
}

func (a *Analyzer) satisfied(c *FuncSymbol, targs []types.TemplateArg, tmpl *FuncSymbol) bool {
	if len(c.Constraints) == 0 {
		return true
	}
	scope := c.ConstraintScope
	if tmpl != nil && tmpl.Template != nil && targs != nil {
		scope = a.bindTemplateArgs(tmpl.Template.Scope, tmpl.Template.Params, targs)
	}
	if scope == nil {
		return true
	}
	for _, e := range c.Constraints {
		ndiags := len(a.diags)
		ok, err := a.evalInScope(a.NewConstContext(), e, scope)
		if len(a.diags) > ndiags {
			a.diags = a.diags[:ndiags]
			return false
		}
		if err != nil {
			if a.dependentContext() {
				continue
			}
			return false
		}
		if !ok.ToBool() {
			return false
		}
	}
	return true
}

// moreConstrained breaks ties by preferring constrained candidates over unconstrained.
func (a *Analyzer) moreConstrained(viable []*FuncSymbol, object *Argument, args []Argument) *FuncSymbol {
	var constrained []*FuncSymbol
	for _, c := range viable {
		if len(c.Constraints) > 0 {
			constrained = append(constrained, c)
		}
	}
	if len(constrained) == 0 || len(constrained) == len(viable) {
		return nil
	}
	if resolved, err := ResolveOverloadOn(constrained, object, args); err == nil {
		return resolved
	}
	return nil
}

// expandPackArgs expands pack expansion expressions in an argument list.
func (a *Analyzer) expandPackArgs(args []ast.Expr) []ast.Expr {
	expanded := false
	for _, arg := range args {
		if _, isPack := arg.(*ast.PackExpansion); isPack {
			expanded = true
		}
	}
	if !expanded {
		return args
	}
	var out []ast.Expr
	for _, arg := range args {
		pe, isPack := arg.(*ast.PackExpansion)
		if !isPack {
			out = append(out, arg)
			continue
		}
		elems, ok := a.expandOne(pe)
		if !ok {
			out = append(out, arg)
			continue
		}
		out = append(out, elems...)
	}
	return out
}

// expandOne expands a single pack expansion expression into its constituent elements.
func (a *Analyzer) expandOne(pe *ast.PackExpansion) ([]ast.Expr, bool) {
	packs := a.packsIn(pe.X)
	valuePacks := a.valuePacksIn(pe.X)
	n := -1
	for _, p := range packs {
		if n >= 0 && n != len(p.pack.Elems) {
			a.errorAt(pe.Pos(), "the packs in this expansion differ in length")
			return nil, false
		}
		n = len(p.pack.Elems)
	}
	for _, p := range valuePacks {
		if p.Open {
			return nil, false // the template as written: nothing to expand yet
		}
		if n >= 0 && n != len(p.Elems) {
			a.errorAt(pe.Pos(), "the packs in this expansion differ in length")
			return nil, false
		}
		n = len(p.Elems)
	}
	if n < 0 {
		return nil, false
	}
	if a.prechecked == nil {
		a.prechecked = map[ast.Expr]ExprInfo{}
	}
	var out []ast.Expr
	for i := 0; i < n; i++ {
		bound := NewScope(a.curScope, BlockScope, nil)
		for _, p := range packs {
			elem := p.pack.Elems[i]
			if elem.IsType {
				bound.Insert(&TypeSymbol{SymName: p.name, SymType: elem.Type, SymScope: bound})
			} else {
				vt := elem.ValType
				if vt == nil {
					vt = types.Typ(types.Int)
				}
				bound.Insert(&VarSymbol{SymName: p.name, SymType: vt, SymScope: bound, Constexpr: true, KnownValue: elem.Val, HasKnownValue: true})
			}
		}
		for _, p := range valuePacks {
			bound.Insert(p.Elems[i])
		}
		copy := ast.Clone(pe.X)
		saved := a.curScope
		a.curScope = bound
		info := a.CheckExpr(copy)
		if !info.IsConst {
			// The pattern may still be a constant expression over the
			// bound values -- `(Vs + S)` as a template argument.
			if n, err := a.NewConstContext().EvalInt(copy); err == nil {
				info.ConstVal, info.IsConst = n, true
			}
		}
		a.curScope = saved
		a.prechecked[copy] = info
		out = append(out, copy)
	}
	return out, true
}

// valuePacksIn finds the function parameter packs an expression names.
func (a *Analyzer) valuePacksIn(e ast.Expr) []*PackSymbol {
	var packs []*PackSymbol
	seen := map[string]bool{}
	ast.Inspect(e, func(n ast.Node) bool {
		id, isIdent := n.(*ast.Ident)
		if !isIdent {
			return true
		}
		name := id.Text(a.unit)
		if seen[name] {
			return true
		}
		for _, sym := range LookupUnqualified(a.curScope, name) {
			if ps, isPack := sym.(*PackSymbol); isPack {
				seen[name] = true
				packs = append(packs, ps)
			}
			break
		}
		return true
	})
	return packs
}

// packLength is sizeof...(name): the elements of the type or function
// parameter pack the name means here, if it is bound.
func (a *Analyzer) packLength(e ast.Expr) (int64, bool) {
	id, isIdent := e.(*ast.Ident)
	if !isIdent {
		if p, isParen := e.(*ast.ParenExpr); isParen {
			return a.packLength(p.X)
		}
		return 0, false
	}
	for _, sym := range LookupUnqualified(a.curScope, id.Text(a.unit)) {
		switch s := sym.(type) {
		case *PackSymbol:
			if s.Open {
				return 0, false
			}
			return int64(len(s.Elems)), true
		case *TypeSymbol:
			if pack, isPack := s.SymType.(*types.Pack); isPack {
				return int64(len(pack.Elems)), true
			}
		}
		break
	}
	return 0, false
}

// resolveOperator resolves overloaded operators.
func (a *Analyzer) resolveOperator(e ast.Expr, opName string, operands []ExprInfo, at ast.Tok) (ExprInfo, bool) {
	if a.dependentContext() {
		for _, o := range operands {
			if isDependentExpr(o) {
				return dependentExpr(), true
			}
		}
	}
	chosen, info, ok := a.chooseOperator(opName, operands, at)
	if !ok {
		return info, ok
	}
	if chosen == nil {
		return info, true
	}
	if a.info != nil && (!chosen.Defaulted || isDefaultedComparison(chosen)) {
		a.info.Operators[e] = chosen
	}
	if exprs := operandExprs(e); len(exprs) == len(operands) {
		args := make([]Argument, len(operands))
		for i, o := range operands {
			args[i] = Argument{Type: o.Type, IsLValue: o.ValCat == LValue}
		}
		if chosen.InClass != nil && !chosen.Static && !chosen.Friend {
			// A member's parameters start after the object.
			a.noteArgConversions(chosen, exprs[1:], args[1:], at)
		} else {
			a.noteArgConversions(chosen, exprs, args, at)
		}
	}
	return info, true
}

// chooseOperator finds the operator function for operands of these types without recording it.
func (a *Analyzer) chooseOperator(opName string, operands []ExprInfo, at ast.Tok) (*FuncSymbol, ExprInfo, bool) {
	args := make([]Argument, len(operands))
	for i, o := range operands {
		args[i] = Argument{Type: o.Type, IsLValue: o.ValCat == LValue}
	}
	left := operands[0]
	lhsRec := types.AsRecord(types.Unqualify(types.RemoveReference(left.Type)))

	// Members, called on the left operand.
	var member *FuncSymbol
	if lhsRec != nil {
		if cands := a.memberFuncs(lhsRec, opName); len(cands) > 0 {
			object := &Argument{Type: left.Type, IsLValue: left.ValCat == LValue}
			member, _ = a.resolveAmongOn(cands, object, nil, args[1:], at)
		}
	}
	// Non-members: unqualified lookup, and ADL in operand namespaces.
	var free []*FuncSymbol
	seen := map[*FuncSymbol]bool{}
	add := func(syms []Symbol) {
		for _, sym := range syms {
			if fn, isFn := sym.(*FuncSymbol); isFn && fn.InClass == nil && !seen[fn] {
				seen[fn] = true
				free = append(free, fn)
			}
		}
	}
	add(LookupUnqualified(a.curScope, opName))
	for _, o := range operands {
		if rec := types.AsRecord(types.Unqualify(types.RemoveReference(o.Type))); rec != nil {
			if rs := a.curScope.recordSymbol(rec); rs != nil && rs.SymScope != nil {
				ns := rs.SymScope
				for ns != nil && ns.Kind != NamespaceScope && ns.Kind != GlobalScope {
					ns = ns.Parent
				}
				if ns != nil {
					add(ns.LookupAssociated(opName))
				}
			}
		}
	}
	var nonMember *FuncSymbol
	if len(free) > 0 {
		nonMember, _ = a.resolveAmong(free, nil, args, at)
	}

	var chosen *FuncSymbol
	switch {
	case member != nil && nonMember != nil:
		asFree := *member
		params := append([]types.Param{{Type: types.AddLValueReference(types.Qualify(member.InClass, member.FuncType.Quals))}}, member.FuncType.Params...)
		asFree.FuncType = &types.Func{Ret: member.FuncType.Ret, Params: params, Noexcept: member.FuncType.Noexcept}
		best, err := ResolveOverload([]*FuncSymbol{&asFree, nonMember}, args)
		if err != nil {
			a.errorAt(at, fmt.Sprintf("%s is ambiguous between a member and a non-member", opName))
			return nil, ExprInfo{Type: types.Typ(types.Int), ValCat: PrValue}, true
		}
		if best == &asFree {
			chosen = member
		} else {
			chosen = nonMember
		}
	case member != nil:
		chosen = member
	case nonMember != nil:
		chosen = nonMember
	default:
		return nil, ExprInfo{}, false
	}
	a.ensureInstantiated(chosen)
	ret := chosen.FuncType.Ret
	cat := PrValue
	if types.IsLValueReference(ret) {
		cat = LValue
	} else if types.IsRValueReference(ret) {
		cat = XValue
	}
	return chosen, ExprInfo{Type: types.RemoveReference(ret), ValCat: cat}, true
}

// rewrittenEquality synthesizes rewritten candidates for == and !=.
func (a *Analyzer) rewrittenEquality(b *ast.BinaryExpr, left, right ExprInfo) (ExprInfo, bool) {
	if b.Op != token.EQL && b.Op != token.NEQ {
		return ExprInfo{}, false
	}
	boolean := ExprInfo{Type: types.Typ(types.Bool), ValCat: PrValue}
	try := func(x, y ast.Expr, xi, yi ExprInfo) ast.Expr {
		eq := &ast.BinaryExpr{Span: b.Span, X: x, OpPos: b.OpPos, Op: token.EQL, Y: y}
		ndiags := len(a.diags)
		if _, resolved := a.resolveOperator(eq, "operator==", []ExprInfo{xi, yi}, b.Pos()); !resolved || len(a.diags) > ndiags {
			a.diags = a.diags[:ndiags]
			return nil
		}
		if a.info != nil {
			a.info.Types[eq] = boolean.Type
		}
		return eq
	}
	var eq ast.Expr
	if b.Op == token.NEQ {
		eq = try(b.X, b.Y, left, right)
	}
	if eq == nil {
		// Reversed operands.
		eq = try(b.Y, b.X, right, left)
	}
	if eq == nil {
		return ExprInfo{}, false
	}
	var rewritten ast.Expr = eq
	if b.Op == token.NEQ {
		not := &ast.UnaryExpr{Span: b.Span, OpPos: b.OpPos, Op: token.NOT, X: eq}
		if a.info != nil {
			a.info.Types[not] = boolean.Type
		}
		rewritten = not
	}
	if a.info != nil {
		a.info.Rewrites[b] = rewritten
	}
	return boolean, true
}

// addAssociated adds functions found via ADL in the arguments' associated namespaces.
func (a *Analyzer) addAssociated(candidates []*FuncSymbol, name string, args []Argument) []*FuncSymbol {
	seen := map[*FuncSymbol]bool{}
	for _, c := range candidates {
		seen[c] = true
	}
	visited := map[*types.Record]bool{}
	var visit func(t types.Type)
	visit = func(t types.Type) {
		t = types.Unqualify(types.RemoveReference(t))
		if ptr, isPtr := t.(*types.Pointer); isPtr {
			visit(ptr.Elem)
			return
		}
		rec := types.AsRecord(t)
		if rec == nil || visited[rec] {
			return
		}
		visited[rec] = true
		if rs := a.curScope.recordSymbol(rec); rs != nil && rs.SymScope != nil {
			ns := rs.SymScope
			for ns != nil && ns.Kind != NamespaceScope && ns.Kind != GlobalScope {
				ns = ns.Parent
			}
			if ns != nil {
				for _, sym := range ns.LookupAssociated(name) {
					if fn, isFn := sym.(*FuncSymbol); isFn && fn.InClass == nil && !seen[fn] {
						seen[fn] = true
						candidates = append(candidates, fn)
					}
				}
			}
		}
		for _, b := range rec.Bases {
			visit(b.Type)
		}
	}
	for _, arg := range args {
		visit(arg.Type)
	}
	return candidates
}

// operandExprs is the operand expressions of an operator expression, in
// the order resolveOperator was given them.
func operandExprs(e ast.Expr) []ast.Expr {
	switch x := e.(type) {
	case *ast.BinaryExpr:
		return []ast.Expr{x.X, x.Y}
	case *ast.AssignExpr:
		return []ast.Expr{x.Lhs, x.Rhs}
	case *ast.UnaryExpr:
		return []ast.Expr{x.X}
	case *ast.IndexExpr:
		if len(x.Args) == 1 {
			return []ast.Expr{x.X, x.Args[0]}
		}
	}
	return nil
}

// noteArgConversions records converting constructors for arguments converted to class parameters.
func (a *Analyzer) noteArgConversions(fn *FuncSymbol, exprs []ast.Expr, args []Argument, at ast.Tok) {
	if a.info == nil || fn == nil || fn.FuncType == nil || a.dependentContext() {
		return
	}
	for i, e := range exprs {
		if i >= len(args) || i >= len(fn.FuncType.Params) {
			break
		}
		want := types.RemoveReference(fn.FuncType.Params[i].Type)
		rec := types.AsRecord(types.Unqualify(want))
		if rec == nil {
			a.noteConversionFunction(e, args[i].Type, want)
			continue
		}
		have := types.Unqualify(types.RemoveReference(args[i].Type))
		if haveRec := types.AsRecord(have); haveRec != nil && (haveRec == rec || types.IsBaseOf(rec, haveRec)) {
			continue
		}
		if isDependentType(args[i].Type) {
			continue
		}
		if ctor := a.convertingConstructor(rec, args[i], at); ctor != nil {
			a.ensureInstantiated(ctor)
			a.info.Conversions[e] = ctor
		}
	}
}

// noteConversionFunction records user-defined conversion functions to non-class targets.
func (a *Analyzer) noteConversionFunction(e ast.Expr, from types.Type, to types.Type) {
	if a.info == nil || e == nil || to == nil {
		return
	}
	rec := types.AsRecord(types.Unqualify(types.RemoveReference(from)))
	if rec == nil || types.AsRecord(types.Unqualify(types.RemoveReference(to))) != nil {
		return
	}
	target := types.Unqualify(types.RemoveReference(to))
	for _, m := range rec.Methods {
		if !strings.HasPrefix(m.Name, "operator ") || len(m.Func.Params) != 0 || m.Func.Ret == nil {
			continue
		}
		if !types.Unqualify(m.Func.Ret).Equal(target) && !ClassifyConversion(m.Func.Ret, target, false).Valid {
			continue
		}
		if fn := a.methodSyms[m]; fn != nil {
			a.info.Conversions[e] = fn
			return
		}
		// A closure's conversion has no declared symbol: the invoker it
		// names is enough for lowering.
		for _, li := range a.info.Lambdas {
			if li.Closure == rec && li.Invoker != nil && li.Invoker.SymName == m.Name {
				a.info.Conversions[e] = li.Invoker
				return
			}
		}
	}
}

// assignSpelling returns the operator function name for an assignment token.
func assignSpelling(op token.Kind) string {
	switch op {
	case token.ASSIGN:
		return "operator="
	case token.ADD_ASSIGN:
		return "operator+="
	case token.SUB_ASSIGN:
		return "operator-="
	case token.MUL_ASSIGN:
		return "operator*="
	case token.QUO_ASSIGN:
		return "operator/="
	case token.REM_ASSIGN:
		return "operator%="
	case token.AND_ASSIGN:
		return "operator&="
	case token.OR_ASSIGN:
		return "operator|="
	case token.XOR_ASSIGN:
		return "operator^="
	case token.SHL_ASSIGN:
		return "operator<<="
	case token.SHR_ASSIGN:
		return "operator>>="
	}
	return "operator="
}

// arityAdmits reports whether ft can accept n arguments.
func arityAdmits(ft *types.Func, n int) bool {
	params := ft.Params
	hasPack := len(params) > 0 && params[len(params)-1].Pack
	if n > len(params) && !ft.Variadic && !hasPack {
		return false
	}
	required := 0
	for _, p := range params {
		if !p.HasDefault && !p.Pack {
			required++
		}
	}
	return n >= required
}
