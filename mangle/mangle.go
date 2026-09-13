// Package mangle spells C++ entities the way a linker sees them.
//
// Two schemes exist and they share nothing but the problem: the Itanium
// C++ ABI, which every ELF and Mach-O toolchain follows, and Microsoft's,
// which MSVC follows and which is documented only by its output. Both
// encode enough of a declaration -- scopes, name, parameter types, and for
// a member its class and cv-qualifiers -- that two overloads get two names.
//
// The Microsoft scheme is diffed against cl.exe in tests/mangle, since
// undname.exe and cl are the only statement of it. The Itanium scheme is
// written from its specification and has no oracle on this machine; its
// tests are examples known from the document.
//
// Anything a scheme cannot spell is an error rather than a guess. A wrong
// symbol name links against nothing and says nothing about why, and a
// name that happens to be spelled the same as an unrelated function links
// against the wrong thing and says even less.
package mangle

import (
	"fmt"

	"github.com/vertex-language/vcx/types"
)

// ABI selects the mangling scheme.
type ABI uint8

const (
	Itanium ABI = iota
	Microsoft
)

// Kind is what sort of function a Function is, beyond its name.
type Kind uint8

const (
	Ordinary   Kind = iota
	Ctor            // Name is the class name
	Dtor            // Name is `~` + the class name
	Conversion      // Name is "operator T"; Conv is T

	// DeletingDtor is Microsoft's vector deleting destructor, `??_E`:
	// the function a virtual destructor's table slot holds, taking a
	// flags word and returning the object. Type is `void *(unsigned)`.
	// ScalarDeletingDtor is `??_G`, the same for one object; cl defines
	// it and makes `??_E` a weak alias of it until an array is deleted.
	// Itanium has no such function (its D0 is a complete-object variant).
	DeletingDtor
	ScalarDeletingDtor

	// CtorBase and DtorBase are Itanium's base-object constructor and
	// destructor, `C2` and `D2`: what a derived class's constructor and
	// destructor call for their bases. DtorDeleting is `D0`, the deleting
	// destructor a virtual destructor's second table slot holds, which
	// destroys the complete object and frees it. Microsoft has none of the
	// three.
	CtorBase
	DtorBase
	DtorDeleting
)

// A Scope is one enclosing namespace or class: its name, and for a
// specialization of a class template, the arguments that are part of it.
type Scope struct {
	Name string
	Args []types.TemplateArg
}

// Scopes spells plain names as scopes, for the common case.
func Scopes(names ...string) []Scope {
	out := make([]Scope, len(names))
	for i, n := range names {
		out[i] = Scope{Name: n}
	}
	return out
}

// Function is what a mangler needs to know about a function.
type Function struct {
	// Scopes are the namespaces and classes enclosing the declaration,
	// outermost first; for a member, the last one is its class.
	Scopes []Scope
	Name   string
	Type   *types.Func
	Kind   Kind
	Conv   types.Type // the target of a conversion function

	// TemplateArgs is set on a specialization of a function template: the
	// arguments it was instantiated with, which are part of its name.
	TemplateArgs []types.TemplateArg

	// Pattern is the template's own signature, written in its template
	// parameters, for a specialization. Itanium mangles a specialization
	// with this rather than the substituted type -- `twice<int>` is
	// `_Z5twiceIiET_S0_`, return type and all -- so that two templates
	// whose instances happen to coincide still get two names.
	Pattern *types.Func

	// Member is a non-static member function, one with an implicit object
	// parameter. Static is a static member. Neither is set for a free
	// function, even one declared inside a namespace.
	Member  bool
	Static  bool
	Virtual bool
	Access  types.Access

	// ExternC keeps the name as written. `main` is treated the same way.
	ExternC bool
}

// Variable is what a mangler needs to know about an object with linkage.
type Variable struct {
	Scopes []Scope
	Name   string
	Type   types.Type

	// Static is a static data member -- the last scope is its class.
	Static bool
	Access types.Access

	ExternC bool
}

// FunctionName is the symbol a function definition gets.
func FunctionName(abi ABI, f *Function) (string, error) {
	if f.ExternC || (f.Name == "main" && len(f.Scopes) == 0) {
		return f.Name, nil
	}
	switch abi {
	case Microsoft:
		return newMSVC().function(f)
	case Itanium:
		name, err := newItanium().function(f)
		if err != nil && f.Pattern != nil {
			// A template's signature may name a type through an alias
			// template, which Itanium spells as what the alias expands to
			// in the template's parameters -- an expansion the analysis
			// does not keep. The instance is then spelled with its
			// substituted signature instead: unique among this program's
			// names, and not the one clang writes, which costs nothing for
			// an inline template's COMDAT copy and would cost a link for
			// an explicit instantiation defined elsewhere.
			plain := *f
			plain.Pattern = nil
			return newItanium().function(&plain)
		}
		return name, err
	}
	return "", fmt.Errorf("mangle: no scheme %d", abi)
}

// VariableName is the symbol an object with linkage gets.
func VariableName(abi ABI, v *Variable) (string, error) {
	if v.ExternC {
		return v.Name, nil
	}
	switch abi {
	case Microsoft:
		return newMSVC().variable(v)
	case Itanium:
		return newItanium().variable(v)
	}
	return "", fmt.Errorf("mangle: no scheme %d", abi)
}

