package constexpr

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/literal"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// FlowSignal indicates control flow transfers inside constexpr statement blocks.
type FlowSignal int

const (
	FlowNone FlowSignal = iota
	FlowReturn
	FlowBreak
	FlowContinue
)

// FuncInfo describes a callable function in constexpr evaluation.
type FuncInfo struct {
	Unit       ast.Unit
	Name       string
	ParamNames []string
	ParamTypes []types.Type
	Body       *ast.CompoundStmt
	RetType    types.Type
}

// Scope represents a local lexical scope holding objects.
type Scope struct {
	Parent *Scope
	Vars   map[string]*Object
}

func NewScope(parent *Scope) *Scope {
	return &Scope{
		Parent: parent,
		Vars:   make(map[string]*Object),
	}
}

func (s *Scope) Lookup(name string) *Object {
	for cur := s; cur != nil; cur = cur.Parent {
		if obj, ok := cur.Vars[name]; ok {
			return obj
		}
	}
	return nil
}

// Context is the execution environment of the compile-time constexpr virtual machine.
type Context struct {
	Unit        ast.Unit
	Model       types.Model
	CurScope    *Scope
	Globals     map[string]*Object
	Funcs       map[string]*FuncInfo
	ResolveVar  func(name string) (Value, error)
	ResolveFunc func(name string) (*FuncInfo, error)

	// ResolveCall resolves a call expression to its target function.
	ResolveCall func(*ast.CallExpr) (*FuncInfo, bool)

	// ResolveType converts an AST type-id into a types.Type (for sizeof/alignof).
	ResolveType func(*ast.TypeId) (types.Type, bool)

	// ResolveTypeExpr resolves an expression naming a type (e.g. for functional casts T(x)).
	ResolveTypeExpr func(ast.Expr) (types.Type, bool)

	// ResolveQualified resolves a qualified name (e.g. Scoped::X).
	ResolveQualified func(*ast.QualifiedName) (Value, error)

	// ResolveTrait resolves built-in type traits (e.g. is_constructible).
	ResolveTrait func(name string, args []types.Type) (answer bool, known bool, err error)

	// ExpandFold expands and evaluates fold expressions.
	ExpandFold func(*ast.FoldExpr) (Value, error)

	// Folded returns sema-evaluated values for expressions like sizeof...(pack).
	Folded func(ast.Expr) (int64, bool)

	// TypeOfExpr returns the semantic type for unevaluated operands (e.g. sizeof).
	TypeOfExpr func(ast.Expr) types.Type

	// ResolveConcept evaluates a concept-id constraint.
	ResolveConcept func(*ast.TemplateName) (Value, bool, error)

	// ResolveRequires decides whether each requirement in a requires-expression is satisfied.
	ResolveRequires func(*ast.RequiresExpr) (bool, error)
	CallDepth       int
	MaxCallDepth    int
	StepCount       int
	MaxSteps        int
	objSeq          int
}

// NewContext creates an evaluation context.
func NewContext(unit ast.Unit, model types.Model) *Context {
	if model.SizeInt == 0 {
		model = types.LP64()
	}
	ctx := &Context{
		Unit:         unit,
		Model:        model,
		CurScope:     NewScope(nil),
		Globals:      make(map[string]*Object),
		Funcs:        make(map[string]*FuncInfo),
		MaxCallDepth: 512,
		MaxSteps:     1000000,
	}
	return ctx
}

func (ctx *Context) nextID() int {
	ctx.objSeq++
	return ctx.objSeq
}

func (ctx *Context) PushScope() {
	ctx.CurScope = NewScope(ctx.CurScope)
}

func (ctx *Context) PopScope() {
	if ctx.CurScope.Parent != nil {
		// Mark local objects as dead
		for _, obj := range ctx.CurScope.Vars {
			obj.IsAlive = false
		}
		ctx.CurScope = ctx.CurScope.Parent
	}
}

func (ctx *Context) DeclareVar(name string, typ types.Type, val Value, isConst bool) *Object {
	obj := NewObject(ctx.nextID(), name, typ, val, isConst)
	ctx.CurScope.Vars[name] = obj
	return obj
}

func (ctx *Context) DeclareGlobal(name string, typ types.Type, val Value, isConst bool) *Object {
	obj := NewObject(ctx.nextID(), name, typ, val, isConst)
	ctx.Globals[name] = obj
	return obj
}

func (ctx *Context) RegisterFunc(fn *FuncInfo) {
	ctx.Funcs[fn.Name] = fn
}

// Eval evaluates any AST expression to a compile-time Value.
func (ctx *Context) Eval(expr ast.Expr) (Value, error) {
	if expr == nil {
		return VoidValue{}, nil
	}
	ctx.StepCount++
	if ctx.StepCount > ctx.MaxSteps {
		return nil, fmt.Errorf("constexpr evaluation exceeded maximum step limit (%d)", ctx.MaxSteps)
	}

	switch e := expr.(type) {
	case *ast.ParenExpr:
		return ctx.Eval(e.X)

	case *ast.BasicLit:
		return ctx.evalBasicLit(e)

	case *ast.StringLit:
		return ctx.evalStringLit(e)

	case *ast.Ident:
		return ctx.evalIdent(e)

	case *ast.UnaryExpr:
		return ctx.evalUnary(e)

	case *ast.BinaryExpr:
		return ctx.evalBinary(e)

	case *ast.CondExpr:
		return ctx.evalCond(e)

	case *ast.AssignExpr:
		return ctx.evalAssign(e)

	case *ast.IncDecExpr:
		return ctx.evalPostfixIncDec(e)

	case *ast.IndexExpr:
		return ctx.evalIndex(e)

	case *ast.MemberExpr:
		return ctx.evalMember(e)

	case *ast.CallExpr:
		return ctx.evalCall(e)

	case *ast.CastExpr:
		v, err := ctx.Eval(e.X)
		if err != nil {
			return nil, err
		}
		return ctx.convertTo(v, e.Type), nil

	case *ast.NamedCastExpr:
		return ctx.evalNamedCast(e)

	case *ast.FunctionalCastExpr:
		return ctx.evalFunctionalCast(e)

	case *ast.SizeofExpr:
		return ctx.evalSizeof(e)

	case *ast.AlignofExpr:
		return ctx.evalAlignof(e)

	case *ast.InitList:
		return ctx.evalInitList(e)

	case *ast.TypeTraitExpr:
		return ctx.evalTypeTrait(e)

	case *ast.QualifiedName:
		if ctx.ResolveQualified != nil {
			return ctx.ResolveQualified(e)
		}
		return nil, fmt.Errorf("%w: qualified name in a constant expression", ErrNonConstexpr)

	case *ast.FoldExpr:
		if ctx.ExpandFold != nil {
			return ctx.ExpandFold(e)
		}
		return nil, fmt.Errorf("%w: a fold expression needs the analysis to expand its pack", ErrNonConstexpr)

	case *ast.TemplateName:
		// The only template-id that is a value rather than a type is a
		// concept-id. Anything else named with arguments here is a class or
		// a function template, and neither has a value of its own.
		if ctx.ResolveConcept != nil {
			if v, ok, err := ctx.ResolveConcept(e); err != nil {
				return nil, err
			} else if ok {
				return v, nil
			}
		}
		return nil, fmt.Errorf("%w: a template-id is a value only when it is a concept-id", ErrNonConstexpr)

	case *ast.RequiresExpr:
		if ctx.ResolveRequires != nil {
			ok, err := ctx.ResolveRequires(e)
			if err != nil {
				return nil, err
			}
			return BoolValue{Val: ok}, nil
		}
		return nil, fmt.Errorf("%w: a requires-expression needs the analysis to form its requirements", ErrNonConstexpr)

	default:
		return nil, fmt.Errorf("%w: unsupported expression node %T", ErrNonConstexpr, expr)
	}
}

