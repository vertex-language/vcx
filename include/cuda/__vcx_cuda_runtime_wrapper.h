/*
 * __vcx_cuda_runtime_wrapper.h -- read before the first line of every
 * CUDA unit, the way clang reads its __clang_cuda_runtime_wrapper.h.
 *
 * It gives the unit what nvcc gives one without an #include: the
 * execution-space and memory-space keywords, dim3 and the vector types,
 * the builtin variables, the synchronization and warp primitives, the
 * atomics, and the device math library. Everything a kernel reaches
 * bottoms out in a compiler builtin -- __nvvm_* for the device, as
 * clang names them -- that the device pass lowers to VIR's §W verbs.
 *
 * When the CUDA toolkit is installed its own cuda_runtime.h is read after
 * this one; when it is not, vcx's minimal cuda_runtime.h stands in for
 * the host API the toolkit's headers would declare.
 */
#ifndef __VCX_CUDA_RUNTIME_WRAPPER_H__
#define __VCX_CUDA_RUNTIME_WRAPPER_H__

#if defined(__CUDACC__)

/* ---- execution and memory spaces ------------------------------------- */

#define __host__ __attribute__((host))
#define __device__ __attribute__((device))
#define __global__ __attribute__((global))
#define __shared__ __attribute__((shared))
#define __constant__ __attribute__((constant))
#define __managed__ __attribute__((managed))
#define __grid_constant__ __attribute__((grid_constant))
#define __launch_bounds__(...) __attribute__((launch_bounds(__VA_ARGS__)))
#define __forceinline__ __inline__ __attribute__((always_inline))
#define __noinline__ __attribute__((noinline))
#define __align__(n) __attribute__((aligned(n)))
#define __builtin_align__(n) __align__(n)
#define __restrict__ __restrict

#define __host_device__ __host__ __device__
#define __VCX_DEVICE_INLINE static __inline__ __attribute__((always_inline)) __device__
#define __VCX_BOTH_INLINE static __inline__ __attribute__((always_inline)) __host__ __device__

/* ---- the vector types and dim3 ---------------------------------------- */

#include <__vcx_cuda_vector_types.h>

/* ---- the builtin variables -------------------------------------------- */

/* Declared as the toolkit's device_launch_parameters.h declares them,
 * and read from the hardware: the device pass knows these five names
 * and lowers threadIdx.x to the work-item's id rather than a load. The
 * guard is the toolkit header's, so that the two are never both read. */
#ifndef __DEVICE_LAUNCH_PARAMETERS_H__
#define __DEVICE_LAUNCH_PARAMETERS_H__
extern const uint3 threadIdx;
extern const uint3 blockIdx;
extern const dim3 blockDim;
extern const dim3 gridDim;
extern const int warpSize;
#endif

/* ---- synchronization and fences -------------------------------------- */

__VCX_DEVICE_INLINE void __syncthreads(void) { __nvvm_barrier0(); }
__VCX_DEVICE_INLINE int __syncthreads_count(int p) { return __nvvm_barrier0_popc(p); }
__VCX_DEVICE_INLINE int __syncthreads_and(int p) { return __nvvm_barrier0_and(p); }
__VCX_DEVICE_INLINE int __syncthreads_or(int p) { return __nvvm_barrier0_or(p); }
__VCX_DEVICE_INLINE void __syncwarp(unsigned mask = 0xffffffffu) { __nvvm_bar_warp_sync(mask); }
__VCX_DEVICE_INLINE void __threadfence_block(void) { __nvvm_membar_cta(); }
__VCX_DEVICE_INLINE void __threadfence(void) { __nvvm_membar_gl(); }
__VCX_DEVICE_INLINE void __threadfence_system(void) { __nvvm_membar_sys(); }

/* ---- warp primitives -------------------------------------------------- */

#include <__vcx_cuda_warp.h>

/* ---- atomics ---------------------------------------------------------- */

#include <__vcx_cuda_atomics.h>

/* ---- the device math library ------------------------------------------ */

#include <__vcx_cuda_math.h>
#include <__vcx_devicelib_math.h>

/* ---- the host API ----------------------------------------------------- */

#include <cuda_runtime.h>

#endif /* __CUDACC__ */
#endif /* __VCX_CUDA_RUNTIME_WRAPPER_H__ */