// VTableName is the symbol a class's virtual function table gets.
//
// Under the Microsoft ABI a class with several polymorphic bases has
// several tables, one per base path; base names the non-primary base the
// table belongs to and is empty for the class's own.
func VTableName(abi ABI, rec *types.Record, base []*types.Record) (string, error) {
	switch abi {
	case Microsoft:
		return newMSVC().vftable(rec, base)
	case Itanium:
		if len(base) != 0 {
			return "", fmt.Errorf("mangle: the Itanium ABI has one vtable per class")
		}
		return newItanium().vtable(rec)
	}
	return "", fmt.Errorf("mangle: no scheme %d", abi)
}

// ThunkName is the symbol of the this-adjusting thunk a secondary virtual
// table holds in place of f: a class D derived from A and B that overrides
// B's virtual is reached through B's table with a pointer to the B
// subobject, and the thunk subtracts adjust to find the D before calling
// the real function. Itanium spells the adjustment as what is added,
// `_ZThn16_` for a subtraction of sixteen; the argument means the same
// thing under both schemes.
func ThunkName(abi ABI, f *Function, adjust int64) (string, error) {
	switch abi {
	case Microsoft:
		return newMSVC().thunk(f, adjust)
	case Itanium:
		return newItanium().thunk(f, adjust)
	}
	return "", fmt.Errorf("mangle: no scheme %d", abi)
}

// TypeInfoName is the symbol of a class's std::type_info object, `_ZTI`
// and the class's encoding, and TypeNameName the symbol of the string that
// object points at, `_ZTS` and the same. TypeName is that string's
// contents: the encoding alone. Only Itanium spells these; Microsoft's RTTI
// is a different structure altogether.
func TypeInfoName(abi ABI, rec *types.Record) (string, error) {
	enc, err := TypeName(abi, rec)
	return "_ZTI" + enc, err
}

// TypeNameName is the symbol of a class's type name string; see TypeInfoName.
func TypeNameName(abi ABI, rec *types.Record) (string, error) {
	enc, err := TypeName(abi, rec)
	return "_ZTS" + enc, err
}

// TypeName is a class's type encoding, as its type_info's name() holds it.
func TypeName(abi ABI, rec *types.Record) (string, error) {
	if abi != Itanium {
		return "", fmt.Errorf("mangle: Itanium type information under another scheme")
	}
	m := newItanium()
	m.className(recordScopes(rec))
	return m.sb.String(), m.err
}

// VBTableName is the symbol a class's virtual base table gets, which only
// the Microsoft ABI has: Itanium keeps virtual base offsets in the vtable.
func VBTableName(abi ABI, rec *types.Record, base []*types.Record) (string, error) {
	if abi != Microsoft {
		return "", fmt.Errorf("mangle: only the Microsoft ABI has a vbtable")
	}
	return newMSVC().vbtable(rec, base)
}

// operators maps a C++ operator-function-id, spelled as the analysis names
// it, to its code under each scheme. The two columns are unrelated
// alphabets that happen to cover the same list.
var operators = map[string]struct{ itanium, msvc string }{
	"operator new":      {"nw", "?2"},
	"operator delete":   {"dl", "?3"},
	"operator new[]":    {"na", "?_U"},
	"operator delete[]": {"da", "?_V"},
	"operator+":         {"pl", "?H"},
	"operator-":         {"mi", "?G"},
	"operator*":         {"ml", "?D"},
	"operator/":         {"dv", "?K"},
	"operator%":         {"rm", "?L"},
	"operator^":         {"eo", "?T"},
	"operator&":         {"an", "?I"},
	"operator|":         {"or", "?U"},
	"operator~":         {"co", "?S"},
	"operator!":         {"nt", "?7"},
	"operator=":         {"aS", "?4"},
	"operator<":         {"lt", "?M"},
	"operator>":         {"gt", "?O"},
	"operator+=":        {"pL", "?Y"},
	"operator-=":        {"mI", "?Z"},
	"operator*=":        {"mL", "?X"},
	"operator/=":        {"dV", "?_0"},
	"operator%=":        {"rM", "?_1"},
	"operator^=":        {"eO", "?_6"},
	"operator&=":        {"aN", "?_4"},
	"operator|=":        {"oR", "?_5"},
	"operator<<":        {"ls", "?6"},
	"operator>>":        {"rs", "?5"},
	"operator<<=":       {"lS", "?_3"},
	"operator>>=":       {"rS", "?_2"},
	"operator==":        {"eq", "?8"},
	"operator!=":        {"ne", "?9"},
	"operator<=":        {"le", "?N"},
	"operator>=":        {"ge", "?P"},
	"operator<=>":       {"ss", "?__M"},
	"operator&&":        {"aa", "?V"},
	"operator||":        {"oo", "?W"},
	"operator++":        {"pp", "?E"},
	"operator--":        {"mm", "?F"},
	"operator,":         {"cm", "?Q"},
	"operator->*":       {"pm", "?J"},
	"operator->":        {"pt", "?C"},
	"operator()":        {"cl", "?R"},
	"operator[]":        {"ix", "?A"},
	"operator co_await": {"aw", "?__L"},
}

// recordScopes is a class's own scope path: its enclosing scopes and then
// itself, with its template arguments if it is a specialization.
func recordScopes(rec *types.Record) []Scope {
	out := Scopes(rec.Scopes...)
	return append(out, Scope{Name: rec.Name, Args: rec.TemplateArgs})
}