// EvalInt evaluates an expression and returns an int64 value.
func (ctx *Context) EvalInt(expr ast.Expr) (int64, error) {
	v, err := ctx.Eval(expr)
	if err != nil {
		return 0, err
	}
	if iv, ok := v.(IntValue); ok {
		return iv.Int64(), nil
	}
	if bv, ok := v.(BoolValue); ok {
		if bv.Val {
			return 1, nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("%w: expected integer constant, got %s", ErrNonConstexpr, v.Kind())
}

// EvalBool evaluates an expression and returns a boolean value.
func (ctx *Context) EvalBool(expr ast.Expr) (bool, error) {
	v, err := ctx.Eval(expr)
	if err != nil {
		return false, err
	}
	return v.ToBool(), nil
}

func (ctx *Context) evalBasicLit(lit *ast.BasicLit) (Value, error) {
	switch lit.Kind {
	case token.TRUE:
		return NewBool(true), nil
	case token.FALSE:
		return NewBool(false), nil
	case token.NULLPTR:
		return NullptrValue{}, nil
	case token.INT_LIT:
		return parseIntegerLiteral(lit.Spelling(ctx.Unit), ctx.Model)
	case token.FLOAT_LIT:
		return parseFloatLiteral(lit.Spelling(ctx.Unit))
	case token.CHAR_LIT:
		return parseCharLiteral(lit.Spelling(ctx.Unit), ctx.Model)
	default:
		return nil, fmt.Errorf("%w: unrecognized literal kind %s", ErrNonConstexpr, lit.Kind)
	}
}

func parseIntegerLiteral(text string, model types.Model) (Value, error) {
	// Strip digit separators '
	clean := strings.ReplaceAll(text, "'", "")
	lower := strings.ToLower(clean)

	// Determine suffixes
	isUnsigned := false
	isLong := 0
	for strings.HasSuffix(lower, "u") || strings.HasSuffix(lower, "l") || strings.HasSuffix(lower, "z") {
		if strings.HasSuffix(lower, "u") {
			isUnsigned = true
			lower = lower[:len(lower)-1]
		} else if strings.HasSuffix(lower, "ll") {
			isLong = 2
			lower = lower[:len(lower)-2]
		} else if strings.HasSuffix(lower, "l") {
			if isLong < 1 {
				isLong = 1
			}
			lower = lower[:len(lower)-1]
		} else if strings.HasSuffix(lower, "z") {
			isUnsigned = true
			isLong = 1
			lower = lower[:len(lower)-1]
		}
	}

	// Base detection
	base := 10
	valStr := lower
	if strings.HasPrefix(lower, "0x") {
		base = 16
		valStr = lower[2:]
	} else if strings.HasPrefix(lower, "0b") {
		base = 2
		valStr = lower[2:]
	} else if strings.HasPrefix(lower, "0") && len(lower) > 1 {
		base = 8
		valStr = lower[1:]
	}

	bi := new(big.Int)
	_, ok := bi.SetString(valStr, base)
	if !ok {
		return nil, fmt.Errorf("malformed integer literal %q", text)
	}

	var typ types.Type
	if isUnsigned {
		switch isLong {
		case 2:
			typ = types.Typ(types.ULongLong)
		case 1:
			typ = types.Typ(types.ULong)
		default:
			typ = types.Typ(types.UInt)
		}
	} else {
		switch isLong {
		case 2:
			typ = types.Typ(types.LongLong)
		case 1:
			typ = types.Typ(types.Long)
		default:
			typ = types.Typ(types.Int)
		}
	}

	return NewBigInt(bi, typ, model), nil
}

func parseFloatLiteral(text string) (Value, error) {
	clean := strings.ReplaceAll(text, "'", "")
	clean = strings.TrimRight(clean, "fFlL")
	f, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return nil, fmt.Errorf("malformed float literal %q: %w", text, err)
	}
	return NewFloat(f, types.Typ(types.Double)), nil
}

func parseCharLiteral(text string, model types.Model) (Value, error) {
	// Simple char literal parsing
	s := strings.TrimPrefix(text, "u8")
	s = strings.TrimPrefix(s, "u")
	s = strings.TrimPrefix(s, "U")
	s = strings.TrimPrefix(s, "L")
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		content := s[1 : len(s)-1]
		var r rune
		if len(content) == 1 {
			r = rune(content[0])
		} else if strings.HasPrefix(content, "\\") {
			switch content {
			case `\n`:
				r = '\n'
			case `\t`:
				r = '\t'
			case `\r`:
				r = '\r'
			case `\0`:
				r = 0
			case `\\`:
				r = '\\'
			case `\'`:
				r = '\''
			case `\"`:
				r = '"'
			default:
				if strings.HasPrefix(content, `\x`) {
					n, _ := strconv.ParseInt(content[2:], 16, 32)
					r = rune(n)
				} else {
					r = rune(content[1])
				}
			}
		}
		return NewInt(int64(r), types.Typ(types.Char), model), nil
	}
	return NewInt(0, types.Typ(types.Char), model), nil
}

func (ctx *Context) evalStringLit(lit *ast.StringLit) (Value, error) {
	s, err := literal.Decode(ctx.Unit, lit)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNonConstexpr, err)
	}
	if s.Enc != literal.Narrow && s.Enc != literal.UTF8 {
		return nil, fmt.Errorf("%w: a %s string literal has no value in a constant expression yet", ErrNonConstexpr, s.Enc)
	}
	return NewString(string(s.Bytes())), nil
}

func (ctx *Context) evalIdent(id *ast.Ident) (Value, error) {
	name := id.Text(ctx.Unit)

	// Check local scopes first
	if obj := ctx.CurScope.Lookup(name); obj != nil {
		return obj.Read(nil, 0)
	}

	// Check globals
	if obj, ok := ctx.Globals[name]; ok {
		return obj.Read(nil, 0)
	}

	// Check resolver if provided
	if ctx.ResolveVar != nil {
		v, err := ctx.ResolveVar(name)
		if err == nil {
			return v, nil
		}
	}

	return nil, fmt.Errorf("%w: undeclared identifier %q in constant expression", ErrNonConstexpr, name)
}

