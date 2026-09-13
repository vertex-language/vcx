package mangle

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vertex-language/vcx/types"
)

// itanium is one Itanium-scheme mangling in progress.
//
// The scheme's compression is a single table of substitution candidates:
// every prefix of a nested name and every type that is not a builtin,
// entered in the order it was first written, and written thereafter as
// `S_`, `S0_`, `S1_` and so on. The table is keyed here by the candidate's
// uncompressed spelling, which is the canonical form the ABI defines the
// table over -- two candidates are the same entry when they would have
// been written the same way.
//
// No oracle for this scheme exists on this machine, so it is written from
// the ABI document and its tests are examples the document gives.
type itanium struct {
	sb   strings.Builder
	subs []string
	err  error
}

func newItanium() *itanium { return &itanium{} }

func (m *itanium) fail(format string, args ...any) {
	if m.err == nil {
		m.err = fmt.Errorf("mangle: "+format, args...)
	}
}

func (m *itanium) function(f *Function) (string, error) {
	m.sb.WriteString("_Z")
	m.functionName(f)
	sig := f.Type
	if f.TemplateArgs != nil {
		// A specialization's encoding is the template's signature, with the return type
		// before parameters (except ctors, dtors, and conversions).
		if f.Pattern != nil {
			sig = f.Pattern
		}
		switch f.Kind {
		case Ctor, CtorBase, Dtor, DtorBase, DtorDeleting, Conversion:
		default:
			m.typ(sig.Ret)
		}
	}
	m.bareFunctionType(sig)
	return m.sb.String(), m.err
}

// thunk emits a non-virtual this-adjusting thunk: `_ZThn8_` followed by the function encoding.
func (m *itanium) thunk(f *Function, adjust int64) (string, error) {
	m.sb.WriteString("_ZTh")
	adjust = -adjust // the scheme writes what is added to `this`
	if adjust < 0 {
		m.sb.WriteString("n")
		adjust = -adjust
	}
	m.sb.WriteString(strconv.FormatInt(adjust, 10))
	m.sb.WriteString("_")
	m.functionName(f)
	m.bareFunctionType(f.Type)
	return m.sb.String(), m.err
}

func (m *itanium) variable(v *Variable) (string, error) {
	// An object at global scope is not mangled at all: `int g;` -> `g`.
	if len(v.Scopes) == 0 {
		return v.Name, nil
	}
	m.sb.WriteString("_ZN")
	m.prefix(v.Scopes)
	m.sourceName(v.Name)
	m.sb.WriteString("E")
	return m.sb.String(), m.err
}

func (m *itanium) vtable(rec *types.Record) (string, error) {
	m.sb.WriteString("_ZTV")
	m.className(recordScopes(rec))
	return m.sb.String(), m.err
}

// functionName is the <name> of a function: unscoped, or nested with the
// member's cv- and ref-qualifiers just inside the `N`.
func (m *itanium) functionName(f *Function) {
	if len(f.Scopes) == 0 {
		m.templateName("", f)
		m.templateArgs(f.TemplateArgs)
		return
	}
	if len(f.Scopes) == 1 && f.Scopes[0].Name == "std" && f.Scopes[0].Args == nil {
		// `St` is the one prefix that is not a nested name.
		m.sb.WriteString("St")
		m.templateName(m.rawPrefix(f.Scopes), f)
		m.templateArgs(f.TemplateArgs)
		return
	}
	m.sb.WriteString("N")
	if f.Member {
		m.cv(f.Type.Quals)
		switch f.Type.RefQual {
		case types.RefQualLValue:
			m.sb.WriteString("R")
		case types.RefQualRValue:
			m.sb.WriteString("O")
		}
	}
	m.prefix(f.Scopes)
	m.templateName(m.rawPrefix(f.Scopes), f)
	m.templateArgs(f.TemplateArgs)
	m.sb.WriteString("E")
}

