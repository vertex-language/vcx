package sema

import (
	"fmt"
	"strings"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/types"
)

// Library builtins (e.g. `__builtin_fabs`, `__builtin_memcpy`): declared with
// extern "C" linkage and their corresponding library link names.

// Compact signature encoding: result and parameter type codes.
//
//	v void    i int      l long     ll long long   u unsigned
//	z size_t  d double   f float    L long double
//	p void*   P const void*   s char*   S const char*
//	w wchar_t* W const wchar_t*   c wchar_t
//	*i int*   *d double* *f float* *L long double*
var libraryBuiltins = map[string]string{}

// floating lists math functions providing double, float, and long double forms.
var floating = map[string]string{
	"acos": "d(d)", "asin": "d(d)", "atan": "d(d)", "cos": "d(d)", "sin": "d(d)", "tan": "d(d)",
	"acosh": "d(d)", "asinh": "d(d)", "atanh": "d(d)", "cosh": "d(d)", "sinh": "d(d)", "tanh": "d(d)",
	"exp": "d(d)", "exp2": "d(d)", "expm1": "d(d)", "log": "d(d)", "log10": "d(d)", "log1p": "d(d)",
	"log2": "d(d)", "logb": "d(d)", "cbrt": "d(d)", "sqrt": "d(d)", "erf": "d(d)", "erfc": "d(d)",
	"lgamma": "d(d)", "tgamma": "d(d)", "ceil": "d(d)", "floor": "d(d)", "nearbyint": "d(d)",
	"rint": "d(d)", "round": "d(d)", "trunc": "d(d)", "fabs": "d(d)",
	"atan2": "d(d,d)", "pow": "d(d,d)", "hypot": "d(d,d)", "fmod": "d(d,d)", "remainder": "d(d,d)",
	"copysign": "d(d,d)", "nextafter": "d(d,d)", "fdim": "d(d,d)", "fmax": "d(d,d)", "fmin": "d(d,d)",
	"frexp": "d(d,*i)", "ldexp": "d(d,i)", "modf": "d(d,*d)", "scalbn": "d(d,i)", "scalbln": "d(d,l)",
	"ilogb": "i(d)", "lrint": "l(d)", "lround": "l(d)", "llrint": "ll(d)", "llround": "ll(d)",
	"remquo": "d(d,d,*i)", "fma": "d(d,d,d)", "nexttoward": "d(d,L)",
}

func init() {
	for name, sig := range floating {
		libraryBuiltins[name] = sig
		libraryBuiltins[name+"f"] = retype(sig, "f")
		libraryBuiltins[name+"l"] = retype(sig, "L")
	}
	for name, sig := range map[string]string{
		"abs": "i(i)", "labs": "l(l)", "llabs": "ll(ll)",
		"memcpy": "p(p,P,z)", "memmove": "p(p,P,z)", "memset": "p(p,i,z)", "memcmp": "i(P,P,z)",
		"memchr": "p(P,i,z)", "char_memchr": "s(S,i,z)", "bzero": "v(p,z)",
		"strlen": "z(S)", "strcmp": "i(S,S)", "strncmp": "i(S,S,z)", "strchr": "s(S,i)",
		"strrchr": "s(S,i)", "strstr": "s(S,S)", "strpbrk": "s(S,S)",
		"wcslen": "z(W)", "wmemchr": "w(W,c,z)", "wmemcmp": "i(W,W,z)", "wmemcpy": "w(w,W,z)",
		"wmemmove": "w(w,W,z)", "wmemset": "w(w,c,z)",
		"abort": "v()", "alloca": "p(z)",
	} {
		libraryBuiltins[name] = sig
	}
}

// libraryName returns the library function name for a builtin.
func libraryName(name string) string {
	if name == "char_memchr" {
		return "memchr"
	}
	return name
}

// retype generates float or long double signatures from a double signature.
func retype(sig, to string) string {
	parts := splitSig(sig)
	for i, p := range parts {
		switch p {
		case "d":
			parts[i] = to
		case "*d":
			parts[i] = "*" + to
		}
	}
	return parts[0] + "(" + strings.Join(parts[1:], ",") + ")"
}