func (ctx *Context) evalUnary(u *ast.UnaryExpr) (Value, error) {
	// Address-of operator '&'
	if u.Op == token.AND {
		obj, path, offset, err := ctx.EvalLValue(u.X)
		if err != nil {
			return nil, err
		}
		ptrTyp := &types.Pointer{Elem: obj.Type}
		return PointerValue{
			Target: obj,
			Path:   path,
			Offset: offset,
			Typ:    ptrTyp,
		}, nil
	}

	// Pointer dereference '*'
	if u.Op == token.MUL {
		val, err := ctx.Eval(u.X)
		if err != nil {
			return nil, err
		}
		ptr, ok := val.(PointerValue)
		if !ok {
			return nil, fmt.Errorf("%w: dereference of non-pointer value %s", ErrUB, val.Kind())
		}
		if ptr.IsNull() {
			return nil, ErrNullPointer
		}
		return ptr.Target.Read(ptr.Path, ptr.Offset)
	}

	val, err := ctx.Eval(u.X)
	if err != nil {
		return nil, err
	}

	switch u.Op {
	case token.ADD:
		return val, nil

	case token.SUB:
		switch v := val.(type) {
		case IntValue:
			res := new(big.Int).Neg(v.Val)
			return NewBigInt(res, v.Type(), ctx.Model), nil
		case FloatValue:
			return NewFloat(-v.Val, v.Type()), nil
		default:
			return nil, fmt.Errorf("%w: unary - not supported on %s", ErrUB, val.Kind())
		}

	case token.NOT:
		return NewBool(!val.ToBool()), nil

	case token.TILDE:
		if iv, ok := val.(IntValue); ok {
			res := new(big.Int).Not(iv.Val)
			return NewBigInt(res, iv.Type(), ctx.Model), nil
		}
		return nil, fmt.Errorf("%w: unary ~ not supported on %s", ErrUB, val.Kind())

	case token.INC: // Prefix ++
		obj, path, offset, err := ctx.EvalLValue(u.X)
		if err != nil {
			return nil, err
		}
		curVal, err := obj.Read(path, offset)
		if err != nil {
			return nil, err
		}
		newVal, err := ctx.evalBinaryOp(token.ADD, curVal, NewInt(1, curVal.Type(), ctx.Model))
		if err != nil {
			return nil, err
		}
		if err := obj.Write(path, offset, newVal); err != nil {
			return nil, err
		}
		return newVal, nil

	case token.DEC: // Prefix --
		obj, path, offset, err := ctx.EvalLValue(u.X)
		if err != nil {
			return nil, err
		}
		curVal, err := obj.Read(path, offset)
		if err != nil {
			return nil, err
		}
		newVal, err := ctx.evalBinaryOp(token.SUB, curVal, NewInt(1, curVal.Type(), ctx.Model))
		if err != nil {
			return nil, err
		}
		if err := obj.Write(path, offset, newVal); err != nil {
			return nil, err
		}
		return newVal, nil

	default:
		return nil, fmt.Errorf("%w: unsupported unary operator %s", ErrNonConstexpr, u.Op)
	}
}

func (ctx *Context) evalPostfixIncDec(e *ast.IncDecExpr) (Value, error) {
	obj, path, offset, err := ctx.EvalLValue(e.X)
	if err != nil {
		return nil, err
	}
	oldVal, err := obj.Read(path, offset)
	if err != nil {
		return nil, err
	}

	var op token.Kind
	if e.Op == token.INC {
		op = token.ADD
	} else {
		op = token.SUB
	}

	newVal, err := ctx.evalBinaryOp(op, oldVal, NewInt(1, oldVal.Type(), ctx.Model))
	if err != nil {
		return nil, err
	}
	if err := obj.Write(path, offset, newVal); err != nil {
		return nil, err
	}
	return oldVal, nil
}

func (ctx *Context) evalBinary(b *ast.BinaryExpr) (Value, error) {
	// Handle short-circuiting logical operators
	if b.Op == token.LAND {
		left, err := ctx.Eval(b.X)
		if err != nil {
			return nil, err
		}
		if !left.ToBool() {
			return NewBool(false), nil
		}
		right, err := ctx.Eval(b.Y)
		if err != nil {
			return nil, err
		}
		return NewBool(right.ToBool()), nil
	}

	if b.Op == token.LOR {
		left, err := ctx.Eval(b.X)
		if err != nil {
			return nil, err
		}
		if left.ToBool() {
			return NewBool(true), nil
		}
		right, err := ctx.Eval(b.Y)
		if err != nil {
			return nil, err
		}
		return NewBool(right.ToBool()), nil
	}

	if b.Op == token.COMMA {
		_, err := ctx.Eval(b.X)
		if err != nil {
			return nil, err
		}
		return ctx.Eval(b.Y)
	}

	left, err := ctx.Eval(b.X)
	if err != nil {
		return nil, err
	}
	right, err := ctx.Eval(b.Y)
	if err != nil {
		return nil, err
	}

	return ctx.evalBinaryOp(b.Op, left, right)
}

// Combine applies a binary operator to two values, the way an expression
// with that operator would; the logical operators and the comma are
// included, without the short-circuit an expression has.
func (ctx *Context) Combine(op token.Kind, left, right Value) (Value, error) {
	switch op {
	case token.LAND:
		return NewBool(left.ToBool() && right.ToBool()), nil
	case token.LOR:
		return NewBool(left.ToBool() || right.ToBool()), nil
	case token.COMMA:
		return right, nil
	}
	return ctx.evalBinaryOp(op, left, right)
}

func (ctx *Context) evalBinaryOp(op token.Kind, left, right Value) (Value, error) {
	// Promote boolean operands to integer for non-logical operations
	if op != token.LAND && op != token.LOR {
		_, lIsBool := left.(BoolValue)
		_, rIsBool := right.(BoolValue)
		if lIsBool && (op != token.EQL && op != token.NEQ || !rIsBool) {
			n := int64(0)
			if left.(BoolValue).Val {
				n = 1
			}
			left = NewInt(n, types.Typ(types.Int), ctx.Model)
		}
		if rIsBool && (op != token.EQL && op != token.NEQ || !lIsBool) {
			n := int64(0)
			if right.(BoolValue).Val {
				n = 1
			}
			right = NewInt(n, types.Typ(types.Int), ctx.Model)
		}
	}

	// Pointer arithmetic
	if lp, ok := left.(PointerValue); ok {
		if ri, ok := right.(IntValue); ok {
			switch op {
			case token.ADD:
				return PointerValue{Target: lp.Target, Path: lp.Path, Offset: lp.Offset + ri.Int64(), Typ: lp.Typ}, nil
			case token.SUB:
				return PointerValue{Target: lp.Target, Path: lp.Path, Offset: lp.Offset - ri.Int64(), Typ: lp.Typ}, nil
			}
		}
		if rp, ok := right.(PointerValue); ok && op == token.SUB {
			if lp.Target != rp.Target {
				return nil, fmt.Errorf("%w: pointer subtraction across different objects", ErrUB)
			}
			return NewInt(lp.Offset-rp.Offset, types.Typ(types.Long), ctx.Model), nil
		}
	}

	// Floating point arithmetic
	if lf, ok := left.(FloatValue); ok {
		rfVal := 0.0
		if rf, ok := right.(FloatValue); ok {
			rfVal = rf.Val
		} else if ri, ok := right.(IntValue); ok {
			rfVal = float64(ri.Int64())
		}
		return ctx.evalFloatBinary(op, lf.Val, rfVal, lf.Type())
	}
	if rf, ok := right.(FloatValue); ok {
		lfVal := 0.0
		if li, ok := left.(IntValue); ok {
			lfVal = float64(li.Int64())
		}
		return ctx.evalFloatBinary(op, lfVal, rf.Val, rf.Type())
	}

	// Integer arithmetic
	li, lOk := left.(IntValue)
	ri, rOk := right.(IntValue)
	if lOk && rOk {
		return ctx.evalIntBinary(op, li, ri)
	}

	// Equality comparisons for other types (nullptr, pointers, booleans)
	if op == token.EQL {
		return NewBool(left.Equal(right)), nil
	}
	if op == token.NEQ {
		return NewBool(!left.Equal(right)), nil
	}

	return nil, fmt.Errorf("%w: binary operator %s not applicable between %s and %s", ErrUB, op, left.Kind(), right.Kind())
}