// templateName writes a function's unqualified name, and for a template's
// specialization enters the template's name -- its prefix and its own name
// -- as a substitution candidate before the arguments that follow it.
func (m *itanium) templateName(prefix string, f *Function) {
	start := m.sb.Len()
	m.unqualifiedName(f)
	if f.TemplateArgs != nil {
		m.subs = append(m.subs, prefix+m.sb.String()[start:])
	}
}

// templateArgs writes `I<args>E` after a template's name. A value is an
// expr-primary, `L<type><value>E` -- `Lb0E` for false, `Li5E` for 5 --
// with a negative written after an `n`.
func (m *itanium) templateArgs(args []types.TemplateArg) {
	if args == nil {
		return
	}
	m.sb.WriteString("I")
	for _, a := range args {
		if pack, isPack := a.Type.(*types.Pack); a.IsType && isPack {
			// A pack's arguments are bracketed `J...E`, empty when the pack is.
			m.sb.WriteString("J")
			m.templateArgsInner(pack.Elems)
			m.sb.WriteString("E")
			continue
		}
		if a.IsType {
			m.typ(a.Type)
			continue
		}
		m.sb.WriteString("L")
		m.typ(a.ValType)
		m.sb.WriteString(m.rawValue(a.Val))
		m.sb.WriteString("E")
	}
	m.sb.WriteString("E")
}

// templateArgsInner writes arguments without the I...E around them.
func (m *itanium) templateArgsInner(args []types.TemplateArg) {
	for _, a := range args {
		if a.IsType {
			m.typ(a.Type)
			continue
		}
		m.sb.WriteString("L")
		m.typ(a.ValType)
		m.sb.WriteString(m.rawValue(a.Val))
		m.sb.WriteString("E")
	}
}

func (m *itanium) rawValue(v int64) string {
	if v < 0 {
		return "n" + strconv.FormatInt(-v, 10)
	}
	return strconv.FormatInt(v, 10)
}

// unqualifiedName is a function's own name: a source name, a constructor
// or destructor code, an operator code, or a conversion.
//
// A constructor is the complete-object one, `C1`, unless the base-object
// variant `C2` is asked for, and a destructor likewise `D1` or `D2` -- or
// `D0`, the deleting destructor.
func (m *itanium) unqualifiedName(f *Function) {
	switch f.Kind {
	case Ctor:
		m.sb.WriteString("C1")
	case CtorBase:
		m.sb.WriteString("C2")
	case Dtor:
		m.sb.WriteString("D1")
	case DtorBase:
		m.sb.WriteString("D2")
	case DtorDeleting:
		m.sb.WriteString("D0")
	case Conversion:
		m.sb.WriteString("cv")
		m.typ(f.Conv)
	default:
		if op, isOp := operators[f.Name]; isOp {
			arity := 0
			if f.Type != nil {
				arity = len(f.Type.Params)
			}
			if f.Member {
				arity++
			}
			if code, unary := unaryItanium[f.Name]; unary && arity == 1 {
				// Disambiguate unary operators (e.g. `&a` is `ad`, binary `a & b` is `an`).
				m.sb.WriteString(code)
				break
			}
			m.sb.WriteString(op.itanium)
		} else if strings.HasPrefix(f.Name, "operator") {
			m.fail("no Itanium code for %q", f.Name)
		} else {
			m.sourceName(f.Name)
		}
	}
}

func (m *itanium) sourceName(name string) {
	if name == "" {
		m.fail("an unnamed entity has no symbol")
		return
	}
	// A closure type has no linkage. It is mangled with $_ prefix followed by index.
	if name == "<lambda_invoker_cdecl>" {
		name = "__invoke"
	} else if n, ok := strings.CutPrefix(name, "<lambda_"); ok {
		if k, err := strconv.Atoi(strings.TrimSuffix(n, ">")); err == nil {
			name = "$_" + strconv.Itoa(k-1)
		}
	}
	m.sb.WriteString(strconv.Itoa(len(name)))
	m.sb.WriteString(name)
}

