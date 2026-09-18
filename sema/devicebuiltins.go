package sema

import (
	"fmt"
	"strings"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/types"
)

// The device builtins: what the offload languages' runtime headers
// bottom out in. They are declared with the names clang gives them --
// __nvvm_* for NVIDIA, __builtin_amdgcn_* for AMD -- so that the vendors'
// own headers, which spell them, are read unchanged. Each is an ordinary
// extern "C" function to the analysis, with the signature the table
// gives it (the encoding is libraryBuiltins'), and marked Intrinsic so
// that lowering turns the call into a verb.
//
// Both passes of a unit declare the device's set: a host function's body
// is not lowered on the device, but a __host__ __device__ one is checked
// once and has to see the same names in both, the way clang's host pass
// keeps the device's builtins as an auxiliary target's.

var nvvmBuiltins = map[string]string{
	// The work-item's place in the grid.
	"__nvvm_read_ptx_sreg_tid_x": "u()", "__nvvm_read_ptx_sreg_tid_y": "u()", "__nvvm_read_ptx_sreg_tid_z": "u()",
	"__nvvm_read_ptx_sreg_ctaid_x": "u()", "__nvvm_read_ptx_sreg_ctaid_y": "u()", "__nvvm_read_ptx_sreg_ctaid_z": "u()",
	"__nvvm_read_ptx_sreg_ntid_x": "u()", "__nvvm_read_ptx_sreg_ntid_y": "u()", "__nvvm_read_ptx_sreg_ntid_z": "u()",
	"__nvvm_read_ptx_sreg_nctaid_x": "u()", "__nvvm_read_ptx_sreg_nctaid_y": "u()", "__nvvm_read_ptx_sreg_nctaid_z": "u()",
	"__nvvm_read_ptx_sreg_laneid":   "u()",
	"__nvvm_read_ptx_sreg_warpsize": "i()",

	// Synchronization.
	"__nvvm_barrier0": "v()", "__nvvm_barrier0_popc": "i(i)", "__nvvm_barrier0_and": "i(i)", "__nvvm_barrier0_or": "i(i)",
	"__nvvm_bar_warp_sync": "v(u)",
	"__nvvm_membar_cta":    "v()", "__nvvm_membar_gl": "v()", "__nvvm_membar_sys": "v()",

	// Warp exchange.
	"__nvvm_shfl_sync_idx_i32": "i(u,i,i,i)", "__nvvm_shfl_sync_idx_f32": "f(u,f,i,i)",
	"__nvvm_shfl_sync_up_i32": "i(u,i,i,i)", "__nvvm_shfl_sync_up_f32": "f(u,f,i,i)",
	"__nvvm_shfl_sync_down_i32": "i(u,i,i,i)", "__nvvm_shfl_sync_down_f32": "f(u,f,i,i)",
	"__nvvm_shfl_sync_bfly_i32": "i(u,i,i,i)", "__nvvm_shfl_sync_bfly_f32": "f(u,f,i,i)",
	"__nvvm_vote_all_sync": "i(u,i)", "__nvvm_vote_any_sync": "i(u,i)", "__nvvm_vote_ballot_sync": "u(u,i)",

	// Bits.
	"__nvvm_bitcast_f2i": "i(f)", "__nvvm_bitcast_i2f": "f(i)", "__nvvm_bitcast_d2ll": "ll(d)", "__nvvm_bitcast_ll2d": "d(ll)",
	"__nvvm_brev32": "u(u)", "__nvvm_brev64": "ull(ull)", "__nvvm_prmt": "u(u,u,u)",
	"__nvvm_mul24_i": "i(i,i)", "__nvvm_mul24_ui": "u(u,u)",
	"__nvvm_mulhi_i": "i(i,i)", "__nvvm_mulhi_ui": "u(u,u)", "__nvvm_mulhi_ll": "ll(ll,ll)", "__nvvm_mulhi_ull": "ull(ull,ull)",
	"__nvvm_fshl": "u(u,u,u)", "__nvvm_fshr": "u(u,u,u)",
	"__nvvm_sad_i": "u(i,i,u)", "__nvvm_sad_ui": "u(u,u,u)",

	// Math the hardware does in an instruction.
	"__nvvm_sqrt_rn_f": "f(f)", "__nvvm_sqrt_rn_d": "d(d)", "__nvvm_rsqrt_approx_f": "f(f)", "__nvvm_rcp_approx_ftz_f": "f(f)",
	"__nvvm_ex2_approx_f": "f(f)", "__nvvm_lg2_approx_f": "f(f)", "__nvvm_sin_approx_f": "f(f)", "__nvvm_cos_approx_f": "f(f)",
	"__nvvm_fma_rn_f": "f(f,f,f)", "__nvvm_fma_rn_d": "d(d,d,d)",
	"__nvvm_f2i_rn": "i(f)",
}

func init() {
	// The atomics, at the three scopes: __nvvm_atom_add_gen_i, its
	// _cta_ and _sys_ forms, and the rest.
	for _, scope := range []string{"", "cta_", "sys_"} {
		for _, op := range []string{"add", "xchg", "max", "min", "and", "or", "xor"} {
			nvvmBuiltins["__nvvm_atom_"+scope+op+"_gen_i"] = "i(*i,i)"
			nvvmBuiltins["__nvvm_atom_"+scope+op+"_gen_ll"] = "ll(*ll,ll)"
		}
		for _, op := range []string{"max", "min"} {
			nvvmBuiltins["__nvvm_atom_"+scope+op+"_gen_ui"] = "u(*u,u)"
			nvvmBuiltins["__nvvm_atom_"+scope+op+"_gen_ull"] = "ull(*ull,ull)"
		}
		nvvmBuiltins["__nvvm_atom_"+scope+"add_gen_f"] = "f(*f,f)"
		nvvmBuiltins["__nvvm_atom_"+scope+"add_gen_d"] = "d(*d,d)"
		nvvmBuiltins["__nvvm_atom_"+scope+"cas_gen_i"] = "i(*i,i,i)"
		nvvmBuiltins["__nvvm_atom_"+scope+"cas_gen_ll"] = "ll(*ll,ll,ll)"
	}
}

