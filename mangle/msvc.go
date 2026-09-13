package mangle

import (
	"fmt"
	"strings"

	"github.com/vertex-language/vcx/types"
)

// msvc tracks the state of MSVC ABI symbol mangling, including identifier and parameter back-references.
type msvc struct {
	sb    strings.Builder
	names []string     // identifier back-references, in slot order
	args  []types.Type // parameter-type back-references, in slot order
	err   error
}

func newMSVC() *msvc { return &msvc{} }

func (m *msvc) fail(format string, args ...any) {
	if m.err == nil {
		m.err = fmt.Errorf("mangle: "+format, args...)
	}
}

func (m *msvc) function(f *Function) (string, error) {
	m.sb.WriteString("?")
	m.unqualifiedName(f)
	m.scopes(f.Scopes)
	m.sb.WriteString("@")

	m.functionClass(f)
	if f.Member {
		// Non-static member implicit object parameter: 64-bit 'E', ref-qualifier, cv-qualifiers.
		m.sb.WriteString("E")
		switch f.Type.RefQual {
		case types.RefQualLValue:
			m.sb.WriteString("G")
		case types.RefQualRValue:
			m.sb.WriteString("H")
		}
		m.cv(f.Type.Quals)
	}
	// Calling convention: __cdecl ('A').
	m.sb.WriteString("A")

	// Return type or '@' for ctor/dtor.
	switch f.Kind {
	case Ctor, Dtor:
		m.sb.WriteString("@")
	default:
		m.result(f.Type.Ret)
	}

	m.params(f.Type)

	// Exception specification ('Z').
	m.sb.WriteString("Z")
	return m.sb.String(), m.err
}

// thunk mangles an MSVC adjustor thunk symbol.
func (m *msvc) thunk(f *Function, adjust int64) (string, error) {
	m.sb.WriteString("?")
	m.unqualifiedName(f)
	m.scopes(f.Scopes)
	m.sb.WriteString("@W")
	m.number(adjust)
	m.sb.WriteString("E")
	m.cv(f.Type.Quals)
	m.sb.WriteString("A")
	switch f.Kind {
	case Ctor, Dtor:
		m.sb.WriteString("@")
	default:
		m.result(f.Type.Ret)
	}
	m.params(f.Type)
	m.sb.WriteString("Z")
	return m.sb.String(), m.err
}

func (m *msvc) variable(v *Variable) (string, error) {
	m.sb.WriteString("?")
	m.ident(v.Name)
	m.scopes(v.Scopes)
	m.sb.WriteString("@")

	// The storage class: a static data member by access, or `3` for a
	// namespace-scope object.
	switch {
	case v.Static && v.Access == types.AccessPrivate:
		m.sb.WriteString("0")
	case v.Static && v.Access == types.AccessProtected:
		m.sb.WriteString("1")
	case v.Static:
		m.sb.WriteString("2")
	default:
		m.sb.WriteString("3")
	}

	quals, t := splitQuals(v.Type)
	switch t := t.(type) {
	case *types.Array:
		// An array object is written as a pointer to its element, with no
		// `E` -- cl writes `?a@@3PAHA` for `int a[3]`, where a pointer
		// variable would have been `?p@@3PEAHEA`.
		elemQuals, elem := splitQuals(t.Elem)
		m.pointerCV(elemQuals)
		m.cv(elemQuals)
		m.typ(elem)
		m.sb.WriteString("A")
	case *types.Pointer:
		// Pointer variable mangling includes pointee cv-qualifiers.
		m.typ(v.Type)
		m.sb.WriteString("E")
		pointeeQuals, _ := splitQuals(t.Elem)
		m.cv(pointeeQuals)
	case *types.LValueReference:
		m.typ(v.Type)
		m.sb.WriteString("E")
		pointeeQuals, _ := splitQuals(t.Elem)
		m.cv(pointeeQuals)
	case *types.RValueReference:
		m.typ(v.Type)
		m.sb.WriteString("E")
		pointeeQuals, _ := splitQuals(t.Elem)
		m.cv(pointeeQuals)
	case *types.MemberPointer:
		m.typ(v.Type)
		m.sb.WriteString("E")
		pointeeQuals, _ := splitQuals(t.Elem)
		m.memberCV(pointeeQuals)
	default:
		m.typ(t)
		m.cv(quals)
	}
	return m.sb.String(), m.err
}