func (ctx *Context) evalFloatBinary(op token.Kind, a, b float64, typ types.Type) (Value, error) {
	switch op {
	case token.ADD:
		return NewFloat(a+b, typ), nil
	case token.SUB:
		return NewFloat(a-b, typ), nil
	case token.MUL:
		return NewFloat(a*b, typ), nil
	case token.QUO:
		if b == 0.0 {
			return nil, fmt.Errorf("%w: floating-point division by zero", ErrUB)
		}
		return NewFloat(a/b, typ), nil
	case token.EQL:
		return NewBool(a == b), nil
	case token.NEQ:
		return NewBool(a != b), nil
	case token.LSS:
		return NewBool(a < b), nil
	case token.LEQ:
		return NewBool(a <= b), nil
	case token.GTR:
		return NewBool(a > b), nil
	case token.GEQ:
		return NewBool(a >= b), nil
	case token.SPACESHIP:
		if a < b {
			return NewInt(-1, types.Typ(types.Int), ctx.Model), nil
		} else if a > b {
			return NewInt(1, types.Typ(types.Int), ctx.Model), nil
		}
		return NewInt(0, types.Typ(types.Int), ctx.Model), nil
	default:
		return nil, fmt.Errorf("%w: binary op %s invalid on float", ErrUB, op)
	}
}

func (ctx *Context) evalIntBinary(op token.Kind, a, b IntValue) (Value, error) {
	res := new(big.Int)
	isSigned := a.IsSigned && b.IsSigned
	retType := a.Type()

	switch op {
	case token.ADD:
		res.Add(a.Val, b.Val)

	case token.SUB:
		res.Sub(a.Val, b.Val)

	case token.MUL:
		res.Mul(a.Val, b.Val)

	case token.QUO:
		if b.Val.Sign() == 0 {
			return nil, fmt.Errorf("%w: division by zero in constant expression", ErrUB)
		}
		res.Quo(a.Val, b.Val)

	case token.REM:
		if b.Val.Sign() == 0 {
			return nil, fmt.Errorf("%w: modulo by zero in constant expression", ErrUB)
		}
		res.Rem(a.Val, b.Val)

	case token.AND:
		res.And(a.Val, b.Val)

	case token.OR:
		res.Or(a.Val, b.Val)

	case token.XOR:
		res.Xor(a.Val, b.Val)

	case token.SHL:
		shift := b.Int64()
		if shift < 0 || shift >= int64(a.BitWidth) {
			return nil, fmt.Errorf("%w: shift count %d out of range [0, %d)", ErrUB, shift, a.BitWidth)
		}
		res.Lsh(a.Val, uint(shift))

	case token.SHR:
		shift := b.Int64()
		if shift < 0 || shift >= int64(a.BitWidth) {
			return nil, fmt.Errorf("%w: shift count %d out of range [0, %d)", ErrUB, shift, a.BitWidth)
		}
		res.Rsh(a.Val, uint(shift))

	case token.EQL:
		return NewBool(a.Val.Cmp(b.Val) == 0), nil

	case token.NEQ:
		return NewBool(a.Val.Cmp(b.Val) != 0), nil

	case token.LSS:
		return NewBool(a.Val.Cmp(b.Val) < 0), nil

	case token.LEQ:
		return NewBool(a.Val.Cmp(b.Val) <= 0), nil

	case token.GTR:
		return NewBool(a.Val.Cmp(b.Val) > 0), nil

	case token.GEQ:
		return NewBool(a.Val.Cmp(b.Val) >= 0), nil

	case token.SPACESHIP:
		return NewInt(int64(a.Val.Cmp(b.Val)), types.Typ(types.Int), ctx.Model), nil

	default:
		return nil, fmt.Errorf("%w: unknown binary operator %s", ErrNonConstexpr, op)
	}

	bw := a.BitWidth
	if b.BitWidth > bw {
		bw = b.BitWidth
	}
	iv := IntValue{
		Val:      res,
		BitWidth: bw,
		IsSigned: isSigned,
		Typ:      retType,
	}
	iv.normalize()
	return iv, nil
}

func (ctx *Context) evalCond(c *ast.CondExpr) (Value, error) {
	cond, err := ctx.Eval(c.Cond)
	if err != nil {
		return nil, err
	}
	if cond.ToBool() {
		return ctx.Eval(c.Then)
	}
	return ctx.Eval(c.Else)
}

func (ctx *Context) evalAssign(a *ast.AssignExpr) (Value, error) {
	obj, path, offset, err := ctx.EvalLValue(a.Lhs)
	if err != nil {
		return nil, err
	}

	rhsVal, err := ctx.Eval(a.Rhs)
	if err != nil {
		return nil, err
	}

	if a.Op == token.ASSIGN {
		if err := obj.Write(path, offset, rhsVal); err != nil {
			return nil, err
		}
		return rhsVal, nil
	}

	// Compound assignment
	curVal, err := obj.Read(path, offset)
	if err != nil {
		return nil, err
	}

	var binOp token.Kind
	switch a.Op {
	case token.ADD_ASSIGN:
		binOp = token.ADD
	case token.SUB_ASSIGN:
		binOp = token.SUB
	case token.MUL_ASSIGN:
		binOp = token.MUL
	case token.QUO_ASSIGN:
		binOp = token.QUO
	case token.REM_ASSIGN:
		binOp = token.REM
	case token.AND_ASSIGN:
		binOp = token.AND
	case token.OR_ASSIGN:
		binOp = token.OR
	case token.XOR_ASSIGN:
		binOp = token.XOR
	case token.SHL_ASSIGN:
		binOp = token.SHL
	case token.SHR_ASSIGN:
		binOp = token.SHR
	default:
		return nil, fmt.Errorf("%w: unsupported assignment operator %s", ErrNonConstexpr, a.Op)
	}

	newVal, err := ctx.evalBinaryOp(binOp, curVal, rhsVal)
	if err != nil {
		return nil, err
	}
	if err := obj.Write(path, offset, newVal); err != nil {
		return nil, err
	}
	return newVal, nil
}

func (ctx *Context) evalIndex(idx *ast.IndexExpr) (Value, error) {
	obj, path, offset, err := ctx.EvalLValue(idx)
	if err != nil {
		return nil, err
	}
	return obj.Read(path, offset)
}

func (ctx *Context) evalMember(m *ast.MemberExpr) (Value, error) {
	obj, path, offset, err := ctx.EvalLValue(m)
	if err != nil {
		return nil, err
	}
	return obj.Read(path, offset)
}

func (ctx *Context) evalCall(call *ast.CallExpr) (Value, error) {
	// Check if call is an explicit type conversion `T(...)`
	if ctx.ResolveTypeExpr != nil {
		if t, ok := ctx.ResolveTypeExpr(call.Fun); ok && t != nil {
			if len(call.Args) == 1 {
				v, err := ctx.Eval(call.Args[0])
				if err != nil {
					return nil, err
				}
				return ctx.convertToType(v, t), nil
			}
			if len(call.Args) == 0 {
				return NewInt(0, t, ctx.Model), nil
			}
		}
	}

	// Check for identifier call
	if id, ok := call.Fun.(*ast.Ident); ok {
		fnName := id.Text(ctx.Unit)

		// Builtin: __builtin_is_constant_evaluated
		if fnName == "__builtin_is_constant_evaluated" {
			return NewBool(true), nil
		}
		// Builtin: __builtin_expect
		if fnName == "__builtin_expect" && len(call.Args) > 0 {
			return ctx.Eval(call.Args[0])
		}

		// Look up user constexpr function
		fn, ok := ctx.Funcs[fnName]
		if !ok && ctx.ResolveCall != nil {
			fn, ok = ctx.ResolveCall(call)
		}
		if !ok && ctx.ResolveFunc != nil {
			var err error
			fn, err = ctx.ResolveFunc(fnName)
			if err != nil {
				return nil, err
			}
		}

		if fn != nil {
			return ctx.callFunction(fn, call.Args)
		}
	}

	return nil, fmt.Errorf("%w: call to non-constexpr function", ErrNonConstexpr)
}