// prefix writes the enclosing scopes of a nested name, using the longest
// already-substitutable prefix and entering every longer one. A scope
// that is a specialization is two candidates: the template name, and the
// template name with its arguments.
func (m *itanium) prefix(scopes []Scope) {
	start := 0
	for i := len(scopes); i > 0; i-- {
		if m.substitute(m.rawPrefix(scopes[:i])) {
			start = i
			break
		}
	}
	for i := start; i < len(scopes); i++ {
		if i == 0 && scopes[0].Name == "std" && scopes[0].Args == nil {
			// `St` is written for `std` and is not itself a candidate;
			// `St` plus the next name is.
			m.sb.WriteString("St")
			continue
		}
		m.sourceName(scopes[i].Name)
		if scopes[i].Args != nil {
			bare := append(append([]Scope{}, scopes[:i]...), Scope{Name: scopes[i].Name})
			m.subs = append(m.subs, m.rawPrefix(bare))
			m.templateArgs(scopes[i].Args)
		}
		m.subs = append(m.subs, m.rawPrefix(scopes[:i+1]))
	}
}

// rawPrefix is the uncompressed spelling of a prefix, the substitution key.
func (m *itanium) rawPrefix(scopes []Scope) string {
	var sb strings.Builder
	for _, s := range scopes {
		sb.WriteString(strconv.Itoa(len(s.Name)))
		sb.WriteString(s.Name)
		if s.Args != nil {
			sb.WriteString("I")
			for _, a := range s.Args {
				if a.IsType {
					sb.WriteString(m.raw(a.Type))
				} else {
					sb.WriteString("L" + m.raw(a.ValType) + m.rawValue(a.Val) + "E")
				}
			}
			sb.WriteString("E")
		}
	}
	return sb.String()
}

// substitute writes `S_`-style reference to an existing candidate and
// reports whether there was one.
func (m *itanium) substitute(key string) bool {
	for i, seen := range m.subs {
		if seen != key {
			continue
		}
		m.sb.WriteString("S")
		if i > 0 {
			m.sb.WriteString(strings.ToUpper(strconv.FormatInt(int64(i-1), 36)))
		}
		m.sb.WriteString("_")
		return true
	}
	return false
}

// className writes a class or enum as a type: a source name at global
// scope, a nested name otherwise. Both are substitution candidates, and a
// nested one's prefixes are too. The last scope is the class itself.
func (m *itanium) className(scopes []Scope) {
	key := m.rawPrefix(scopes)
	if m.substitute(key) {
		return
	}
	last := scopes[len(scopes)-1]
	std := len(scopes) == 2 && scopes[0].Name == "std" && scopes[0].Args == nil
	if len(scopes) == 1 || std {
		// A name directly in std is an <unscoped-name> prefixed with `St`.
		head := []Scope{{Name: last.Name}}
		if std {
			m.sb.WriteString("St")
			head = []Scope{scopes[0], {Name: last.Name}}
		}
		m.sourceName(last.Name)
		if last.Args != nil {
			m.subs = append(m.subs, m.rawPrefix(head))
			m.templateArgs(last.Args)
		}
	} else {
		m.sb.WriteString("N")
		m.prefix(scopes)
		m.sb.WriteString("E")
		return // prefix entered every candidate, the class included
	}
	m.subs = append(m.subs, key)
}

// bareFunctionType is the parameter list, with `v` for an empty one and
// `z` for the ellipsis. The return type is not part of it for an ordinary
// function; it is for a template instance, which is a case for later.
func (m *itanium) bareFunctionType(fn *types.Func) {
	if len(fn.Params) == 0 && !fn.Variadic {
		m.sb.WriteString("v")
		return
	}
	for _, p := range fn.Params {
		if p.Pack {
			// A function parameter pack is a pack expansion prefixed with `Dp`.
			t := adjustParamItanium(p.Type)
			key := "Dp" + m.raw(t)
			if m.substitute(key) {
				continue
			}
			m.sb.WriteString("Dp")
			m.typ(t)
			m.subs = append(m.subs, key)
			continue
		}
		m.typ(adjustParamItanium(p.Type))
	}
	if fn.Variadic {
		m.sb.WriteString("z")
	}
}

