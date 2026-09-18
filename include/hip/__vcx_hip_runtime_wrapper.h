/*
 * __vcx_hip_runtime_wrapper.h -- read before the first line of every HIP
 * unit, as clang reads its __clang_hip_runtime_wrapper.h. The execution
 * spaces, the builtin variables, synchronization, the warp primitives
 * and the atomics, over the __builtin_amdgcn_* and __hip_atomic_*
 * builtins the device pass lowers to VIR's §W and §H verbs.
 *
 * When ROCm is installed its own hip/hip_runtime.h is read after this;
 * when it is not, vcx's minimal one stands in.
 */
#ifndef __VCX_HIP_RUNTIME_WRAPPER_H__
#define __VCX_HIP_RUNTIME_WRAPPER_H__

#if defined(__HIP__)

/* ---- execution and memory spaces ------------------------------------- */

#define __host__ __attribute__((host))
#define __device__ __attribute__((device))
#define __global__ __attribute__((global))
#define __shared__ __attribute__((shared))
#define __constant__ __attribute__((constant))
#define __managed__ __attribute__((managed))
#define __launch_bounds__(...) __attribute__((launch_bounds(__VA_ARGS__)))
#define __forceinline__ __inline__ __attribute__((always_inline))
#define __noinline__ __attribute__((noinline))
#define __align__(n) __attribute__((aligned(n)))

#define __VCX_DEVICE_INLINE static __inline__ __attribute__((always_inline)) __device__
#define __VCX_BOTH_INLINE static __inline__ __attribute__((always_inline)) __host__ __device__

/* The memory orders, which a GNU-dialect host predefines and an MSVC
 * one does not; the atomics below spell them. */
#ifndef __ATOMIC_RELAXED
#define __ATOMIC_RELAXED 0
#define __ATOMIC_CONSUME 1
#define __ATOMIC_ACQUIRE 2
#define __ATOMIC_RELEASE 3
#define __ATOMIC_ACQ_REL 4
#define __ATOMIC_SEQ_CST 5
#endif

/* ---- the vector types and dim3 ---------------------------------------- */

#include <hip/__vcx_hip_vector_types.h>

/* ---- the builtin variables -------------------------------------------- */

/* Declared as ROCm's declares them, and read from the hardware: the
 * device pass knows these five names and lowers threadIdx.x to the
 * work-item's id rather than a load. */
extern const uint3 threadIdx;
extern const uint3 blockIdx;
extern const dim3 blockDim;
extern const dim3 gridDim;
extern const int warpSize;
#if defined(__HIP_DEVICE_COMPILE__)
#define hipThreadIdx_x (__builtin_amdgcn_workitem_id_x())
#define hipThreadIdx_y (__builtin_amdgcn_workitem_id_y())
#define hipThreadIdx_z (__builtin_amdgcn_workitem_id_z())
#define hipBlockIdx_x (__builtin_amdgcn_workgroup_id_x())
#define hipBlockIdx_y (__builtin_amdgcn_workgroup_id_y())
#define hipBlockIdx_z (__builtin_amdgcn_workgroup_id_z())
#define hipBlockDim_x (__builtin_amdgcn_workgroup_size_x())
#define hipBlockDim_y (__builtin_amdgcn_workgroup_size_y())
#define hipBlockDim_z (__builtin_amdgcn_workgroup_size_z())
#define hipGridDim_x (__builtin_amdgcn_grid_size_x() / __builtin_amdgcn_workgroup_size_x())
#define hipGridDim_y (__builtin_amdgcn_grid_size_y() / __builtin_amdgcn_workgroup_size_y())
#define hipGridDim_z (__builtin_amdgcn_grid_size_z() / __builtin_amdgcn_workgroup_size_z())
#endif

/* ---- synchronization and fences -------------------------------------- */

__VCX_DEVICE_INLINE void __syncthreads(void) { __builtin_amdgcn_s_barrier(); }
__VCX_DEVICE_INLINE void __threadfence_block(void) { __builtin_amdgcn_fence(__ATOMIC_SEQ_CST, "workgroup"); }
__VCX_DEVICE_INLINE void __threadfence(void) { __builtin_amdgcn_fence(__ATOMIC_SEQ_CST, "agent"); }
__VCX_DEVICE_INLINE void __threadfence_system(void) { __builtin_amdgcn_fence(__ATOMIC_SEQ_CST, ""); }

