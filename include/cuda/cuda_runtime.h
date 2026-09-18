/*
 * cuda_runtime.h -- vcx's minimal stand-in for the toolkit's, read when
 * no CUDA toolkit is installed: the runtime API a program needs to move
 * memory and launch kernels, declared as the toolkit declares it, over
 * the same cudart the toolkit ships. It claims the toolkit header's
 * guard, and the toolkit's own header takes precedence when it is found.
 */
#ifndef __CUDA_RUNTIME_H__
#define __CUDA_RUNTIME_H__

#include <stddef.h>
#include <__vcx_cuda_vector_types.h>

#if defined(__cplusplus)
extern "C" {
#endif

typedef enum cudaError {
  cudaSuccess = 0,
  cudaErrorInvalidValue = 1,
  cudaErrorMemoryAllocation = 2,
  cudaErrorInitializationError = 3,
  cudaErrorInvalidConfiguration = 9,
  cudaErrorInvalidDevice = 101,
  cudaErrorNoDevice = 100,
  cudaErrorInvalidDeviceFunction = 98,
  cudaErrorLaunchFailure = 719,
  cudaErrorUnknown = 999,
} cudaError_t;

typedef enum cudaMemcpyKind {
  cudaMemcpyHostToHost = 0,
  cudaMemcpyHostToDevice = 1,
  cudaMemcpyDeviceToHost = 2,
  cudaMemcpyDeviceToDevice = 3,
  cudaMemcpyDefault = 4,
} cudaMemcpyKind;

typedef struct CUstream_st *cudaStream_t;
typedef struct CUevent_st *cudaEvent_t;

struct cudaDeviceProp {
  char name[256];
  unsigned char uuid[16];
  size_t totalGlobalMem;
  size_t sharedMemPerBlock;
  int regsPerBlock;
  int warpSize;
  size_t memPitch;
  int maxThreadsPerBlock;
  int maxThreadsDim[3];
  int maxGridSize[3];
  int clockRate;
  size_t totalConstMem;
  int major;
  int minor;
  int multiProcessorCount;
  char __vcx_rest[1024];
};

cudaError_t cudaMalloc(void **devPtr, size_t size);
cudaError_t cudaFree(void *devPtr);
cudaError_t cudaMallocHost(void **ptr, size_t size);
cudaError_t cudaFreeHost(void *ptr);
cudaError_t cudaMallocManaged(void **devPtr, size_t size, unsigned int flags);
cudaError_t cudaMemcpy(void *dst, const void *src, size_t count, cudaMemcpyKind kind);
cudaError_t cudaMemcpyAsync(void *dst, const void *src, size_t count, cudaMemcpyKind kind, cudaStream_t stream);
cudaError_t cudaMemset(void *devPtr, int value, size_t count);
cudaError_t cudaMemcpyToSymbol(const void *symbol, const void *src, size_t count, size_t offset, cudaMemcpyKind kind);
cudaError_t cudaMemcpyFromSymbol(void *dst, const void *symbol, size_t count, size_t offset, cudaMemcpyKind kind);
cudaError_t cudaDeviceSynchronize(void);
cudaError_t cudaDeviceReset(void);
cudaError_t cudaGetLastError(void);
cudaError_t cudaPeekAtLastError(void);
const char *cudaGetErrorString(cudaError_t error);
const char *cudaGetErrorName(cudaError_t error);
cudaError_t cudaGetDeviceCount(int *count);
cudaError_t cudaGetDevice(int *device);
cudaError_t cudaSetDevice(int device);
cudaError_t cudaGetDeviceProperties(struct cudaDeviceProp *prop, int device);
cudaError_t cudaStreamCreate(cudaStream_t *stream);
cudaError_t cudaStreamDestroy(cudaStream_t stream);
cudaError_t cudaStreamSynchronize(cudaStream_t stream);
cudaError_t cudaEventCreate(cudaEvent_t *event);
cudaError_t cudaEventDestroy(cudaEvent_t event);
cudaError_t cudaEventRecord(cudaEvent_t event, cudaStream_t stream);
cudaError_t cudaEventSynchronize(cudaEvent_t event);
cudaError_t cudaEventElapsedTime(float *ms, cudaEvent_t start, cudaEvent_t end);
cudaError_t cudaLaunchKernel(const void *func, dim3 gridDim, dim3 blockDim, void **args, size_t sharedMem, cudaStream_t stream);

#if defined(__cplusplus)
}
#endif

/* The vector constructors of vector_functions.h. */
#define __VCX_MAKE1(T, E) __VCX_BOTH_INLINE T make_##T(E x) { T v; v.x = x; return v; }
#define __VCX_MAKE2(T, E) __VCX_BOTH_INLINE T make_##T(E x, E y) { T v; v.x = x; v.y = y; return v; }
#define __VCX_MAKE3(T, E) __VCX_BOTH_INLINE T make_##T(E x, E y, E z) { T v; v.x = x; v.y = y; v.z = z; return v; }
#define __VCX_MAKE4(T, E) __VCX_BOTH_INLINE T make_##T(E x, E y, E z, E w) { T v; v.x = x; v.y = y; v.z = z; v.w = w; return v; }
#define __VCX_MAKE(E, N)  __VCX_MAKE1(N##1, E) __VCX_MAKE2(N##2, E) __VCX_MAKE3(N##3, E) __VCX_MAKE4(N##4, E)
__VCX_MAKE(signed char, char)
__VCX_MAKE(unsigned char, uchar)
__VCX_MAKE(short, short)
__VCX_MAKE(unsigned short, ushort)
__VCX_MAKE(int, int)
__VCX_MAKE(unsigned int, uint)
__VCX_MAKE(long, long)
__VCX_MAKE(unsigned long, ulong)
__VCX_MAKE(long long, longlong)
__VCX_MAKE(unsigned long long, ulonglong)
__VCX_MAKE(float, float)
__VCX_MAKE(double, double)
#undef __VCX_MAKE
#undef __VCX_MAKE1
#undef __VCX_MAKE2
#undef __VCX_MAKE3
#undef __VCX_MAKE4

#endif /* __CUDA_RUNTIME_H__ */
