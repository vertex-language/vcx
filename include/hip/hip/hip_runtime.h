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

typedef struct ihipStream_t *hipStream_t;
typedef struct ihipEvent_t *hipEvent_t;

hipError_t hipMalloc(void **ptr, size_t size);
hipError_t hipFree(void *ptr);
hipError_t hipHostMalloc(void **ptr, size_t size, unsigned int flags);
hipError_t hipHostFree(void *ptr);
hipError_t hipMallocManaged(void **ptr, size_t size, unsigned int flags);
hipError_t hipMemcpy(void *dst, const void *src, size_t sizeBytes, hipMemcpyKind kind);
hipError_t hipMemcpyAsync(void *dst, const void *src, size_t sizeBytes, hipMemcpyKind kind, hipStream_t stream);
hipError_t hipMemset(void *dst, int value, size_t sizeBytes);
hipError_t hipDeviceSynchronize(void);
hipError_t hipDeviceReset(void);
hipError_t hipGetLastError(void);
hipError_t hipPeekAtLastError(void);
const char *hipGetErrorString(hipError_t error);
const char *hipGetErrorName(hipError_t error);
hipError_t hipGetDeviceCount(int *count);
hipError_t hipGetDevice(int *device);
hipError_t hipSetDevice(int device);
hipError_t hipStreamCreate(hipStream_t *stream);
hipError_t hipStreamDestroy(hipStream_t stream);
hipError_t hipStreamSynchronize(hipStream_t stream);
hipError_t hipEventCreate(hipEvent_t *event);
hipError_t hipEventDestroy(hipEvent_t event);
hipError_t hipEventRecord(hipEvent_t event, hipStream_t stream);
hipError_t hipEventSynchronize(hipEvent_t event);
hipError_t hipEventElapsedTime(float *ms, hipEvent_t start, hipEvent_t stop);
hipError_t hipLaunchKernel(const void *function_address, dim3 numBlocks, dim3 dimBlocks, void **args, size_t sharedMemBytes, hipStream_t stream);

#if defined(__cplusplus)
}
#endif

#define hipLaunchKernelGGL(kernelName, numBlocks, numThreads, memPerBlock, streamId, ...) \
  kernelName<<<(numBlocks), (numThreads), (memPerBlock), (streamId)>>>(__VA_ARGS__)

#endif /* HIP_INCLUDE_HIP_HIP_RUNTIME_H */