/* ---- warp primitives -------------------------------------------------- */

__VCX_DEVICE_INLINE unsigned __lane_id(void) {
  return __builtin_amdgcn_mbcnt_hi(~0u, __builtin_amdgcn_mbcnt_lo(~0u, 0u));
}
__VCX_DEVICE_INLINE int __shfl(int var, int srcLane, int width = warpSize) {
  unsigned lane = __lane_id();
  unsigned j = (lane & ~(unsigned)(width - 1)) | ((unsigned)srcLane & (unsigned)(width - 1));
  return __builtin_amdgcn_ds_bpermute((int)(j << 2), var);
}
__VCX_DEVICE_INLINE unsigned __shfl(unsigned var, int srcLane, int width = warpSize) { return (unsigned)__shfl((int)var, srcLane, width); }
__VCX_DEVICE_INLINE float __shfl(float var, int srcLane, int width = warpSize) {
  union { float f; int i; } u; u.f = var; u.i = __shfl(u.i, srcLane, width); return u.f;
}
__VCX_DEVICE_INLINE int __shfl_up(int var, unsigned delta, int width = warpSize) {
  unsigned lane = __lane_id();
  unsigned base = lane & ~(unsigned)(width - 1);
  unsigned j = lane - delta < base || delta > lane ? lane : lane - delta;
  return __builtin_amdgcn_ds_bpermute((int)(j << 2), var);
}
__VCX_DEVICE_INLINE int __shfl_down(int var, unsigned delta, int width = warpSize) {
  unsigned lane = __lane_id();
  unsigned top = lane | (unsigned)(width - 1);
  unsigned j = lane + delta > top ? lane : lane + delta;
  return __builtin_amdgcn_ds_bpermute((int)(j << 2), var);
}
__VCX_DEVICE_INLINE int __shfl_xor(int var, int laneMask, int width = warpSize) {
  unsigned lane = __lane_id();
  unsigned j = lane ^ (unsigned)laneMask;
  if (j > (lane | (unsigned)(width - 1))) j = lane;
  return __builtin_amdgcn_ds_bpermute((int)(j << 2), var);
}
__VCX_DEVICE_INLINE float __shfl_down(float var, unsigned delta, int width = warpSize) {
  union { float f; int i; } u; u.f = var; u.i = __shfl_down(u.i, delta, width); return u.f;
}
__VCX_DEVICE_INLINE float __shfl_xor(float var, int laneMask, int width = warpSize) {
  union { float f; int i; } u; u.f = var; u.i = __shfl_xor(u.i, laneMask, width); return u.f;
}
__VCX_DEVICE_INLINE unsigned long long __ballot(int pred) { return __builtin_amdgcn_uicmp(pred, 0, 33); }
__VCX_DEVICE_INLINE int __all(int pred) { return __builtin_amdgcn_uicmp(pred, 0, 33) == __builtin_amdgcn_read_exec(); }
__VCX_DEVICE_INLINE int __any(int pred) { return __builtin_amdgcn_uicmp(pred, 0, 33) != 0; }
__VCX_DEVICE_INLINE int __popc(unsigned x) { return __builtin_popcount(x); }
__VCX_DEVICE_INLINE int __popcll(unsigned long long x) { return __builtin_popcountll(x); }
__VCX_DEVICE_INLINE int __clz(int x) { return x == 0 ? 32 : __builtin_clz((unsigned)x); }
__VCX_DEVICE_INLINE int __clzll(long long x) { return x == 0 ? 64 : __builtin_clzll((unsigned long long)x); }
__VCX_DEVICE_INLINE int __ffs(int x) { return x == 0 ? 0 : __builtin_ctz((unsigned)x) + 1; }
__VCX_DEVICE_INLINE int __ffsll(long long x) { return x == 0 ? 0 : __builtin_ctzll((unsigned long long)x) + 1; }

/* ---- atomics ---------------------------------------------------------- */

#include <hip/__vcx_hip_atomics.h>

/* ---- the device math library ------------------------------------------ */

#include <hip/__vcx_hip_math.h>
#include <hip/__vcx_devicelib_math.h>

/* ---- the host API ----------------------------------------------------- */

#include <hip/hip_runtime.h>

#endif /* __HIP__ */
#endif /* __VCX_HIP_RUNTIME_WRAPPER_H__ */