func (ctx *Context) callFunction(fn *FuncInfo, args []ast.Expr) (Value, error) {
	if ctx.CallDepth >= ctx.MaxCallDepth {
		return nil, fmt.Errorf("maximum constexpr call depth (%d) exceeded in call to %q", ctx.MaxCallDepth, fn.Name)
	}

	// Evaluate arguments
	argVals := make([]Value, len(args))
	for i, arg := range args {
		v, err := ctx.Eval(arg)
		if err != nil {
			return nil, err
		}
		argVals[i] = v
	}

	ctx.CallDepth++
	defer func() { ctx.CallDepth-- }()

	savedUnit := ctx.Unit
	if fn.Unit != nil {
		ctx.Unit = fn.Unit
	}
	defer func() { ctx.Unit = savedUnit }()

	// Enter new scope for function call
	savedScope := ctx.CurScope
	ctx.CurScope = NewScope(nil)
	defer func() { ctx.CurScope = savedScope }()

	// Bind parameters
	for i, name := range fn.ParamNames {
		var paramTyp types.Type
		if i < len(fn.ParamTypes) {
			paramTyp = fn.ParamTypes[i]
		}
		var val Value
		if i < len(argVals) {
			val = argVals[i]
		}
		ctx.DeclareVar(name, paramTyp, val, false)
	}

	// Execute body
	if fn.Body == nil {
		return VoidValue{}, nil
	}

	sig, retVal, err := ctx.EvalCompoundStmt(fn.Body)
	if err != nil {
		return nil, err
	}
	if sig == FlowReturn {
		return retVal, nil
	}

	return VoidValue{}, nil
}

func (ctx *Context) evalNamedCast(c *ast.NamedCastExpr) (Value, error) {
	if c.Kind == token.REINTERPRET_CAST {
		return nil, fmt.Errorf("%w: reinterpret_cast is not permitted in constant expressions", ErrUB)
	}
	v, err := ctx.Eval(c.X)
	if err != nil {
		return nil, err
	}
	return ctx.convertTo(v, c.Type), nil
}

// convertTo applies a cast's conversion to an already-evaluated value,
// truncating floats or adjusting integer widths to match the target type.
func (ctx *Context) convertTo(v Value, id *ast.TypeId) Value {
	target, ok := ctx.resolveTypeId(id)
	if !ok || target == nil || v == nil {
		return v
	}
	return ctx.convertToType(v, target)
}

func (ctx *Context) convertToType(v Value, target types.Type) Value {
	if target == nil || v == nil {
		return v
	}
	switch {
	case types.IsBool(target):
		switch x := v.(type) {
		case IntValue:
			return NewBool(x.Val.Sign() != 0)
		case FloatValue:
			return NewBool(x.Val != 0)
		case BoolValue:
			return x
		}
	case types.IsInteger(target) || types.IsEnum(target):
		switch x := v.(type) {
		case IntValue:
			return NewInt(x.Val.Int64(), target, ctx.Model)
		case FloatValue:
			// The fractional part is discarded.
			return NewInt(int64(x.Val), target, ctx.Model)
		case BoolValue:
			n := int64(0)
			if x.Val {
				n = 1
			}
			return NewInt(n, target, ctx.Model)
		}
	case types.IsFloat(target):
		switch x := v.(type) {
		case IntValue:
			f, _ := new(big.Float).SetInt(x.Val).Float64()
			return NewFloat(f, target)
		case FloatValue:
			return NewFloat(x.Val, target)
		case BoolValue:
			f := 0.0
			if x.Val {
				f = 1
			}
			return NewFloat(f, target)
		}
	}
	return v
}

func (ctx *Context) evalFunctionalCast(fc *ast.FunctionalCastExpr) (Value, error) {
	if len(fc.ArgList) == 1 {
		return ctx.Eval(fc.ArgList[0])
	}
	if fc.Args != nil {
		return ctx.evalInitList(fc.Args)
	}
	return VoidValue{}, nil
}

// evalSizeof evaluates sizeof expressions using the target data model.
func (ctx *Context) evalSizeof(s *ast.SizeofExpr) (Value, error) {
	sizeType := types.Typ(types.ULong)

	if s.Ellipsis.IsValid() {
		// sizeof...(pack): the count the analysis noted when it bound
		// the pack.
		if ctx.Folded != nil {
			if n, ok := ctx.Folded(s); ok {
				return NewInt(n, sizeType, ctx.Model), nil
			}
		}
		return nil, fmt.Errorf("%w: sizeof... of a pack this evaluator cannot count", ErrNonConstexpr)
	}

	if s.Type != nil {
		t, ok := ctx.resolveTypeId(s.Type)
		if !ok {
			return nil, fmt.Errorf("%w: sizeof of a type this evaluator cannot resolve", ErrNonConstexpr)
		}
		sz, ok := ctx.Model.Sizeof(t)
		if !ok {
			return nil, fmt.Errorf("%w: sizeof of an incomplete type", ErrNonConstexpr)
		}
		return NewInt(sz, sizeType, ctx.Model), nil
	}

	// sizeof applied to an expression: the operand is unevaluated.
	if s.X != nil {
		if ctx.TypeOfExpr != nil {
			if t := ctx.TypeOfExpr(s.X); t != nil {
				if sz, ok := ctx.Model.Sizeof(types.RemoveReference(t)); ok {
					return NewInt(sz, sizeType, ctx.Model), nil
				}
			}
		}
		if ctx.ResolveTypeExpr != nil {
			if t, ok := ctx.ResolveTypeExpr(s.X); ok && t != nil {
				if sz, ok := ctx.Model.Sizeof(types.RemoveReference(t)); ok {
					return NewInt(sz, sizeType, ctx.Model), nil
				}
			}
		}
		v, err := ctx.Eval(s.X)
		if err != nil {
			return nil, err
		}
		if sz, ok := ctx.Model.Sizeof(v.Type()); ok {
			return NewInt(sz, sizeType, ctx.Model), nil
		}
	}
	return nil, fmt.Errorf("%w: sizeof of an operand this evaluator cannot type", ErrNonConstexpr)
}

func (ctx *Context) evalAlignof(a *ast.AlignofExpr) (Value, error) {
	if a.Type == nil {
		return nil, fmt.Errorf("%w: alignof without a type-id", ErrNonConstexpr)
	}
	t, ok := ctx.resolveTypeId(a.Type)
	if !ok {
		return nil, fmt.Errorf("%w: alignof of a type this evaluator cannot resolve", ErrNonConstexpr)
	}
	al, ok := ctx.Model.Alignof(t)
	if !ok {
		return nil, fmt.Errorf("%w: alignof of an incomplete type", ErrNonConstexpr)
	}
	return NewInt(al, types.Typ(types.ULong), ctx.Model), nil
}

func (ctx *Context) resolveTypeId(id *ast.TypeId) (types.Type, bool) {
	if id == nil || ctx.ResolveType == nil {
		return nil, false
	}
	return ctx.ResolveType(id)
}

// declaredArrayBound reads the `[n]` off a declarator, so that an array
// initialized by a shorter list still has the length it was declared with.
func (ctx *Context) declaredArrayBound(d ast.Declarator) (int64, bool) {
	for d != nil {
		switch t := d.(type) {
		case *ast.ArrayDeclarator:
			if t.Size == nil {
				return 0, false
			}
			n, err := ctx.EvalInt(t.Size)
			if err != nil || n < 0 {
				return 0, false
			}
			return n, true
		case *ast.PointerDeclarator:
			d = t.Inner
		case *ast.ParenDeclarator:
			d = t.Inner
		default:
			return 0, false
		}
	}
	return 0, false
}