// splitSig is a signature as its result and then its parameters.
func splitSig(sig string) []string {
	open := strings.IndexByte(sig, '(')
	out := []string{sig[:open]}
	params := strings.TrimSuffix(sig[open+1:], ")")
	if params != "" {
		out = append(out, strings.Split(params, ",")...)
	}
	return out
}

// sigType is one letter of a signature as a type of the model.
func (a *Analyzer) sigType(letter string) types.Type {
	ptr := func(t types.Type) types.Type { return &types.Pointer{Elem: t} }
	cnst := func(t types.Type) types.Type { return types.Qualify(t, types.QConst) }
	switch letter {
	case "v":
		return types.Typ(types.Void)
	case "i":
		return types.Typ(types.Int)
	case "l":
		return types.Typ(types.Long)
	case "ll":
		return types.Typ(types.LongLong)
	case "u":
		return types.Typ(types.UInt)
	case "z":
		return a.sizeT()
	case "d":
		return types.Typ(types.Double)
	case "f":
		return types.Typ(types.Float)
	case "L":
		return types.Typ(types.LongDouble)
	case "p":
		return ptr(types.Typ(types.Void))
	case "P":
		return ptr(cnst(types.Typ(types.Void)))
	case "s":
		return ptr(types.Typ(types.Char))
	case "S":
		return ptr(cnst(types.Typ(types.Char)))
	case "w":
		return ptr(types.Typ(types.WChar))
	case "W":
		return ptr(cnst(types.Typ(types.WChar)))
	case "c":
		return types.Typ(types.WChar)
	}
	if strings.HasPrefix(letter, "*") {
		return ptr(a.sigType(letter[1:]))
	}
	return nil
}

// declareLibraryBuiltins enters every library builtin in the global scope.
func (a *Analyzer) declareLibraryBuiltins() {
	g := a.globalScope
	for name, sig := range libraryBuiltins {
		parts := splitSig(sig)
		ft := &types.Func{Ret: a.sigType(parts[0])}
		sym := &FuncSymbol{
			SymName:  "__builtin_" + name,
			LinkName: libraryName(name),
			FuncType: ft,
			SymScope: g,
			ExternC:  true,
		}
		for _, p := range parts[1:] {
			pt := a.sigType(p)
			ft.Params = append(ft.Params, types.Param{Type: pt})
			sym.Params = append(sym.Params, &VarSymbol{SymType: pt, IsParam: true})
		}
		g.Insert(sym)
	}
}