func (m *msvc) vftable(rec *types.Record, base []*types.Record) (string, error) {
	// Virtual function table: ??_7<class>6B<bases>@
	m.sb.WriteString("??_7")
	m.className(rec)
	m.sb.WriteString("6B")
	for _, b := range base {
		// Each base is a name of its own with its own terminator:
		// `??_7Labelled@@6BShape@@@`.
		m.className(b)
	}
	m.sb.WriteString("@")
	return m.sb.String(), m.err
}

func (m *msvc) vbtable(rec *types.Record, base []*types.Record) (string, error) {
	m.sb.WriteString("??_8")
	m.className(rec)
	m.sb.WriteString("7B")
	for _, b := range base {
		m.className(b)
	}
	m.sb.WriteString("@")
	return m.sb.String(), m.err
}

// className writes a class's qualified name and its terminator: the class
// itself first, with its template arguments if it is a specialization,
// then the scopes enclosing it innermost first.
func (m *msvc) className(rec *types.Record) {
	m.scope(Scope{Name: rec.Name, Args: rec.TemplateArgs})
	m.scopes(Scopes(rec.Scopes...))
	m.sb.WriteString("@")
}

// unqualifiedName is the function's own name: an identifier, or one of the
// special codes for a constructor, destructor, operator or conversion.
// The codes take no back-reference slot; identifiers do.
//
// A specialization of a function template is `?$name@args@` -- the name
// and the arguments as one unit, which takes one slot together (see
// templateName).
func (m *msvc) unqualifiedName(f *Function) {
	if f.TemplateArgs != nil {
		// A specialization of a member template keeps the special code
		// for its name: a constructor template's instance is `?$?0`
		// and the arguments, `??$?0N@P@@QEAA@AEBN@Z` for P::P<double>.
		switch f.Kind {
		case Ctor:
			m.templateCode("?0", f.TemplateArgs)
		case Dtor:
			m.templateCode("?1", f.TemplateArgs)
		case Conversion:
			m.templateCode("?B", f.TemplateArgs)
		default:
			if op, isOp := operators[f.Name]; isOp {
				m.templateCode(op.msvc, f.TemplateArgs)
			} else {
				m.templateName(f.Name, f.TemplateArgs)
			}
		}
		return
	}
	switch f.Kind {
	case Ctor:
		m.sb.WriteString("?0")
	case Dtor:
		m.sb.WriteString("?1")
	case DeletingDtor:
		m.sb.WriteString("?_E")
	case ScalarDeletingDtor:
		m.sb.WriteString("?_G")
	case CtorBase, DtorBase, DtorDeleting:
		m.fail("the Microsoft ABI has no base-object or D0 structor variant")
	case Conversion:
		m.sb.WriteString("?B")
	default:
		if op, isOp := operators[f.Name]; isOp {
			m.sb.WriteString(op.msvc)
		} else if strings.HasPrefix(f.Name, "operator") {
			m.fail("no Microsoft code for %q", f.Name)
		} else {
			m.ident(f.Name)
		}
	}
}

// scopes writes the enclosing scopes innermost first, which is the order
// the scheme reads a qualified name in.
func (m *msvc) scopes(scopes []Scope) {
	for i := len(scopes) - 1; i >= 0; i-- {
		m.scope(scopes[i])
	}
}

// scope writes one scope: a plain identifier, or a template name with its
// arguments.
func (m *msvc) scope(s Scope) {
	if s.Args != nil {
		m.templateName(s.Name, s.Args)
		return
	}
	m.ident(s.Name)
}

