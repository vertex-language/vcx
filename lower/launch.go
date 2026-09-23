package lower

import (
	"fmt"
	"strings"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/mangle"
	"github.com/vertex-language/vcx/offload"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// The host side of a kernel, as nvcc and clang build it.
//
// A kernel definition in the host pass is a stub of the kernel's own
// host name: it pops the configuration the launch pushed and hands the
// runtime its parameters' addresses. A launch, f<<<g, b>>>(args), pushes
// the configuration and calls the stub like any function. At startup a
// constructor registers the device image and, against it, each stub by
// the device name of its kernel and each __device__ object by its
// shadow, so that the runtime can find the kernel a stub's address
// stands for and copy to and from the objects by name.
//
// The runtime entry points are the vendors' -- __cudaRegisterFatBinary
// and the rest -- with two of vcx's own where the vendor's take a dim3
// by value, which is a class the IR here would rather pass as six words:
// __vcx_cudaPushCallConfiguration and __vcx_cudaLaunch, which the
// runtime forwards.

// runtimeNames is the offload language's runtime, by role.
type runtimeNames struct {
	push, pop, launch                                                      string
	registerFatbin, registerFunction, registerVar, registerEnd, unregister string
	fatbinSection, wrapperSection                                          string
}

func (u *unit) runtime() runtimeNames {
	// A PE image's section name is eight bytes: what nvcc and hipcc use
	// on Windows is the ELF name cut to fit.
	pe := strings.HasSuffix(u.opt.Target.Use(), "/windows")
	// A Mach-O section is named by its segment as well, because the
	// segment is what the loader protects: the names below are written as
	// as(1) specifiers, which is what the container reads them as.
	macho := strings.HasSuffix(u.opt.Target.Use(), "/macos")
	if u.model.Offload == types.HIP {
		rt := runtimeNames{
			push: "__vcx_hipPushCallConfiguration", pop: "__hipPopCallConfiguration", launch: "__vcx_hipLaunch",
			registerFatbin: "__hipRegisterFatBinary", registerFunction: "__hipRegisterFunction",
			registerVar: "__hipRegisterVar", registerEnd: "", unregister: "__hipUnregisterFatBinary",
			fatbinSection: ".hip_fatbin", wrapperSection: ".hipFatBinSegment",
		}
		if pe {
			rt.fatbinSection, rt.wrapperSection = ".hip_fat", ".hipFatB"
		}
		if macho {
			// A Mach-O section name is sixteen bytes, so the segment's
			// spelling of .hipFatBinSegment is the one that fits.
			rt.fatbinSection, rt.wrapperSection = "__HIP,__hip_fatbin", "__HIP,__hipFatBinSeg"
		}
		return rt
	}
	rt := runtimeNames{
		push: "__vcx_cudaPushCallConfiguration", pop: "__cudaPopCallConfiguration", launch: "__vcx_cudaLaunch",
		registerFatbin: "__cudaRegisterFatBinary", registerFunction: "__cudaRegisterFunction",
		registerVar: "__cudaRegisterVar", registerEnd: "__cudaRegisterFatBinaryEnd", unregister: "__cudaUnregisterFatBinary",
		fatbinSection: ".nv_fatbin", wrapperSection: ".nvFatBinSegment",
	}
	if pe {
		rt.fatbinSection, rt.wrapperSection = ".nv_fatb", ".nvFatBi"
	}
	if macho {
		// clang's own names for these on a Mach-O target.
		rt.fatbinSection, rt.wrapperSection = "__NV_CUDA,__nv_fatbin", "__NV_CUDA,__fatbin"
	}
	return rt
}

// rtImport is the runtime function named, imported once.
func (u *unit) rtImport(name string, sig *ir.Sig) ir.Callee {
	name = u.symbolName(name)
	if imp, ok := u.importsByName[name]; ok {
		return imp
	}
	imp := u.mod.ImportFunc(name, sig)
	u.importsByName[name] = imp
	return imp
}

// deviceName is a kernel's or object's name in the device image: the
// Itanium mangling, whatever the host mangles with.
func (u *unit) deviceName(fn *sema.FuncSymbol) string {
	name, err := mangle.FunctionName(mangle.Itanium, mangle.Describe(fn))
	if err != nil {
		u.errorf(fn.SymPos, "%v", err)
		return fn.SymName
	}
	return name
}

func (u *unit) deviceVarName(v *sema.VarSymbol) string {
	name, err := mangle.VariableName(mangle.Itanium, mangle.DescribeVariable(v, nil))
	if err != nil {
		u.errorf(v.SymPos, "%v", err)
		return v.SymName
	}
	return name
}

// cString is a NUL-terminated string in the read-only data.
func (u *unit) cString(name, s string) *ir.Global {
	g := u.mod.Global(u.symbolName(name), ir.RO, ir.Array(uint64(len(s)+1), ir.StoreI8.FType())).Internal()
	g.Align(1)
	g.Init(ir.Str(s))
	return g
}

// defineKernelStub is a kernel's definition in the host pass.
func (u *unit) defineKernelStub(sym *sema.FuncSymbol, f *ir.Func) {
	rt := u.runtime()
	fl := &fn{u: u, sym: sym, f: f, slots: map[sema.Symbol]ir.Ptr{}}
	fl.entry = f.Entry()
	fl.blk = f.Block("body")
	b := fl.blk
	ptr := u.model.SizePtr

	// The parameters, each in storage of its own, and the table of
	// their addresses the runtime reads them through.
	args := u.params[sym]
	table := fl.entry.Ptr.Alloc(uint64(ptr*int64(max(len(args), 1))), uint64(ptr)).Named("args")
	for i, p := range sym.Params {
		if i >= len(args) {
			break
		}
		var slot ir.Ptr
		if addr, isPtr := args[i].(ir.Ptr); isPtr && classOf(p.SymType) != nil && !isReference(p.SymType) {
			// A class parameter arrives in storage the caller made.
			slot = addr
		} else {
			slot = fl.alloc(p.SymType, p.SymName)
			fl.store(slot, args[i], p.SymType)
		}
		b.Ptr.Store(slot, b.Ptr.Add(table, b.I64.Const(int64(i)*ptr)))
	}

	// The configuration the launch pushed.
	grid := fl.entry.Ptr.Alloc(12, 4).Named("grid")
	block := fl.entry.Ptr.Alloc(12, 4).Named("block")
	shmem := fl.entry.Ptr.Alloc(8, 8).Named("shmem")
	stream := fl.entry.Ptr.Alloc(8, 8).Named("stream")
	pop := u.rtImport(rt.pop, ir.NewSig().Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypePtr).Ret(ir.TypeI32))
	b.Call(pop, grid, block, shmem, stream)
	dim := func(p ir.Ptr, off int64) ir.I32 { return b.I32.Load(b.Ptr.Add(p, b.I64.Const(off))) }
	launch := u.rtImport(rt.launch, ir.NewSig().Param(ir.TypePtr).
		Param(ir.TypeI32).Param(ir.TypeI32).Param(ir.TypeI32).Param(ir.TypeI32).Param(ir.TypeI32).Param(ir.TypeI32).
		Param(ir.TypePtr).Param(ir.TypeI64).Param(ir.TypePtr).Ret(ir.TypeI32))
	b.Call(launch, b.Ptr.GetAddr(f),
		dim(grid, 0), dim(grid, 4), dim(grid, 8), dim(block, 0), dim(block, 4), dim(block, 8),
		table, b.I64.Load(shmem), b.Ptr.Load(stream))
	b.Return()
	fl.entry.Br(f.Blocks()[1].To())
	u.kernelStubs = append(u.kernelStubs, sym)
}