// gnuBuiltinCall types and handles GNU compiler builtins.
func (a *Analyzer) gnuBuiltinCall(name string, c *ast.CallExpr) (ExprInfo, bool) {
	base, ok := strings.CutPrefix(name, "__builtin_")
	if !ok {
		return ExprInfo{}, false
	}
	kind, known := builtinKinds[base]
	if !known {
		return ExprInfo{}, false
	}
	var infos []ExprInfo
	dependent := false
	for _, arg := range c.Args {
		info := a.CheckExpr(arg)
		infos = append(infos, info)
		dependent = dependent || isDependentExpr(info)
	}
	prv := func(t types.Type) ExprInfo { return ExprInfo{Type: t, ValCat: PrValue} }
	want := func(n int) bool {
		if len(c.Args) != n {
			a.errorAt(c.Pos(), name+" takes "+itoaSmall(n)+" arguments")
			return false
		}
		return true
	}
	if dependent && kind != builtinOperatorNew {
		return dependentExpr(), true
	}
	switch kind {
	case builtinClassify:
		want(1)
		return prv(types.Typ(types.Int)), true
	case builtinFPClassify:
		want(6)
		return prv(types.Typ(types.Int)), true
	case builtinCompare:
		want(2)
		return prv(types.Typ(types.Int)), true
	case builtinConstant:
		t := types.Typ(types.Double)
		switch {
		case strings.HasSuffix(base, "f"):
			t = types.Typ(types.Float)
		case strings.HasSuffix(base, "l"):
			t = types.Typ(types.LongDouble)
		}
		return prv(t), true
	case builtinBitCount:
		if strings.HasSuffix(base, "g") {
			if len(c.Args) != 1 && len(c.Args) != 2 {
				a.errorAt(c.Pos(), name+" takes one or two arguments")
			}
		} else {
			want(1)
		}
		return prv(types.Typ(types.Int)), true
	case builtinBswap:
		want(1)
		switch base {
		case "bswap16":
			return prv(types.Typ(types.UShort)), true
		case "bswap32":
			return prv(types.Typ(types.UInt)), true
		}
		return prv(types.Typ(types.ULongLong)), true
	case builtinOverflow:
		want(3)
		return prv(types.Typ(types.Bool)), true
	case builtinExpect:
		return prv(types.Typ(types.Long)), true
	case builtinConstantP:
		want(1)
		return prv(types.Typ(types.Int)), true
	case builtinTrap:
		return prv(types.Typ(types.Void)), true
	case builtinIdentity:
		if len(infos) == 0 {
			a.errorAt(c.Pos(), name+" takes an argument")
			return prv(types.Typ(types.Void)), true
		}
		return prv(types.RemoveReference(infos[0].Type)), true
	case builtinOperatorNew:
		fn := "operator new"
		if base == "operator_delete" {
			fn = "operator delete"
		}
		if dependent || a.dependentContext() {
			return dependentExpr(), true
		}
		var candidates []*FuncSymbol
		for _, sym := range LookupUnqualified(a.globalScope, fn) {
			if f, isFn := sym.(*FuncSymbol); isFn {
				candidates = append(candidates, f)
			}
		}
		var args []Argument
		for i, info := range infos {
			args = append(args, Argument{Type: info.Type, IsLValue: info.ValCat == LValue, NullConst: isNullConstant(c.Args[i], info)})
		}
		chosen, err := ResolveOverload(candidates, args)
		if err != nil || chosen == nil {
			a.errorAt(c.Pos(), name+": no "+fn+" takes these arguments")
			return prv(types.Typ(types.Void)), true
		}
		if a.info != nil {
			a.info.Calls[c] = chosen
			a.ensureInstantiated(chosen)
		}
		return prv(chosen.FuncType.Ret), true
	}
	return ExprInfo{}, false
}

type builtinKind uint8

const (
	builtinClassify    builtinKind = iota + 1 // isnan(x) and its kin
	builtinFPClassify                         // fpclassify(nan, inf, normal, subnormal, zero, x)
	builtinCompare                            // isgreater(x, y) and its kin
	builtinConstant                           // huge_val(), inf(), nan("")
	builtinBitCount                           // clz, ctz, popcount, parity
	builtinBswap                              // bswap16/32/64
	builtinOverflow                           // add_overflow(a, b, &r)
	builtinExpect                             // expect(x, v)
	builtinConstantP                          // constant_p(x)
	builtinTrap                               // trap(), verbose_trap(category, message)
	builtinIdentity                           // launder(p), assume_aligned(p, n)
	builtinOperatorNew                        // operator_new(args...), operator_delete(args...)
)

