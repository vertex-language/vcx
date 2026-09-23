/*
 * hip/hip_runtime.h -- vcx's minimal stand-in for ROCm's, read when no
 * ROCm is installed: the runtime API a program needs to move memory and
 * launch kernels, over the same amdhip64 ROCm ships. ROCm's own header
 * takes precedence when it is found.
 */
#ifndef HIP_INCLUDE_HIP_HIP_RUNTIME_H
#define HIP_INCLUDE_HIP_HIP_RUNTIME_H

#include <stddef.h>
#include <hip/__vcx_hip_vector_types.h>

/* A default argument where the header is read as C++, and nothing where
 * it is read as C -- ROCm's own __dparm. */
#if defined(__cplusplus)
#define __vcx_dparm(x) = x
#else
#define __vcx_dparm(x)
#endif

#if defined(__cplusplus)
extern "C" {
#endif

typedef enum hipError_t {
  hipSuccess = 0,
  hipErrorInvalidValue = 1,
  hipErrorOutOfMemory = 2,
  hipErrorNotInitialized = 3,
  hipErrorInvalidConfiguration = 9,
  hipErrorInvalidDevice = 101,
  hipErrorNoDevice = 100,
  hipErrorInvalidDeviceFunction = 98,
  hipErrorLaunchFailure = 719,
  hipErrorUnknown = 999,
} hipError_t;

typedef enum hipMemcpyKind {
  hipMemcpyHostToHost = 0,
  hipMemcpyHostToDevice = 1,
  hipMemcpyDeviceToHost = 2,
  hipMemcpyDeviceToDevice = 3,
  hipMemcpyDefault = 4,
} hipMemcpyKind;

enum hipMemAttach {
  hipMemAttachGlobal = 0x01,
  hipMemAttachHost = 0x02,
  hipMemAttachSingle = 0x04,
};

enum hipHostMallocFlags {
  hipHostMallocDefault = 0x00,
  hipHostMallocPortable = 0x01,
  hipHostMallocMapped = 0x02,
  hipHostMallocWriteCombined = 0x04,
  hipHostMallocCoherent = 0x40000000,
  hipHostMallocNonCoherent = 0x80000000,
};

typedef struct hipDeviceProp_t {
  char name[256];
  size_t totalGlobalMem;
  size_t sharedMemPerBlock;
  int regsPerBlock;
  int warpSize;
  int maxThreadsPerBlock;
  int maxThreadsDim[3];
  int maxGridSize[3];
  int clockRate;
  size_t totalConstMem;
  int major;
  int minor;
  int multiProcessorCount;
  char __vcx_rest[1024];
} hipDeviceProp_t;

typedef struct ihipStream_t *hipStream_t;
typedef struct ihipEvent_t *hipEvent_t;

hipError_t hipMalloc(void **ptr, size_t size);
hipError_t hipFree(void *ptr);
hipError_t hipHostMalloc(void **ptr, size_t size, unsigned int flags);
hipError_t hipHostFree(void *ptr);
hipError_t hipMallocManaged(void **ptr, size_t size, unsigned int flags);
hipError_t hipMemcpy(void *dst, const void *src, size_t sizeBytes, hipMemcpyKind kind);
hipError_t hipMemcpyAsync(void *dst, const void *src, size_t sizeBytes, hipMemcpyKind kind, hipStream_t stream __vcx_dparm(0));
hipError_t hipMemcpyToSymbol(const void *symbol, const void *src, size_t sizeBytes, size_t offset __vcx_dparm(0),
                             hipMemcpyKind kind __vcx_dparm(hipMemcpyHostToDevice));
hipError_t hipMemcpyFromSymbol(void *dst, const void *symbol, size_t sizeBytes, size_t offset __vcx_dparm(0),
                               hipMemcpyKind kind __vcx_dparm(hipMemcpyDeviceToHost));
hipError_t hipGetSymbolAddress(void **devPtr, const void *symbol);
hipError_t hipGetSymbolSize(size_t *size, const void *symbol);
hipError_t hipMemset(void *dst, int value, size_t sizeBytes);
hipError_t hipDeviceSynchronize(void);
hipError_t hipDeviceReset(void);
hipError_t hipGetLastError(void);
hipError_t hipPeekAtLastError(void);
const char *hipGetErrorString(hipError_t error);
const char *hipGetErrorName(hipError_t error);
hipError_t hipGetDeviceCount(int *count);
hipError_t hipGetDevice(int *device);
hipError_t hipGetDeviceProperties(hipDeviceProp_t *prop, int deviceId);
hipError_t hipSetDevice(int device);
hipError_t hipStreamCreate(hipStream_t *stream);
hipError_t hipStreamDestroy(hipStream_t stream);
hipError_t hipStreamSynchronize(hipStream_t stream);
hipError_t hipEventCreate(hipEvent_t *event);
hipError_t hipEventDestroy(hipEvent_t event);
hipError_t hipEventRecord(hipEvent_t event, hipStream_t stream __vcx_dparm(0));
hipError_t hipEventSynchronize(hipEvent_t event);
hipError_t hipEventElapsedTime(float *ms, hipEvent_t start, hipEvent_t stop);
hipError_t hipLaunchKernel(const void *function_address, dim3 numBlocks, dim3 dimBlocks, void **args,
                           size_t sharedMemBytes __vcx_dparm(0), hipStream_t stream __vcx_dparm(0));

#if defined(__cplusplus)
}
#endif