// pushLaunch lowers a launch's configuration: the push the stub pops.
func (fl *fn) pushLaunch(e *ast.CallExpr) bool {
	b := fl.blk
	rt := fl.u.runtime()
	var dims [6]ir.I32
	for i := 0; i < 2 && i < len(e.Config); i++ {
		x, y, z, ok := fl.dim3Of(e.Config[i])
		if !ok {
			return false
		}
		dims[3*i], dims[3*i+1], dims[3*i+2] = x, y, z
	}
	shmem := b.I64.Const(0)
	if len(e.Config) > 2 {
		v := fl.expr(e.Config[2])
		if v == nil {
			return false
		}
		shmem = fl.convert(v, fl.typeOf(e.Config[2]), types.Typ(types.ULongLong)).(ir.I64)
	}
	stream := b.Ptr.Const()
	if len(e.Config) > 3 {
		v := fl.expr(e.Config[3])
		if v == nil {
			return false
		}
		switch s := v.(type) {
		case ir.Ptr:
			stream = s
		case ir.I64:
			stream = b.Ptr.FromI64(s)
		case ir.I32:
			stream = b.Ptr.FromI64(b.I64.ZExtI32(s))
		}
	}
	push := fl.u.rtImport(rt.push, ir.NewSig().
		Param(ir.TypeI32).Param(ir.TypeI32).Param(ir.TypeI32).Param(ir.TypeI32).Param(ir.TypeI32).Param(ir.TypeI32).
		Param(ir.TypeI64).Param(ir.TypePtr).Ret(ir.TypeI32))
	b.Call(push, dims[0], dims[1], dims[2], dims[3], dims[4], dims[5], shmem, stream)
	return true
}