// cv writes cv-qualifiers, in the ABI's order: volatile before const.
func (m *itanium) cv(q types.Qual) {
	if q&types.QVolatile != 0 {
		m.sb.WriteString("V")
	}
	if q&types.QConst != 0 {
		m.sb.WriteString("K")
	}
}

// typ writes one type, through the substitution table for anything that
// is not a builtin.
func (m *itanium) typ(t types.Type) {
	if t == nil {
		m.sb.WriteString("v")
		return
	}
	if b, isBasic := t.(*types.Basic); isBasic {
		m.sb.WriteString(m.basic(b.K))
		return
	}
	key := m.raw(t)
	if m.substitute(key) {
		return
	}
	switch t := t.(type) {
	case *types.Qualified:
		// A qualified type is a candidate of its own, over and above the
		// unqualified one: `RKi` enters `Ki` and then `RKi`.
		m.cv(t.Q)
		m.typ(t.T)
	case *types.Pointer:
		m.sb.WriteString("P")
		m.typ(t.Elem)
	case *types.LValueReference:
		m.sb.WriteString("R")
		m.typ(t.Elem)
	case *types.RValueReference:
		m.sb.WriteString("O")
		m.typ(t.Elem)
	case *types.Array:
		m.sb.WriteString("A")
		if !t.Incomplete {
			m.sb.WriteString(strconv.FormatInt(t.Len, 10))
		}
		m.sb.WriteString("_")
		m.typ(t.Elem)
	case *types.MemberPointer:
		m.sb.WriteString("M")
		m.typ(t.Class)
		m.typ(t.Elem)
	case *types.Func:
		m.sb.WriteString("F")
		m.typ(t.Ret)
		m.bareFunctionType(t)
		m.sb.WriteString("E")
	case *types.Record:
		m.className(recordScopes(t))
		return // className entered the candidate itself
	case *types.Enum:
		m.className(append(Scopes(t.Scopes...), Scope{Name: t.Name}))
		return
	case *types.TemplateParam:
		// Reference to the function template parameter (`T_`, `T0_`, etc.).
		m.sb.WriteString(templateParamRef(t))
	case *types.TemplateSpecialization:
		// A class template named with arguments still open, `Box<T>`: the
		// template's name and the arguments as written.
		if scopes, ok := specializationScopes(t); ok {
			m.className(scopes)
			return
		}
		if t.Name == "enable_if_t" || t.Name == "__enable_if_t" {
			if len(t.Args) >= 2 && t.Args[1].IsType && t.Args[1].Type != nil {
				m.typ(t.Args[1].Type)
				return
			}
			if len(t.Args) == 1 {
				m.typ(types.Typ(types.Void))
				return
			}
		}
		if t.Type != nil && t.Type.Kind() != types.DependentKind {
			m.typ(t.Type)
			return
		}
		m.fail("no Itanium spelling for %s", t)
		return
	default:
		m.fail("no Itanium spelling for %s", t)
		return
	}
	m.subs = append(m.subs, key)
}

// specializationScopes is a dependent class template specialization as a
// class name: the primary template's scopes, and its name with the
// arguments. An alias template has no class to name and is refused.
func specializationScopes(t *types.TemplateSpecialization) ([]Scope, bool) {
	rec, ok := types.Unqualify(t.Type).(*types.Record)
	if !ok || rec == nil {
		return nil, false
	}
	return append(Scopes(rec.Scopes...), Scope{Name: rec.Name, Args: t.Args}), true
}

func templateParamRef(t *types.TemplateParam) string {
	if t.Index == 0 {
		return "T_"
	}
	return "T" + strconv.Itoa(t.Index-1) + "_"
}