/* The C++ side, as ROCm's own headers have it. The allocators take the
 * address of the caller's pointer, which is a void** in C and is not one
 * in C++, so each is a template that casts; the symbol API names the
 * symbol and passes its shadow's address. */
#if defined(__cplusplus)
template <class T>
static inline hipError_t hipMalloc(T **ptr, size_t size) {
  return hipMalloc((void **)ptr, size);
}
template <class T>
static inline hipError_t hipHostMalloc(T **ptr, size_t size, unsigned int flags = hipHostMallocDefault) {
  return hipHostMalloc((void **)ptr, size, flags);
}
template <class T>
static inline hipError_t hipMallocManaged(T **ptr, size_t size, unsigned int flags = hipMemAttachGlobal) {
  return hipMallocManaged((void **)ptr, size, flags);
}
template <class T>
static inline hipError_t hipMemcpyToSymbol(const T &symbol, const void *src, size_t sizeBytes, size_t offset = 0,
                                           hipMemcpyKind kind = hipMemcpyHostToDevice) {
  return hipMemcpyToSymbol((const void *)&symbol, src, sizeBytes, offset, kind);
}
template <class T>
static inline hipError_t hipMemcpyFromSymbol(void *dst, const T &symbol, size_t sizeBytes, size_t offset = 0,
                                             hipMemcpyKind kind = hipMemcpyDeviceToHost) {
  return hipMemcpyFromSymbol(dst, (const void *)&symbol, sizeBytes, offset, kind);
}
template <class T>
static inline hipError_t hipGetSymbolAddress(void **devPtr, const T &symbol) {
  return hipGetSymbolAddress(devPtr, (const void *)&symbol);
}
template <class T>
static inline hipError_t hipGetSymbolSize(size_t *size, const T &symbol) {
  return hipGetSymbolSize(size, (const void *)&symbol);
}
template <class T>
static inline hipError_t hipLaunchKernel(const T *function_address, dim3 numBlocks, dim3 dimBlocks, void **args,
                                         size_t sharedMemBytes = 0, hipStream_t stream = 0) {
  return hipLaunchKernel((const void *)function_address, numBlocks, dimBlocks, args, sharedMemBytes, stream);
}
#endif

/* HIP_SYMBOL is what a program writes around a __device__ variable it
 * passes to the symbol API. Here the C++ overloads take the variable
 * itself, so it is the variable. */
#define HIP_SYMBOL(x) (x)

#define hipLaunchKernelGGL(kernelName, numBlocks, numThreads, memPerBlock, streamId, ...) \
  kernelName<<<(numBlocks), (numThreads), (memPerBlock), (streamId)>>>(__VA_ARGS__)

#endif /* HIP_INCLUDE_HIP_HIP_RUNTIME_H */