// dim3Of is a launch extent: a dim3's three coordinates, or an integer
// as the x extent with y and z one.
func (fl *fn) dim3Of(x ast.Expr) (ir.I32, ir.I32, ir.I32, bool) {
	b := fl.blk
	t := types.Unqualify(types.RemoveReference(fl.typeOf(x)))
	if rec := types.AsRecord(t); rec != nil {
		addr, ok := fl.convertedObject(x, rec)
		if !ok {
			addr, ok = fl.objectOf(x)
		}
		if !ok {
			fl.u.errorf(x.Pos(), "lowering has no object for this launch extent")
			return ir.I32{}, ir.I32{}, ir.I32{}, false
		}
		at := func(off int64) ir.I32 { return b.I32.Load(b.Ptr.Add(addr, b.I64.Const(off))) }
		return at(0), at(4), at(8), true
	}
	v := fl.expr(x)
	if v == nil {
		return ir.I32{}, ir.I32{}, ir.I32{}, false
	}
	n := fl.convert(v, fl.typeOf(x), types.Typ(types.UInt)).(ir.I32)
	return n, b.I32.Const(1), b.I32.Const(1), true
}

// defineRegistration emits the constructor that registers the unit's
// device image, kernels and objects with the runtime, and the
// destructor that unregisters it, when the unit has any of them.
func (u *unit) defineRegistration() {
	if !u.offload() || u.devicePass() || u.mod.Err() != nil {
		return
	}
	var vars []*sema.VarSymbol
	for _, v := range u.declaredOrder {
		if v.Memory == sema.MemDevice || v.Memory == sema.MemConstant || v.Memory == sema.MemManaged {
			vars = append(vars, v)
		}
	}
	if len(u.kernelStubs) == 0 && len(vars) == 0 {
		return
	}
	if len(u.opt.Images) == 0 {
		u.errorf(ast.NoTok, "the unit defines kernels and no device image was given to embed; the device pass runs first")
		return
	}
	rt := u.runtime()
	ptr := u.model.SizePtr
	tag := identOf(u.opt.Name)

	// The image, its wrapper, and the handle registration hands back.
	var container []byte
	if u.model.Offload == types.HIP {
		container = offload.Bundle(u.opt.Images)
	} else {
		container = offload.Fatbin(u.opt.Images)
	}
	image := u.mod.Global(u.symbolName("__vcx_fatbin_"+tag), ir.RO, ir.Array(uint64(len(container)), ir.StoreI8.FType())).Internal()
	image.Align(8)
	image.Init(ir.Str(string(container)))
	image.Section(rt.fatbinSection)
	wrapperType := u.mod.Struct("__vcx_fatbin_wrapper_t_"+tag).
		Field("magic", ir.StoreI32.FType()).Field("version", ir.StoreI32.FType()).
		Field("data", ir.StorePtr.FType()).Field("filename", ir.StorePtr.FType())
	wrapper := u.mod.Global(u.symbolName("__vcx_fatbin_wrapper_"+tag), ir.RO, wrapperType.FType()).Internal()
	wrapper.Align(uint64(ptr))
	wrapper.Init(ir.List(ir.Lit(ir.Int(offload.FatbinWrapperMagic)), ir.Lit(ir.Int(offload.FatbinWrapperVersion)), ir.RelocInit(image), ir.Lit(ir.Int(0))))
	wrapper.Section(rt.wrapperSection)
	handle := u.mod.Global(u.symbolName("__vcx_gpubin_handle_"+tag), ir.RW, ir.StorePtr.FType()).Internal()
	handle.Align(uint64(ptr))

	// The destructor: unregister.
	dtor := u.mod.Func(u.symbolName("__vcx_offload_dtor_" + tag)).Internal()
	{
		e := dtor.Entry()
		body := dtor.Block("body")
		e.Br(body.To())
		unregister := u.rtImport(rt.unregister, ir.NewSig().Param(ir.TypePtr))
		body.Call(unregister, body.Ptr.Load(body.Ptr.GetAddr(handle)))
		body.Return()
	}

	// The constructor: register the image, each kernel, each object,
	// and the destructor for exit.
	ctor := u.mod.Func(u.symbolName("__vcx_offload_ctor_" + tag)).Internal()
	e := ctor.Entry()
	b := ctor.Block("body")
	e.Br(b.To())
	register := u.rtImport(rt.registerFatbin, ir.NewSig().Param(ir.TypePtr).Ret(ir.TypePtr))
	h := b.Call(register, b.Ptr.GetAddr(wrapper)).Ptr(0)
	b.Ptr.Store(h, b.Ptr.GetAddr(handle))
	null := b.Ptr.Const()
	registerFunction := u.rtImport(rt.registerFunction, ir.NewSig().
		Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypeI32).
		Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypePtr).Ret(ir.TypeI32))
	for i, k := range u.kernelStubs {
		stub := u.funcs[k]
		if stub == nil {
			continue
		}
		name := b.Ptr.GetAddr(u.cString(fmt.Sprintf("__vcx_kernel_name_%s_%d", tag, i), u.deviceName(k)))
		b.Call(registerFunction, h, b.Ptr.GetAddr(stub), name, name, b.I32.Const(-1), null, null, null, null, null)
	}
	registerVar := u.rtImport(rt.registerVar, ir.NewSig().
		Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypeI32).
		Param(ir.TypeI64).Param(ir.TypeI32).Param(ir.TypeI32))
	for i, v := range vars {
		g, ok := u.globals[v]
		if !ok {
			continue
		}
		size, _ := u.sizeAlign(v.SymType)
		name := b.Ptr.GetAddr(u.cString(fmt.Sprintf("__vcx_var_name_%s_%d", tag, i), u.deviceVarName(v)))
		constant := int64(0)
		if v.Memory == sema.MemConstant {
			constant = 1
		}
		b.Call(registerVar, h, b.Ptr.GetAddr(g), name, name, b.I32.Const(0), b.I64.Const(size), b.I32.Const(constant), b.I32.Const(0))
	}
	if rt.registerEnd != "" {
		end := u.rtImport(rt.registerEnd, ir.NewSig().Param(ir.TypePtr))
		b.Call(end, h)
	}
	atexit := u.rtImport("atexit", ir.NewSig().Param(ir.TypePtr).Ret(ir.TypeI32))
	b.Call(atexit, b.Ptr.GetAddr(dtor))
	b.Return()

	// The constructor's place in the startup order.
	sec, kind := u.initSection()
	reg := u.mod.Global(u.symbolName("__vcx_offload_ctor_ptr_"+tag), kind, ir.StorePtr.FType()).Internal()
	reg.Align(uint64(ptr))
	reg.Init(ir.RelocInit(ctor))
	reg.Section(sec)
}