func (ctx *Context) evalInitList(init *ast.InitList) (Value, error) {
	elems := make([]Value, len(init.Items))
	for i, item := range init.Items {
		v, err := ctx.Eval(item)
		if err != nil {
			return nil, err
		}
		elems[i] = v
	}
	var elemType types.Type = types.Typ(types.Int)
	if len(elems) > 0 && elems[0] != nil {
		elemType = elems[0].Type()
	}
	return NewArray(elems, elemType), nil
}

// evalTypeTrait answers the traits a front end has to answer itself, because
// no library can: they are questions about the type system, and the compiler
// is the only thing that knows it.
//
// Only the ones decidable from a resolved type are here, and a trait outside
// that set says so rather than guessing. That matters more than it looks: a
// trait that returns true by default makes every constraint written with it
// satisfied, so a concept meant to reject a type quietly accepts it, and the
// wrong overload is chosen with no diagnostic anywhere. Refusing is the only
// answer that cannot be silently wrong -- which this used to be, returning
// true for every unknown trait and false for every __is_same.
func (ctx *Context) evalTypeTrait(tt *ast.TypeTraitExpr) (Value, error) {
	name := tt.Name.Text(ctx.Unit)

	arg := func(i int) (types.Type, bool) {
		if i >= len(tt.Args) {
			return nil, false
		}
		return ctx.resolveTypeId(tt.Args[i])
	}
	one := func(f func(types.Type) bool) (Value, error) {
		a, ok := arg(0)
		if !ok {
			return nil, fmt.Errorf("%w: %s of a type this evaluator cannot resolve", ErrNonConstexpr, name)
		}
		return NewBool(f(a)), nil
	}

	switch name {
	// In the constant evaluator, is_constant_evaluated evaluates to true.
	case "__builtin_is_constant_evaluated", "__is_constant_evaluated":
		return NewBool(true), nil

	case "__builtin_offsetof":
		// The parser read `__builtin_offsetof(S, m)` as two type-ids; the
		// second is the member's name, spelled as a declarator-id.
		t, ok := arg(0)
		if !ok || len(tt.Args) < 2 {
			return nil, fmt.Errorf("%w: __builtin_offsetof of a type this evaluator cannot resolve", ErrNonConstexpr)
		}
		member := typeIdName(tt.Args[1], ctx.Unit)
		if member == "" {
			return nil, fmt.Errorf("%w: __builtin_offsetof with a member this evaluator cannot read", ErrNonConstexpr)
		}
		off, found := ctx.Model.Offsetof(types.Unqualify(t), member)
		if !found {
			return nil, fmt.Errorf("%w: %s has no member %s", ErrNonConstexpr, t, member)
		}
		return NewInt(off, types.Typ(types.ULongLong), ctx.Model), nil

	case "__is_same", "__is_same_as":
		a, aok := arg(0)
		b, bok := arg(1)
		if aok && bok {
			return NewBool(a.Equal(b)), nil
		}

	case "__is_const":
		return one(types.IsConst)
	case "__is_volatile":
		return one(types.IsVolatile)
	case "__is_reference":
		return one(types.IsReference)
	case "__is_lvalue_reference":
		return one(types.IsLValueReference)
	case "__is_rvalue_reference":
		return one(types.IsRValueReference)
	case "__is_pointer":
		return one(func(t types.Type) bool { return types.IsPointer(types.Unqualify(t)) })
	case "__is_void":
		return one(func(t types.Type) bool { return types.IsVoid(types.Unqualify(t)) })
	case "__is_integral":
		return one(func(t types.Type) bool { return types.IsInteger(types.Unqualify(t)) })
	case "__is_floating_point":
		return one(func(t types.Type) bool { return types.IsFloat(types.Unqualify(t)) })
	case "__is_arithmetic":
		return one(func(t types.Type) bool { return types.IsArithmetic(types.Unqualify(t)) })
	case "__is_enum":
		return one(func(t types.Type) bool { return types.IsEnum(types.Unqualify(t)) })
	case "__is_array":
		return one(func(t types.Type) bool {
			_, isArr := types.Unqualify(t).(*types.Array)
			return isArr
		})
	case "__is_class":
		return one(func(t types.Type) bool {
			rec := types.AsRecord(types.Unqualify(t))
			return rec != nil && rec.Tag != types.TagUnion
		})
	case "__is_union":
		return one(func(t types.Type) bool {
			rec := types.AsRecord(types.Unqualify(t))
			return rec != nil && rec.Tag == types.TagUnion
		})
	case "__is_function":
		return one(func(t types.Type) bool { return types.IsFunc(types.Unqualify(t)) })
	case "__is_scalar":
		return one(types.IsScalar)
	case "__is_object":
		return one(func(t types.Type) bool {
			u := types.Unqualify(t)
			return !types.IsFunc(u) && !types.IsReference(t) && !types.IsVoid(u)
		})
	case "__is_fundamental":
		return one(func(t types.Type) bool {
			u := types.Unqualify(t)
			return types.IsArithmetic(u) || types.IsVoid(u) || u.Kind() == types.NullptrKind
		})
	case "__is_compound":
		return one(func(t types.Type) bool {
			u := types.Unqualify(t)
			return !(types.IsArithmetic(u) || types.IsVoid(u) || u.Kind() == types.NullptrKind)
		})
	case "__is_signed":
		return one(func(t types.Type) bool {
			u := types.Unqualify(t)
			return types.IsArithmetic(u) && (types.IsFloat(u) || types.IsSigned(u))
		})
	case "__is_unsigned":
		return one(func(t types.Type) bool {
			u := types.Unqualify(t)
			return types.IsInteger(u) && types.IsUnsigned(u)
		})
	case "__is_bounded_array":
		return one(func(t types.Type) bool {
			arr, isArr := types.Unqualify(t).(*types.Array)
			return isArr && !arr.Incomplete
		})
	case "__is_unbounded_array":
		return one(func(t types.Type) bool {
			arr, isArr := types.Unqualify(t).(*types.Array)
			return isArr && arr.Incomplete
		})
	case "__is_scoped_enum":
		return one(func(t types.Type) bool {
			e, isEnum := types.Unqualify(t).(*types.Enum)
			return isEnum && e.Scoped
		})
	case "__is_nullptr":
		return one(func(t types.Type) bool { return types.Unqualify(t).Kind() == types.NullptrKind })
	case "__is_member_pointer":
		return one(func(t types.Type) bool {
			_, isMP := types.Unqualify(t).(*types.MemberPointer)
			return isMP
		})
	case "__is_member_function_pointer":
		return one(func(t types.Type) bool {
			mp, isMP := types.Unqualify(t).(*types.MemberPointer)
			return isMP && types.IsFunc(mp.Elem)
		})
	case "__is_member_object_pointer":
		return one(func(t types.Type) bool {
			mp, isMP := types.Unqualify(t).(*types.MemberPointer)
			return isMP && !types.IsFunc(mp.Elem)
		})

	case "__is_base_of":
		a, aok := arg(0)
		b, bok := arg(1)
		if aok && bok {
			base := types.AsRecord(types.Unqualify(a))
			derived := types.AsRecord(types.Unqualify(b))
			if base == nil || derived == nil {
				return NewBool(false), nil
			}
			return NewBool(base == derived || types.IsBaseOf(base, derived)), nil
		}
	}

	if ctx.ResolveTrait != nil {
		var args []types.Type
		for i := range tt.Args {
			t, ok := arg(i)
			if !ok {
				return nil, fmt.Errorf("%w: %s of a type this evaluator cannot resolve", ErrNonConstexpr, name)
			}
			// Flatten pack argument into individual type arguments.
			if pack, isPack := types.Unqualify(t).(*types.Pack); isPack {
				for _, e := range pack.Elems {
					args = append(args, e.Type)
				}
				continue
			}
			args = append(args, t)
		}
		answer, known, err := ctx.ResolveTrait(name, args)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrNonConstexpr, name, err)
		}
		if known {
			return NewBool(answer), nil
		}
	}
	return nil, fmt.Errorf("%w: the type trait %s is not one this evaluator decides", ErrNonConstexpr, name)
}