// templateName writes `?$name@args@`, the spelling of a specialization,
// as one back-reference unit.
//
// The arguments are mangled in a fresh back-reference context: cl starts
// both tables over inside a template name and restores them after, so
// that `?$Box@H@` reads the same wherever it appears. The whole spelling
// then takes one slot in the outer table, as an identifier would.
func (m *msvc) templateName(name string, args []types.TemplateArg) {
	spelled := m.spellTemplate(name, args, false)
	for i, seen := range m.names {
		if seen == spelled {
			m.sb.WriteByte(byte('0' + i))
			return
		}
	}
	if len(m.names) < 10 {
		m.names = append(m.names, spelled)
	}
	m.sb.WriteString(spelled)
}

// templateCode is templateName for a name that is a special code rather
// than an identifier: the code takes no back-reference slot of its own,
// but the whole `?$code@args@` takes one, as any template name does.
func (m *msvc) templateCode(code string, args []types.TemplateArg) {
	spelled := m.spellTemplate(code, args, true)
	for i, seen := range m.names {
		if seen == spelled {
			m.sb.WriteByte(byte('0' + i))
			return
		}
	}
	if len(m.names) < 10 {
		m.names = append(m.names, spelled)
	}
	m.sb.WriteString(spelled)
}

// spellTemplate is `?$name@args@` written out in full, in a context of
// its own; code says the name is a special code, written as it is.
func (m *msvc) spellTemplate(name string, args []types.TemplateArg, code bool) string {
	inner := &msvc{}
	inner.sb.WriteString("?$")
	if code {
		inner.sb.WriteString(name)
	} else {
		inner.ident(name)
	}
	for _, a := range args {
		inner.templateArg(a)
	}
	inner.sb.WriteString("@")
	if inner.err != nil && m.err == nil {
		m.err = inner.err
	}
	return inner.sb.String()
}

// templateArg writes one argument. The context is the template name's
// own, so a class argument enters that context's table and not the
// enclosing symbol's; a basic type is its letter. A value is `$0` and the
// number: `?$integral_constant@_N$0A@@` is integral_constant<bool, false>.
func (m *msvc) templateArg(a types.TemplateArg) {
	if !a.IsType {
		m.sb.WriteString("$0")
		m.number(a.Val)
		return
	}
	// A parameter pack's arguments are written one after another as if
	// each were its own; an empty pack is `$$V`. Measured:
	// `count<int,double,int &>` is `??$count@HNAEAH@@`, `count<>` is
	// `??$count@$$V@@`.
	if ref, isRef := a.Type.(*types.TemplateRef); isRef {
		// A template as an argument: `$$Y` and the template's name.
		// Unmeasured against cl -- refused until it is.
		m.fail("a template template argument (%s) has no measured spelling yet", ref.Name)
		return
	}
	if pack, isPack := a.Type.(*types.Pack); isPack {
		if len(pack.Elems) == 0 {
			m.sb.WriteString("$$V")
			return
		}
		for _, e := range pack.Elems {
			m.templateArg(e)
		}
		return
	}
	t := a.Type
	quals, bare := splitQuals(t)
	if quals != 0 {
		// A cv-qualified argument is spelled with the `$$C` escape, since
		// it has no pointer letter to carry the qualifiers.
		if _, isPtr := bare.(*types.Pointer); !isPtr {
			m.sb.WriteString("$$C")
			m.cv(quals)
			m.typ(bare)
			return
		}
	}
	m.typ(t)
}

// ident writes one identifier and its terminator, or the digit of the slot
// it was first written in.
func (m *msvc) ident(name string) {
	if name == "" {
		m.fail("an unnamed entity has no symbol")
		return
	}
	for i, seen := range m.names {
		if seen == name {
			m.sb.WriteByte(byte('0' + i))
			return
		}
	}
	if len(m.names) < 10 {
		m.names = append(m.names, name)
	}
	m.sb.WriteString(name)
	m.sb.WriteString("@")
}

