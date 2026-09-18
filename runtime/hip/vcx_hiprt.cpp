// vcx_hiprt.cpp -- the HIP runtime glue a program built by vcx links:
// the two entry points a compiled unit calls where ROCm's own take a
// dim3 by value, forwarded to ROCm's. Everything else -- the
// registration, the launch, the memory API -- is libamdhip64's, which
// is the HIP runtime and driver in one, and which the link names with
// -lamdhip64 (or which ROCm discovery adds, once it exists).

#include <hip/hip_runtime.h>

extern "C" hipError_t __hipPushCallConfiguration(dim3 grid, dim3 block, size_t shmem, hipStream_t stream);

extern "C" unsigned __vcx_hipPushCallConfiguration(unsigned gx, unsigned gy, unsigned gz, unsigned bx, unsigned by,
                                                   unsigned bz, size_t shmem, void *stream) {
  return __hipPushCallConfiguration(dim3(gx, gy, gz), dim3(bx, by, bz), shmem, (hipStream_t)stream);
}

extern "C" hipError_t __vcx_hipLaunch(const void *func, unsigned gx, unsigned gy, unsigned gz, unsigned bx, unsigned by,
                                      unsigned bz, void **args, size_t shmem, hipStream_t stream) {
  return hipLaunchKernel(func, dim3(gx, gy, gz), dim3(bx, by, bz), args, shmem, stream);
}