// EvalLValue evaluates an expression to an Object reference, subobject path, and offset.
func (ctx *Context) EvalLValue(expr ast.Expr) (*Object, []PathStep, int64, error) {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return ctx.EvalLValue(e.X)

	case *ast.Ident:
		name := e.Text(ctx.Unit)
		if obj := ctx.CurScope.Lookup(name); obj != nil {
			return obj, nil, 0, nil
		}
		if obj, ok := ctx.Globals[name]; ok {
			return obj, nil, 0, nil
		}
		// A constant object the analysis knows -- a namespace-scope
		// constexpr array, say -- read into an object of this
		// evaluation, which is enough to subscript or take a member of.
		if ctx.ResolveVar != nil {
			if v, err := ctx.ResolveVar(name); err == nil {
				return NewObject(ctx.nextID(), name, v.Type(), v, true), nil, 0, nil
			}
		}
		return nil, nil, 0, fmt.Errorf("%w: identifier %q not found for lvalue", ErrNonConstexpr, name)

	case *ast.StringLit:
		// A string literal is an lvalue array of const char.
		v, err := ctx.evalStringLit(e)
		if err != nil {
			return nil, nil, 0, err
		}
		return NewObject(ctx.nextID(), "", v.Type(), v, true), nil, 0, nil

	case *ast.UnaryExpr:
		if e.Op == token.MUL { // *ptr
			val, err := ctx.Eval(e.X)
			if err != nil {
				return nil, nil, 0, err
			}
			ptr, ok := val.(PointerValue)
			if !ok {
				return nil, nil, 0, fmt.Errorf("%w: dereference of non-pointer in lvalue", ErrUB)
			}
			if ptr.IsNull() {
				return nil, nil, 0, ErrNullPointer
			}
			return ptr.Target, ptr.Path, ptr.Offset, nil
		}

	case *ast.IndexExpr:
		// Array indexing: arr[idx]
		subObj, subPath, subOffset, err := ctx.EvalLValue(e.X)
		if err != nil {
			return nil, nil, 0, err
		}
		if len(e.Args) == 0 {
			return nil, nil, 0, errors.New("empty index expression")
		}
		idxVal, err := ctx.EvalInt(e.Args[0])
		if err != nil {
			return nil, nil, 0, err
		}

		// Append index step
		newPath := append(append([]PathStep(nil), subPath...), PathStep{
			Kind:  PathIndex,
			Index: int(idxVal),
		})
		return subObj, newPath, subOffset, nil

	case *ast.MemberExpr:
		fieldName := ""
		if id, ok := e.Sel.(*ast.Ident); ok {
			fieldName = id.Text(ctx.Unit)
		}
		if fieldName == "" {
			return nil, nil, 0, errors.New("invalid member name in lvalue")
		}

		if e.Op == token.ARROW {
			// ptr->field: dereference ptr first
			val, err := ctx.Eval(e.X)
			if err != nil {
				return nil, nil, 0, err
			}
			ptr, ok := val.(PointerValue)
			if !ok {
				return nil, nil, 0, fmt.Errorf("%w: arrow on non-pointer in lvalue", ErrUB)
			}
			if ptr.IsNull() {
				return nil, nil, 0, ErrNullPointer
			}
			newPath := append(append([]PathStep(nil), ptr.Path...), PathStep{
				Kind: PathField,
				Name: fieldName,
			})
			return ptr.Target, newPath, ptr.Offset, nil
		}

		// obj.field
		subObj, subPath, subOffset, err := ctx.EvalLValue(e.X)
		if err != nil {
			return nil, nil, 0, err
		}
		newPath := append(append([]PathStep(nil), subPath...), PathStep{
			Kind: PathField,
			Name: fieldName,
		})
		return subObj, newPath, subOffset, nil
	}

	return nil, nil, 0, fmt.Errorf("%w: cannot evaluate %T as lvalue", ErrNonConstexpr, expr)
}

// EvalStmt evaluates a statement in the constexpr environment.
func (ctx *Context) EvalStmt(stmt ast.Stmt) (FlowSignal, Value, error) {
	if stmt == nil {
		return FlowNone, nil, nil
	}
	ctx.StepCount++
	if ctx.StepCount > ctx.MaxSteps {
		return FlowNone, nil, fmt.Errorf("constexpr step limit (%d) exceeded", ctx.MaxSteps)
	}

	switch s := stmt.(type) {
	case *ast.CompoundStmt:
		return ctx.EvalCompoundStmt(s)

	case *ast.ExprStmt:
		_, err := ctx.Eval(s.X)
		return FlowNone, nil, err

	case *ast.DeclStmt:
		return ctx.evalDeclStmt(s)

	case *ast.IfStmt:
		return ctx.evalIfStmt(s)

	case *ast.WhileStmt:
		return ctx.evalWhileStmt(s)

	case *ast.DoStmt:
		return ctx.evalDoStmt(s)

	case *ast.ForStmt:
		return ctx.evalForStmt(s)

	case *ast.RangeForStmt:
		return ctx.evalRangeForStmt(s)

	case *ast.ReturnStmt:
		if s.X != nil {
			v, err := ctx.Eval(s.X)
			if err != nil {
				return FlowNone, nil, err
			}
			return FlowReturn, v, nil
		}
		return FlowReturn, VoidValue{}, nil

	case *ast.BreakStmt:
		return FlowBreak, nil, nil

	case *ast.ContinueStmt:
		return FlowContinue, nil, nil

	case *ast.EmptyStmt:
		return FlowNone, nil, nil

	default:
		return FlowNone, nil, fmt.Errorf("%w: unsupported statement type %T", ErrNonConstexpr, stmt)
	}
}

func (ctx *Context) EvalCompoundStmt(comp *ast.CompoundStmt) (FlowSignal, Value, error) {
	ctx.PushScope()
	defer ctx.PopScope()

	for _, stmt := range comp.Stmts {
		sig, val, err := ctx.EvalStmt(stmt)
		if err != nil {
			return FlowNone, nil, err
		}
		if sig != FlowNone {
			return sig, val, nil
		}
	}
	return FlowNone, nil, nil
}

func (ctx *Context) evalDeclStmt(ds *ast.DeclStmt) (FlowSignal, Value, error) {
	sd, ok := ds.Decl.(*ast.SimpleDecl)
	if !ok {
		return FlowNone, nil, nil
	}

	for _, init := range sd.Inits {
		name := ""
		if init.Decl != nil && init.Decl.DeclName() != nil {
			if id, ok := init.Decl.DeclName().(*ast.Ident); ok {
				name = id.Text(ctx.Unit)
			}
		}
		if name == "" {
			continue
		}

		var initVal Value
		if init.Value != nil {
			v, err := ctx.Eval(init.Value)
			if err != nil {
				return FlowNone, nil, err
			}
			initVal = v
		} else if init.Braced != nil {
			v, err := ctx.evalInitList(init.Braced)
			if err != nil {
				return FlowNone, nil, err
			}
			initVal = v
		} else if len(init.Args) == 1 {
			v, err := ctx.Eval(init.Args[0])
			if err != nil {
				return FlowNone, nil, err
			}
			initVal = v
		}

		// Aggregate initialization: elements not explicitly initialized
		// are value-initialized up to the declared bound.
		if arr, ok := initVal.(ArrayValue); ok {
			if n, ok := ctx.declaredArrayBound(init.Decl); ok && n > int64(len(arr.Elements)) {
				elems := make([]Value, n)
				copy(elems, arr.Elements)
				elemType := arr.ElemType
				if elemType == nil {
					elemType = types.Typ(types.Int)
				}
				for i := int64(len(arr.Elements)); i < n; i++ {
					elems[i] = NewInt(0, elemType, ctx.Model)
				}
				initVal = NewArray(elems, elemType)
			}
		}

		var typ types.Type = types.Typ(types.Int)
		if initVal != nil {
			typ = initVal.Type()
		}
		ctx.DeclareVar(name, typ, initVal, false)
	}

	return FlowNone, nil, nil
}