// functionClass is the letter that says what kind of function this is:
// its access and whether it is static or virtual for a member, `Y` for a
// free function.
func (m *msvc) functionClass(f *Function) {
	if !f.Member && !f.Static {
		m.sb.WriteString("Y")
		return
	}
	// The table is three rows of six: private, protected, public; and in
	// each row plain, static, virtual, with a far variant of each that no
	// 64-bit program uses.
	var row byte
	switch f.Access {
	case types.AccessPrivate:
		row = 'A'
	case types.AccessProtected:
		row = 'I'
	default:
		row = 'Q'
	}
	switch {
	case f.Static:
		row += 2
	case f.Virtual:
		row += 4
	}
	m.sb.WriteByte(row)
}

// cv is the cv-qualifier letter used on a pointee, `this`, or a variable.
func (m *msvc) cv(q types.Qual) {
	switch {
	case q&types.QConst != 0 && q&types.QVolatile != 0:
		m.sb.WriteString("D")
	case q&types.QVolatile != 0:
		m.sb.WriteString("C")
	case q&types.QConst != 0:
		m.sb.WriteString("B")
	default:
		m.sb.WriteString("A")
	}
}

// pointerCV is the letter that opens a pointer type, which carries the
// pointer's own cv-qualifiers: `int *const` is `Q` where `int *` is `P`.
func (m *msvc) pointerCV(q types.Qual) {
	switch {
	case q&types.QConst != 0 && q&types.QVolatile != 0:
		m.sb.WriteString("S")
	case q&types.QVolatile != 0:
		m.sb.WriteString("R")
	case q&types.QConst != 0:
		m.sb.WriteString("Q")
	default:
		m.sb.WriteString("P")
	}
}

// result writes a return type. A class or enum returned by value gets a
// `?` and a cv letter first: cl writes `?f@@YA?AUS@@XZ` for `S f()`.
func (m *msvc) result(t types.Type) {
	if t == nil {
		m.sb.WriteString("X")
		return
	}
	quals, bare := splitQuals(t)
	switch bare.(type) {
	case *types.Record, *types.Enum:
		m.sb.WriteString("?")
		m.cv(quals)
		m.typ(bare)
	default:
		m.typ(bare)
	}
}

// params writes the parameter list ('X' for empty, terminated by '@' or 'Z' for variadic).
func (m *msvc) params(ft *types.Func) {
	if len(ft.Params) == 0 {
		if ft.Variadic {
			m.sb.WriteString("Z")
		} else {
			m.sb.WriteString("X")
		}
		return
	}
	for _, p := range ft.Params {
		m.param(p.Type)
	}
	if ft.Variadic {
		m.sb.WriteString("Z")
	} else {
		m.sb.WriteString("@")
	}
}

// param writes one parameter type, utilizing the type back-reference table.
func (m *msvc) param(t types.Type) {
	t = adjustParam(t)

	// Single-letter basic types are not back-referenced.
	if b, isBasic := t.(*types.Basic); isBasic && len(m.basic(b.K)) == 1 {
		m.sb.WriteString(m.basic(b.K))
		return
	}
	for i, seen := range m.args {
		if identical(seen, t) {
			m.sb.WriteByte(byte('0' + i))
			return
		}
	}
	if len(m.args) < 10 {
		m.args = append(m.args, t)
	}
	m.typ(t)
}

// adjustParam adjusts a parameter type for mangling.
func adjustParam(t types.Type) types.Type {
	quals, bare := splitQuals(t)
	switch u := bare.(type) {
	case *types.Array:
		return &types.Qualified{Q: types.QConst, T: &types.Pointer{Elem: u.Elem}}
	case *types.Func:
		return &types.Pointer{Elem: u}
	case *types.Pointer, *types.MemberPointer:
		if quals != 0 {
			return &types.Qualified{Q: quals, T: bare}
		}
		return bare
	}
	return bare
}

