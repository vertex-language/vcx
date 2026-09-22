package sema

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// The offload languages -- CUDA and HIP -- add two things to a
// declaration: where a function runs, and where an object lives. Both
// are attributes, spelled __global__, __device__, __host__, __shared__,
// __constant__ and __managed__ by macros in the runtime's headers, and
// __attribute__((global)) and the rest underneath.

// An ExecSpace is where a function of an offload unit runs.
type ExecSpace uint8

const (
	// SpaceHost is a function with no execution-space attribute: the
	// host's, which the device pass does not lower and device code may
	// not call.
	SpaceHost ExecSpace = iota

	// SpaceDevice is __device__: lowered in the device pass alone, and
	// called only from device code.
	SpaceDevice

	// SpaceHostDevice is __host__ __device__: lowered in both passes, one
	// body for two targets. A constexpr function is this implicitly.
	SpaceHostDevice

	// SpaceGlobal is __global__: a kernel. It returns void, is lowered
	// with the kernel calling convention in the device pass, and is a
	// launch stub in the host's.
	SpaceGlobal
)

func (s ExecSpace) String() string {
	switch s {
	case SpaceDevice:
		return "__device__"
	case SpaceHostDevice:
		return "__host__ __device__"
	case SpaceGlobal:
		return "__global__"
	}
	return "__host__"
}

// OnDevice reports whether the function has a body the device pass lowers.
func (s ExecSpace) OnDevice() bool { return s != SpaceHost }

// OnHost reports whether the function has a body the host pass lowers.
func (s ExecSpace) OnHost() bool { return s == SpaceHost || s == SpaceHostDevice }

// A MemSpace is where an object of an offload unit lives.
type MemSpace uint8

const (
	// MemDefault is an object with no memory-space attribute: the
	// host's, in the host pass, and nothing in the device pass -- device
	// code reaching one is an error, unless it is a constant the
	// analysis folds.
	MemDefault MemSpace = iota

	// MemShared is __shared__: workgroup-local storage, one instance per
	// block, zeroed and never initialized.
	MemShared

	// MemConstant is __constant__: the device's read-only storage, which
	// the host writes through the runtime.
	MemConstant

	// MemDevice is __device__: the device's global storage.
	MemDevice

	// MemManaged is __managed__: storage the driver migrates between the
	// two, addressable from both.
	MemManaged
)

func (m MemSpace) String() string {
	switch m {
	case MemShared:
		return "__shared__"
	case MemConstant:
		return "__constant__"
	case MemDevice:
		return "__device__"
	case MemManaged:
		return "__managed__"
	}
	return ""
}

// OnDevice reports whether the object exists in the device pass's module.
func (m MemSpace) OnDevice() bool { return m != MemDefault }

// offload reports whether the unit is written in an offload language.
func (a *Analyzer) offload() bool { return a.model.Offload != types.NoOffload }

// execSpaceOf reads a declaration's execution-space attributes. Two
// attributes, host and device, make a function of both; global is a
// kernel and admits neither of the others.
func (a *Analyzer) execSpaceOf(groups []*ast.AttrGroup, at ast.Tok) ExecSpace {
	if !a.offload() {
		return SpaceHost
	}
	if a.metal() {
		return a.metalExecSpace(groups, at)
	}
	host := hasAttrNamed(groups, "host", a.unit)
	device := hasAttrNamed(groups, "device", a.unit)
	global := hasAttrNamed(groups, "global", a.unit)
	switch {
	case global:
		if host || device {
			a.errorAt(at, "a __global__ function is a kernel and is neither __host__ nor __device__")
		}
		return SpaceGlobal
	case host && device:
		return SpaceHostDevice
	case device:
		return SpaceDevice
	}
	return SpaceHost
}

// memSpaceOf reads a declaration's memory-space attributes. t is the
// declared object's type, which is what a Metal address space is read
// against.
func (a *Analyzer) memSpaceOf(groups []*ast.AttrGroup, at ast.Tok, t types.Type) MemSpace {
	if !a.offload() {
		return MemDefault
	}
	if a.metal() {
		return a.metalMemSpace(groups, at, t)
	}
	var out MemSpace
	for _, name := range []string{"shared", "constant", "device", "managed"} {
		if !hasAttrNamed(groups, name, a.unit) {
			continue
		}
		space := map[string]MemSpace{"shared": MemShared, "constant": MemConstant, "device": MemDevice, "managed": MemManaged}[name]
		if out != MemDefault {
			a.errorAt(at, fmt.Sprintf("an object is %s or %s, not both", out, space))
			continue
		}
		out = space
	}
	return out
}

// launchBoundsOf reads __launch_bounds__(max[, min]): integer literals,
// which is what the attribute is written with in practice; an
// expression there is not evaluated yet and is ignored with a note.
func (a *Analyzer) launchBoundsOf(groups []*ast.AttrGroup, at ast.Tok) [2]int64 {
	var out [2]int64
	for _, g := range groups {
		if g == nil {
			continue
		}
		for _, attr := range g.Attrs {
			if attr == nil || attr.Name == nil || strings.TrimSuffix(strings.TrimPrefix(attr.Name.Text(a.unit), "__"), "__") != "launch_bounds" {
				continue
			}
			i := 0
			for t := attr.Args.Lo; t < attr.Args.Hi && i < 2; t++ {
				switch a.unit.Kind(t) {
				case token.COMMA:
					i++
				case token.INT_LIT:
					n, err := strconv.ParseInt(strings.TrimRight(a.unit.Text(t), "uUlL"), 0, 64)
					if err == nil {
						out[i] = n
					}
				default:
					a.errorAt(at, "__launch_bounds__ takes integer literals here; an expression in it is not evaluated yet")
					return out
				}
			}
		}
	}
	return out
}