// raw is a type's spelling with no substitutions applied, the key its
// candidate is entered under.
func (m *itanium) raw(t types.Type) string {
	switch t := t.(type) {
	case nil:
		return "v"
	case *types.Basic:
		return m.basic(t.K)
	case *types.Qualified:
		var sb strings.Builder
		if t.Q&types.QVolatile != 0 {
			sb.WriteString("V")
		}
		if t.Q&types.QConst != 0 {
			sb.WriteString("K")
		}
		sb.WriteString(m.raw(t.T))
		return sb.String()
	case *types.Pointer:
		return "P" + m.raw(t.Elem)
	case *types.LValueReference:
		return "R" + m.raw(t.Elem)
	case *types.RValueReference:
		return "O" + m.raw(t.Elem)
	case *types.Array:
		n := ""
		if !t.Incomplete {
			n = strconv.FormatInt(t.Len, 10)
		}
		return "A" + n + "_" + m.raw(t.Elem)
	case *types.MemberPointer:
		return "M" + m.raw(t.Class) + m.raw(t.Elem)
	case *types.Func:
		var sb strings.Builder
		sb.WriteString("F")
		sb.WriteString(m.raw(t.Ret))
		if len(t.Params) == 0 && !t.Variadic {
			sb.WriteString("v")
		}
		for _, p := range t.Params {
			sb.WriteString(m.raw(adjustParamItanium(p.Type)))
		}
		if t.Variadic {
			sb.WriteString("z")
		}
		sb.WriteString("E")
		return sb.String()
	case *types.Record:
		return m.rawPrefix(recordScopes(t))
	case *types.Enum:
		return m.rawPrefix(append(Scopes(t.Scopes...), Scope{Name: t.Name}))
	case *types.TemplateParam:
		return templateParamRef(t)
	case *types.TemplateSpecialization:
		if scopes, ok := specializationScopes(t); ok {
			return m.rawPrefix(scopes)
		}
		if t.Name == "enable_if_t" || t.Name == "__enable_if_t" {
			if len(t.Args) >= 2 && t.Args[1].IsType && t.Args[1].Type != nil {
				return m.raw(t.Args[1].Type)
			}
			if len(t.Args) == 1 {
				return "v"
			}
		}
		if t.Type != nil && t.Type.Kind() != types.DependentKind {
			return m.raw(t.Type)
		}
	}
	return fmt.Sprintf("<%s>", t)
}

func (m *itanium) basic(k types.Kind) string {
	switch k {
	case types.Void:
		return "v"
	case types.Bool:
		return "b"
	case types.Char:
		return "c"
	case types.SChar:
		return "a"
	case types.UChar:
		return "h"
	case types.Char8:
		return "Du"
	case types.Char16:
		return "Ds"
	case types.Char32:
		return "Di"
	case types.WChar:
		return "w"
	case types.Short:
		return "s"
	case types.UShort:
		return "t"
	case types.Int:
		return "i"
	case types.UInt:
		return "j"
	case types.Long:
		return "l"
	case types.ULong:
		return "m"
	case types.LongLong:
		return "x"
	case types.ULongLong:
		return "y"
	case types.Int128:
		return "n"
	case types.UInt128:
		return "o"
	case types.Float:
		return "f"
	case types.Double:
		return "d"
	case types.LongDouble:
		return "e"
	case types.NullptrKind:
		return "Dn"
	}
	m.fail("no Itanium spelling for the basic type %s", types.Typ(k))
	return "?"
}

// adjustParamItanium adjusts a parameter type: arrays and functions decay to pointers,
// and top-level cv-qualifiers are dropped.
func adjustParamItanium(t types.Type) types.Type {
	switch u := types.Unqualify(t).(type) {
	case *types.Array:
		return &types.Pointer{Elem: u.Elem}
	case *types.Func:
		return &types.Pointer{Elem: u}
	}
	return types.Unqualify(t)
}

// unaryItanium is the code of each operator whose unary form is spelled
// apart from its binary one.
var unaryItanium = map[string]string{
	"operator&": "ad",
	"operator-": "ng",
	"operator+": "ps",
	"operator*": "de",
}