// typ writes a type in any position but a parameter list's top level.
func (m *msvc) typ(t types.Type) {
	if t == nil {
		m.sb.WriteString("X")
		return
	}
	quals, bare := splitQuals(t)

	switch t := bare.(type) {
	case *types.Basic:
		m.sb.WriteString(m.basic(t.K))

	case *types.Pointer:
		m.pointerCV(quals)
		m.pointee(t.Elem)

	case *types.LValueReference:
		m.sb.WriteString("A")
		m.pointee(t.Elem)

	case *types.RValueReference:
		m.sb.WriteString("$$Q")
		m.pointee(t.Elem)

	case *types.MemberPointer:
		m.pointerCV(quals)
		m.memberPointee(t)

	case *types.Array:
		// Array type: 'Y' <dimension count> <dimensions...> <element>.
		var dims []int64
		var elem types.Type = t
		for {
			arr, isArr := elem.(*types.Array)
			if !isArr {
				break
			}
			if arr.Incomplete {
				m.fail("an array of unknown bound has no Microsoft spelling here")
				return
			}
			dims = append(dims, arr.Len)
			elem = arr.Elem
		}
		m.sb.WriteString("Y")
		m.number(int64(len(dims)))
		for _, d := range dims {
			m.number(d)
		}
		elemQuals, elemBare := splitQuals(elem)
		if elemQuals != 0 {
			m.sb.WriteString("$$C")
			m.cv(elemQuals)
		}
		m.typ(elemBare)

	case *types.Record:
		switch t.Tag {
		case types.TagClass:
			m.sb.WriteString("V")
		case types.TagUnion:
			m.sb.WriteString("T")
		default:
			m.sb.WriteString("U")
		}
		m.className(t)

	case *types.Enum:
		// 'W4' for enum.
		m.sb.WriteString("W4")
		m.ident(t.Name)
		m.scopes(Scopes(t.Scopes...))
		m.sb.WriteString("@")

	case *types.Func:
		// Bare function type prefix '6'.
		m.sb.WriteString("6")
		m.functionType(t)

	default:
		m.fail("no Microsoft spelling for %s", t)
	}
}

// pointee writes the target of a pointer or reference.
func (m *msvc) pointee(t types.Type) {
	quals, bare := splitQuals(t)
	if fn, isFunc := bare.(*types.Func); isFunc {
		m.sb.WriteString("6")
		m.functionType(fn)
		return
	}
	m.sb.WriteString("E")
	m.cv(quals)
	m.typ(t)
}

// memberPointee is the part of a pointer-to-member after its pointer
// letter: `8` + class + function type for a member function, or the
// member-cv letter + class + type for a data member.
func (m *msvc) memberPointee(mp *types.MemberPointer) {
	class, isRec := types.Unqualify(mp.Class).(*types.Record)
	if !isRec {
		m.fail("a pointer to member of a non-class")
		return
	}
	quals, bare := splitQuals(mp.Elem)
	if fn, isFunc := bare.(*types.Func); isFunc {
		m.sb.WriteString("8")
		m.className(class)
		m.sb.WriteString("E")
		m.cv(fn.Quals)
		m.sb.WriteString("A")
		m.result(fn.Ret)
		m.params(fn)
		m.sb.WriteString("Z")
		return
	}
	m.sb.WriteString("E")
	m.memberCV(quals)
	m.className(class)
	m.typ(bare)
}

// memberCV is the data-member row of the cv table, `Q R S T` where the
// plain row is `A B C D`.
func (m *msvc) memberCV(q types.Qual) {
	switch {
	case q&types.QConst != 0 && q&types.QVolatile != 0:
		m.sb.WriteString("T")
	case q&types.QVolatile != 0:
		m.sb.WriteString("S")
	case q&types.QConst != 0:
		m.sb.WriteString("R")
	default:
		m.sb.WriteString("Q")
	}
}

// functionType is a function's type where it is a type and not a symbol:
// calling convention, return, parameters, exception specification.
func (m *msvc) functionType(fn *types.Func) {
	m.sb.WriteString("A")
	m.result(fn.Ret)
	m.params(fn)
	m.sb.WriteString("Z")
}