// checkKernel is what a kernel may and may not be: [cuda]/B.1 wants it
// to return void, and a member function has an object no launch passes.
func (a *Analyzer) checkKernel(fn *FuncSymbol) {
	if fn.Space != SpaceGlobal {
		return
	}
	if ret := fn.FuncType.Ret; ret != nil && !types.IsVoid(types.Unqualify(ret)) && !isDependentType(ret) {
		if a.metal() {
			a.errorAt(fn.SymPos, fmt.Sprintf("invalid return type '%s' for kernel function: a kernel returns void", ret))
		} else {
			a.errorAt(fn.SymPos, fmt.Sprintf("a __global__ function returns void, not %s", ret))
		}
	}
	if fn.InClass != nil && !fn.Static {
		a.errorAt(fn.SymPos, "a __global__ function cannot be a non-static member: a launch passes no object")
	}
	if fn.FuncType.Variadic {
		a.errorAt(fn.SymPos, "a __global__ function cannot be variadic")
	}
}

// mergeSpace carries a declaration's spaces onto the symbol that
// survives a redeclaration: an attribute on any declaration of a
// function is the function's, as clang inherits them.
func mergeSpace(surviving, decl *FuncSymbol) {
	if decl.Space == SpaceHost {
		return
	}
	switch {
	case surviving.Space == SpaceHost:
		surviving.Space = decl.Space
	case surviving.Space == SpaceDevice && decl.Space == SpaceHostDevice,
		surviving.Space == SpaceHostDevice && decl.Space == SpaceDevice:
		surviving.Space = SpaceHostDevice
	}
}

// implicitSpace is the space a function has with no attribute of its
// own: a constexpr function is host and device, as clang makes it, since
// its body is one every target evaluates; a lambda's operator() takes
// the space of the function it is written in; and a defaulted special
// member is host and device, since its body is the class's own.
func (a *Analyzer) implicitSpace(fn *FuncSymbol) ExecSpace {
	if !a.offload() || fn.Space != SpaceHost {
		return fn.Space
	}
	if a.metal() {
		// A .metal file has no host: every function is the device's.
		return SpaceDevice
	}
	if fn.Constexpr || fn.Consteval || fn.Defaulted {
		return SpaceHostDevice
	}
	return SpaceHost
}

// checkLaunch is what a kernel launch, f<<<grid, block, shmem, stream>>>(
// args), may be: the callee is a kernel, the configuration has two to
// four expressions, the first two convertible to dim3 -- an integer is
// the x extent -- and a launch is written in host code, since dynamic
// parallelism is not lowered.
func (a *Analyzer) checkLaunch(c *ast.CallExpr, callee *FuncSymbol) {
	if !c.IsLaunch() {
		if callee.Space == SpaceGlobal && a.offload() {
			a.errorAt(c.Pos(), fmt.Sprintf("%q is a __global__ function and is launched with <<<grid, block>>>, not called", callee.SymName))
		}
		return
	}
	if !a.offload() {
		a.errorAt(c.Launch, "a kernel launch in a C++ unit; name the file .cu or .hip, or pass -x cuda")
		return
	}
	if callee.Space != SpaceGlobal {
		a.errorAt(c.Launch, fmt.Sprintf("%q is not a __global__ function and cannot be launched", callee.SymName))
	}
	if a.curFunc != nil && a.curFunc.Space == SpaceGlobal || a.curFunc != nil && a.curFunc.Space == SpaceDevice {
		a.errorAt(c.Launch, "a launch from device code is dynamic parallelism, which is not lowered")
	}
	if len(c.Config) < 2 || len(c.Config) > 4 {
		a.errorAt(c.Launch, fmt.Sprintf("a launch configuration is <<<grid, block>>>, <<<grid, block, shmem>>> or <<<grid, block, shmem, stream>>>; this one has %d expressions", len(c.Config)))
	}
	for i, x := range c.Config {
		info := a.CheckExpr(x)
		if isDependentExpr(info) {
			continue
		}
		t := types.Unqualify(types.RemoveReference(info.Type))
		switch i {
		case 0, 1:
			if rec := types.AsRecord(t); rec != nil && rec.Name == "dim3" {
				continue
			}
			if types.IsInteger(t) || types.IsEnum(t) {
				continue
			}
			a.errorAt(x.Pos(), fmt.Sprintf("a launch's %s is a dim3 or an integer, not %s", [...]string{"grid", "block"}[i], info.Type))
		case 2:
			if !types.IsInteger(t) && !types.IsEnum(t) {
				a.errorAt(x.Pos(), fmt.Sprintf("a launch's shared memory size is an integer, not %s", info.Type))
			}
		case 3:
			if _, isPtr := t.(*types.Pointer); !isPtr && !isNullConstant(x, info) && !types.IsInteger(t) {
				a.errorAt(x.Pos(), fmt.Sprintf("a launch's stream is a cudaStream_t, not %s", info.Type))
			}
		}
	}
}