// builtinKinds are the expression builtins, by name without __builtin_.
var builtinKinds = map[string]builtinKind{
	"isnan": builtinClassify, "isinf": builtinClassify, "isfinite": builtinClassify,
	"isnormal": builtinClassify, "signbit": builtinClassify,
	"fpclassify": builtinFPClassify,
	"isgreater":  builtinCompare, "isgreaterequal": builtinCompare, "isless": builtinCompare,
	"islessequal": builtinCompare, "islessgreater": builtinCompare, "isunordered": builtinCompare,
	"huge_val": builtinConstant, "huge_valf": builtinConstant, "huge_vall": builtinConstant,
	"inf": builtinConstant, "inff": builtinConstant, "infl": builtinConstant,
	"nan": builtinConstant, "nanf": builtinConstant, "nanl": builtinConstant,
	"nans": builtinConstant, "nansf": builtinConstant, "nansl": builtinConstant,
	"clz": builtinBitCount, "clzl": builtinBitCount, "clzll": builtinBitCount, "clzg": builtinBitCount,
	"ctz": builtinBitCount, "ctzl": builtinBitCount, "ctzll": builtinBitCount, "ctzg": builtinBitCount,
	"popcount": builtinBitCount, "popcountl": builtinBitCount, "popcountll": builtinBitCount,
	"popcountg": builtinBitCount, "parity": builtinBitCount, "parityl": builtinBitCount,
	"parityll": builtinBitCount,
	"bswap16":  builtinBswap, "bswap32": builtinBswap, "bswap64": builtinBswap,
	"add_overflow": builtinOverflow, "sub_overflow": builtinOverflow, "mul_overflow": builtinOverflow,
	"expect": builtinExpect, "expect_with_probability": builtinExpect,
	"constant_p": builtinConstantP,
	"trap":       builtinTrap, "verbose_trap": builtinTrap,
	"launder": builtinIdentity, "assume_aligned": builtinIdentity,
	"operator_new": builtinOperatorNew, "operator_delete": builtinOperatorNew,
}

// ExpressionBuiltins returns all supported __builtin_* expression names.
func ExpressionBuiltins() []string {
	out := make([]string, 0, len(builtinKinds))
	for name := range builtinKinds {
		out = append(out, "__builtin_"+name)
	}
	return out
}

// LibraryBuiltins are the library builtins declareLibraryBuiltins enters.
func LibraryBuiltins() []string {
	out := make([]string, 0, len(libraryBuiltins))
	for name := range libraryBuiltins {
		out = append(out, "__builtin_"+name)
	}
	return out
}

func itoaSmall(n int) string { return string(rune('0' + n)) }

// builtinTemplates lists compiler-provided built-in templates.
var builtinTemplates = []string{"__make_integer_seq"}

// BuiltinTemplates are the builtin templates declareBuiltinTemplates enters.
func BuiltinTemplates() []string { return builtinTemplates }

// declareBuiltinTemplates enters every builtin template in the global scope.
func (a *Analyzer) declareBuiltinTemplates() {
	g := a.globalScope
	for _, name := range builtinTemplates {
		g.Insert(&TypeSymbol{
			SymName:  name,
			SymType:  &types.DependentType{Name: name},
			SymScope: g,
			Alias:    &AliasTemplate{Scope: g, Builtin: name},
		})
	}
}

// instantiateBuiltinTemplate names the type a builtin template means for
// its arguments.
func (a *Analyzer) instantiateBuiltinTemplate(name string, args []types.TemplateArg, at ast.Tok) types.Type {
	switch name {
	case "__make_integer_seq":
		// __make_integer_seq<TT, T, N> is TT<T, 0, 1, ..., N-1>.
		if len(args) != 3 || !args[0].IsType || !args[1].IsType || args[2].IsType {
			a.errorAt(at, "__make_integer_seq takes a class template, an integer type, and a length")
			return nil
		}
		var primary *RecordSymbol
		switch t := args[0].Type.(type) {
		case *types.TemplateRef:
			primary = a.globalScope.recordSymbol(t.Primary)
		case *types.Record:
			if rs := a.globalScope.recordSymbol(t); rs != nil && rs.ClassTemplate != nil {
				primary = rs
			}
		}
		if primary != nil && primary.TemplateOf != nil {
			primary = primary.TemplateOf
		}
		if primary == nil || primary.ClassTemplate == nil {
			a.errorAt(at, "the first argument of __make_integer_seq must be a class template")
			return nil
		}
		n := args[2].Val
		if n < 0 {
			a.errorAt(at, fmt.Sprintf("__make_integer_seq length %d is negative", n))
			return nil
		}
		seq := make([]types.TemplateArg, 0, n+1)
		seq = append(seq, args[1])
		for i := int64(0); i < n; i++ {
			seq = append(seq, types.TemplateArg{Val: i, ValType: args[1].Type})
		}
		inst := a.globalScope.instantiate()
		if inst == nil {
			return nil
		}
		return inst(primary, seq, at)
	}
	return nil
}