func (ctx *Context) evalIfStmt(ifs *ast.IfStmt) (FlowSignal, Value, error) {
	if ifs.Init != nil {
		ctx.PushScope()
		defer ctx.PopScope()
		sig, val, err := ctx.EvalStmt(ifs.Init)
		if err != nil {
			return FlowNone, nil, err
		}
		if sig != FlowNone {
			return sig, val, nil
		}
	}

	var condBool bool
	switch {
	case ifs.Consteval.IsValid():
		// In constant evaluation, if consteval is true (or inverted for !consteval).
		condBool = !ifs.Not.IsValid()

	default:
		if expr, ok := ifs.Cond.(ast.Expr); ok {
			b, err := ctx.EvalBool(expr)
			if err != nil {
				return FlowNone, nil, err
			}
			condBool = b
		}
	}

	if condBool {
		if ifs.Then != nil {
			return ctx.EvalStmt(ifs.Then)
		}
	} else if ifs.Else != nil {
		return ctx.EvalStmt(ifs.Else)
	}

	return FlowNone, nil, nil
}

func (ctx *Context) evalWhileStmt(w *ast.WhileStmt) (FlowSignal, Value, error) {
	for {
		if expr, ok := w.Cond.(ast.Expr); ok {
			b, err := ctx.EvalBool(expr)
			if err != nil {
				return FlowNone, nil, err
			}
			if !b {
				break
			}
		}

		sig, val, err := ctx.EvalStmt(w.Body)
		if err != nil {
			return FlowNone, nil, err
		}
		if sig == FlowBreak {
			break
		}
		if sig == FlowReturn {
			return FlowReturn, val, nil
		}
	}
	return FlowNone, nil, nil
}

func (ctx *Context) evalDoStmt(d *ast.DoStmt) (FlowSignal, Value, error) {
	for {
		sig, val, err := ctx.EvalStmt(d.Body)
		if err != nil {
			return FlowNone, nil, err
		}
		if sig == FlowBreak {
			break
		}
		if sig == FlowReturn {
			return FlowReturn, val, nil
		}

		if d.Cond != nil {
			b, err := ctx.EvalBool(d.Cond)
			if err != nil {
				return FlowNone, nil, err
			}
			if !b {
				break
			}
		}
	}
	return FlowNone, nil, nil
}

// evalRangeForStmt evaluates a range-based for loop over an array or string literal.
func (ctx *Context) evalRangeForStmt(f *ast.RangeForStmt) (FlowSignal, Value, error) {
	ctx.PushScope()
	defer ctx.PopScope()

	if f.Init != nil {
		sig, val, err := ctx.EvalStmt(f.Init)
		if err != nil {
			return FlowNone, nil, err
		}
		if sig != FlowNone {
			return sig, val, nil
		}
	}

	rangeVal, err := ctx.Eval(f.Range)
	if err != nil {
		return FlowNone, nil, err
	}

	var elems []Value
	switch r := rangeVal.(type) {
	case ArrayValue:
		elems = r.Elements
	case StringValue:
		// A string literal is an array of char including its null terminator.
		for i := 0; i <= len(r.Val); i++ {
			var ch byte
			if i < len(r.Val) {
				ch = r.Val[i]
			}
			elems = append(elems, NewInt(int64(ch), types.Typ(types.Char), ctx.Model))
		}
	default:
		return FlowNone, nil, fmt.Errorf("%w: a ranged loop over %T needs begin() and end(), which this evaluator does not call yet",
			ErrNonConstexpr, rangeVal)
	}

	name, byRef := ctx.rangeLoopVar(f.Decl)
	if name == "" {
		return FlowNone, nil, fmt.Errorf("%w: a ranged loop whose declaration names nothing", ErrNonConstexpr)
	}

	for i := range elems {
		// Each iteration declares the variable in its own scope.
		ctx.PushScope()
		var elemType types.Type
		if elems[i] != nil {
			elemType = elems[i].Type()
		}
		obj := ctx.DeclareVar(name, elemType, elems[i], false)

		sig, val, err := ctx.EvalStmt(f.Body)
		if byRef && obj != nil {
			// The loop bound a reference, so what the body wrote goes back
			// into the range rather than into a copy that is about to be
			// discarded.
			elems[i] = obj.Val
		}
		ctx.PopScope()

		if err != nil {
			return FlowNone, nil, err
		}
		if sig == FlowBreak {
			break
		}
		if sig == FlowReturn {
			return FlowReturn, val, nil
		}
	}

	return FlowNone, nil, nil
}

// rangeLoopVar reads the name a for-range-declaration declares, and whether
// it declared a reference to the element rather than a copy of it.
func (ctx *Context) rangeLoopVar(d ast.Decl) (string, bool) {
	sd, ok := d.(*ast.SimpleDecl)
	if !ok || len(sd.Inits) == 0 {
		return "", false
	}
	decl := sd.Inits[0].Decl
	byRef := false
	for cur := decl; cur != nil; {
		ptr, isPtr := cur.(*ast.PointerDeclarator)
		if !isPtr {
			break
		}
		if ptr.Kind == token.AND || ptr.Kind == token.LAND {
			byRef = true
		}
		cur = ptr.Inner
	}
	if decl == nil || decl.DeclName() == nil {
		return "", byRef
	}
	id, ok := decl.DeclName().(*ast.Ident)
	if !ok {
		return "", byRef
	}
	return id.Text(ctx.Unit), byRef
}

func (ctx *Context) evalForStmt(f *ast.ForStmt) (FlowSignal, Value, error) {
	ctx.PushScope()
	defer ctx.PopScope()

	if f.Init != nil {
		sig, val, err := ctx.EvalStmt(f.Init)
		if err != nil {
			return FlowNone, nil, err
		}
		if sig != FlowNone {
			return sig, val, nil
		}
	}

	for {
		if f.Cond != nil {
			if expr, ok := f.Cond.(ast.Expr); ok {
				b, err := ctx.EvalBool(expr)
				if err != nil {
					return FlowNone, nil, err
				}
				if !b {
					break
				}
			}
		}

		if f.Body != nil {
			sig, val, err := ctx.EvalStmt(f.Body)
			if err != nil {
				return FlowNone, nil, err
			}
			if sig == FlowBreak {
				break
			}
			if sig == FlowReturn {
				return FlowReturn, val, nil
			}
		}

		if f.Post != nil {
			_, err := ctx.Eval(f.Post)
			if err != nil {
				return FlowNone, nil, err
			}
		}
	}

	return FlowNone, nil, nil
}

// typeIdName is the one identifier a type-id is made of, or "": the shape
// a member name takes when the parser reads it where a type-id goes.
func typeIdName(id *ast.TypeId, u ast.Unit) string {
	if id == nil || id.Specs == nil || len(id.Specs.List) != 1 {
		return ""
	}
	named, ok := id.Specs.List[0].(*ast.NamedTypeSpec)
	if !ok {
		return ""
	}
	if ident, isIdent := named.Name.(*ast.Ident); isIdent {
		return ident.Text(u)
	}
	return ""
}