var amdgcnBuiltins = map[string]string{
	"__builtin_amdgcn_workitem_id_x": "u()", "__builtin_amdgcn_workitem_id_y": "u()", "__builtin_amdgcn_workitem_id_z": "u()",
	"__builtin_amdgcn_workgroup_id_x": "u()", "__builtin_amdgcn_workgroup_id_y": "u()", "__builtin_amdgcn_workgroup_id_z": "u()",
	"__builtin_amdgcn_workgroup_size_x": "u()", "__builtin_amdgcn_workgroup_size_y": "u()", "__builtin_amdgcn_workgroup_size_z": "u()",
	"__builtin_amdgcn_grid_size_x": "u()", "__builtin_amdgcn_grid_size_y": "u()", "__builtin_amdgcn_grid_size_z": "u()",
	"__builtin_amdgcn_wavefrontsize": "u()",

	"__builtin_amdgcn_s_barrier": "v()", "__builtin_amdgcn_wave_barrier": "v()",
	"__builtin_amdgcn_fence":   "v(i,S)",
	"__builtin_amdgcn_s_sleep": "v(i)",

	"__builtin_amdgcn_mbcnt_lo": "u(u,u)", "__builtin_amdgcn_mbcnt_hi": "u(u,u)",
	"__builtin_amdgcn_ds_bpermute": "i(i,i)", "__builtin_amdgcn_ds_swizzle": "i(i,i)",
	"__builtin_amdgcn_readfirstlane": "i(i)", "__builtin_amdgcn_readlane": "i(i,i)",
	"__builtin_amdgcn_uicmp": "ull(u,u,u)", "__builtin_amdgcn_read_exec": "ull()",

	"__builtin_amdgcn_sqrtf": "f(f)", "__builtin_amdgcn_rsqf": "f(f)", "__builtin_amdgcn_rcpf": "f(f)",
	"__builtin_amdgcn_exp2f": "f(f)", "__builtin_amdgcn_logf": "f(f)", "__builtin_amdgcn_sinf": "f(f)", "__builtin_amdgcn_cosf": "f(f)",
	"__builtin_amdgcn_s_memtime": "ull()",
}

// deviceBuiltins is the set of the device the unit is compiled for.
func (a *Analyzer) deviceBuiltins() map[string]string {
	switch a.model.DeviceISA {
	case types.NVPTX:
		return nvvmBuiltins
	case types.AMDGCN:
		return amdgcnBuiltins
	}
	return nil
}

// declareDeviceBuiltins enters the device's builtins in the global scope.
func (a *Analyzer) declareDeviceBuiltins() {
	g := a.globalScope
	for name, sig := range a.deviceBuiltins() {
		parts := splitSig(sig)
		ft := &types.Func{Ret: a.sigType(parts[0])}
		sym := &FuncSymbol{
			SymName:   name,
			LinkName:  name,
			FuncType:  ft,
			SymScope:  g,
			ExternC:   true,
			Intrinsic: true,
			Space:     SpaceDevice,
		}
		for _, p := range parts[1:] {
			pt := a.sigType(p)
			ft.Params = append(ft.Params, types.Param{Type: pt})
			sym.Params = append(sym.Params, &VarSymbol{SymType: pt, IsParam: true})
		}
		g.Insert(sym)
	}
}

// hipAtomicCall types HIP's generic atomics, __hip_atomic_fetch_add(p, v,
// order, scope) and its kin: the result is what p points to, and the
// order and scope are integer constants the header spells.
func (a *Analyzer) hipAtomicCall(name string, c *ast.CallExpr) (ExprInfo, bool) {
	if !a.offload() || !strings.HasPrefix(name, "__hip_atomic_") {
		return ExprInfo{}, false
	}
	op := strings.TrimPrefix(name, "__hip_atomic_")
	nargs := map[string]int{
		"load": 3, "store": 4, "exchange": 4, "compare_exchange_strong": 6, "compare_exchange_weak": 6,
		"fetch_add": 4, "fetch_sub": 4, "fetch_and": 4, "fetch_or": 4, "fetch_xor": 4, "fetch_min": 4, "fetch_max": 4,
	}
	n, known := nargs[op]
	if !known {
		return ExprInfo{}, false
	}
	var infos []ExprInfo
	for _, arg := range c.Args {
		infos = append(infos, a.CheckExpr(arg))
	}
	void := ExprInfo{Type: types.Typ(types.Void), ValCat: PrValue}
	if len(c.Args) != n {
		a.errorAt(c.Pos(), fmt.Sprintf("%s takes %d arguments, not %d", name, n, len(c.Args)))
		return void, true
	}
	for _, info := range infos {
		if isDependentExpr(info) {
			return dependentExpr(), true
		}
	}
	ptr, isPtr := types.Unqualify(types.RemoveReference(infos[0].Type)).(*types.Pointer)
	if !isPtr {
		a.errorAt(c.Args[0].Pos(), fmt.Sprintf("%s wants a pointer to the object, not %s", name, infos[0].Type))
		return void, true
	}
	elem := types.Unqualify(ptr.Elem)
	switch op {
	case "store":
		return void, true
	case "compare_exchange_strong", "compare_exchange_weak":
		return ExprInfo{Type: types.Typ(types.Bool), ValCat: PrValue}, true
	}
	return ExprInfo{Type: elem, ValCat: PrValue}, true
}
