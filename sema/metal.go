package sema

import (
	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// Metal is the third offload language, and the simplest to analyze: all
// of a .metal file is device code, so every function is the device's and
// a kernel is the only other kind. What it adds to C++ is an address
// space on each pointer and object -- device, constant, threadgroup,
// thread -- which vcx's Metal header spells as attributes:
//
//	#define device __attribute__((metal_device))
//
// An address space qualifies a type, the way const does: in
// `threadgroup float t[64]` the array is in threadgroup memory, and in
// `threadgroup float* p` what p points at is, while p itself is a thread's.
// So a space written on a declaration is the object's own when the
// declared type is no pointer or reference, and the pointee's otherwise.
// The pointee's space is not checked here yet -- a device pointer
// converts to a threadgroup one without a word -- and the backend infers
// every pointer's space from where it came from, refusing one that could
// be in two.

// metal reports whether the unit is a .metal file.
func (a *Analyzer) metal() bool { return a.model.Offload == types.Metal }

// metalExecSpace is a Metal function's kind: a kernel, or a device
// function, which every other function is.
func (a *Analyzer) metalExecSpace(groups []*ast.AttrGroup, at ast.Tok) ExecSpace {
	for _, name := range []string{"metal_vertex", "vertex", "metal_fragment", "fragment", "visible", "metal_visible"} {
		if hasAttrNamed(groups, name, a.unit) {
			a.errorAt(at, "vertex and fragment functions are not built yet; vcx compiles a .metal file's kernel functions")
			return SpaceDevice
		}
	}
	if hasAttrNamed(groups, "global", a.unit) || hasAttrNamed(groups, "kernel", a.unit) {
		return SpaceGlobal
	}
	return SpaceDevice
}

// metalMemSpace is where a Metal object lives, read from the address
// space on its declaration and its type.
func (a *Analyzer) metalMemSpace(groups []*ast.AttrGroup, at ast.Tok, t types.Type) MemSpace {
	var space string
	for _, name := range []string{"device", "constant", "threadgroup", "thread"} {
		if !hasAttrNamed(groups, "metal_"+name, a.unit) {
			continue
		}
		if space != "" && space != name {
			a.errorAt(at, "a type has one address space, and this one is both "+space+" and "+name)
			return MemDefault
		}
		space = name
	}
	obj := types.Unqualify(t)
	for {
		arr, isArray := obj.(*types.Array)
		if !isArray {
			break
		}
		obj = types.Unqualify(arr.Elem)
	}
	if _, isPtr := obj.(*types.Pointer); isPtr || types.IsReference(obj) {
		return MemDefault // the space is what it points at
	}
	switch space {
	case "threadgroup":
		if a.curFunc == nil || a.curFunc.Space != SpaceGlobal {
			a.errorAt(at, "variables in the threadgroup address space cannot be declared in a non-qualified function: a threadgroup variable is a kernel's")
			return MemDefault
		}
		return MemShared
	case "constant":
		if a.curFunc != nil {
			a.errorAt(at, "a constant variable is declared at program scope; in a function, a variable is the thread's")
			return MemDefault
		}
		return MemConstant
	case "device":
		a.errorAt(at, "program scope variable must reside in constant address space: device memory is a kernel's buffer arguments")
	}
	return MemDefault
}

// checkMetalTokens refuses what MSL has no word for, wherever it is
// written: double, since an Apple GPU has no 64-bit float. (A floating
// literal with no suffix is a float in MSL, not a double; see the
// literal's type in expr.go.)
func (a *Analyzer) checkMetalTokens() {
	if !a.metal() {
		return
	}
	for t := ast.Tok(0); int(t) < a.unit.Len(); t++ {
		if a.unit.Kind(t) == token.DOUBLE {
			a.errorAt(t, "'double' is not supported in Metal")
		}
	}
}