// number is the scheme's integer encoding: 0 is `A@`, 1 through 10 are the
// digits 0 through 9, and anything larger is its hexadecimal digits with
// `A` for 0 through `P` for 15, closed by `@`. A negative number takes a
// `?` first.
func (m *msvc) number(n int64) {
	if n < 0 {
		m.sb.WriteString("?")
		n = -n
	}
	if n == 0 {
		m.sb.WriteString("A@")
		return
	}
	if n <= 10 {
		m.sb.WriteByte(byte('0' + n - 1))
		return
	}
	var hex []byte
	for n > 0 {
		hex = append([]byte{byte('A' + n%16)}, hex...)
		n /= 16
	}
	m.sb.Write(hex)
	m.sb.WriteString("@")
}

func (m *msvc) basic(k types.Kind) string {
	switch k {
	case types.Void:
		return "X"
	case types.Bool:
		return "_N"
	case types.Char:
		return "D"
	case types.SChar:
		return "C"
	case types.UChar:
		return "E"
	case types.Short:
		return "F"
	case types.UShort:
		return "G"
	case types.Int:
		return "H"
	case types.UInt:
		return "I"
	case types.Long:
		return "J"
	case types.ULong:
		return "K"
	case types.LongLong:
		return "_J"
	case types.ULongLong:
		return "_K"
	case types.Int128:
		return "_L"
	case types.UInt128:
		return "_M"
	case types.Float:
		return "M"
	case types.Double:
		return "N"
	case types.LongDouble:
		return "O"
	case types.WChar:
		return "_W"
	case types.Char8:
		return "_Q"
	case types.Char16:
		return "_S"
	case types.Char32:
		return "_U"
	case types.NullptrKind:
		return "$$T"
	}
	m.fail("no Microsoft spelling for the basic type %s", types.Typ(k))
	return "?"
}

// splitQuals separates a type's top-level cv-qualifiers from the type.
func splitQuals(t types.Type) (types.Qual, types.Type) {
	if q, isQ := t.(*types.Qualified); isQ {
		inner, bare := splitQuals(q.T)
		return q.Q | inner, bare
	}
	return 0, t
}

// identical is type identity with every qualifier at every level, which
// is what the back-reference table keys on. The types' own Equal is not
// that: it ignores the qualifiers of its argument, which made `void *`
// and `const void *` one entry and cl says they are two.
func identical(a, b types.Type) bool {
	qa, ba := splitQuals(a)
	qb, bb := splitQuals(b)
	if qa != qb {
		return false
	}
	switch x := ba.(type) {
	case *types.Basic:
		y, ok := bb.(*types.Basic)
		return ok && x.K == y.K
	case *types.Pointer:
		y, ok := bb.(*types.Pointer)
		return ok && identical(x.Elem, y.Elem)
	case *types.LValueReference:
		y, ok := bb.(*types.LValueReference)
		return ok && identical(x.Elem, y.Elem)
	case *types.RValueReference:
		y, ok := bb.(*types.RValueReference)
		return ok && identical(x.Elem, y.Elem)
	case *types.Array:
		y, ok := bb.(*types.Array)
		return ok && x.Len == y.Len && x.Incomplete == y.Incomplete && identical(x.Elem, y.Elem)
	case *types.MemberPointer:
		y, ok := bb.(*types.MemberPointer)
		return ok && identical(x.Class, y.Class) && identical(x.Elem, y.Elem)
	case *types.Func:
		y, ok := bb.(*types.Func)
		if !ok || len(x.Params) != len(y.Params) || x.Variadic != y.Variadic || x.Quals != y.Quals || x.RefQual != y.RefQual {
			return false
		}
		if !identical(x.Ret, y.Ret) {
			return false
		}
		for i := range x.Params {
			if !identical(x.Params[i].Type, y.Params[i].Type) {
				return false
			}
		}
		return true
	case *types.Record:
		y, ok := bb.(*types.Record)
		return ok && x == y
	case *types.Enum:
		y, ok := bb.(*types.Enum)
		return ok && x == y
	case nil:
		return bb == nil
	}
	return false
}
