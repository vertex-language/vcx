// vcx_cudart_glue.cpp -- the two entry points a compiled unit calls that
// NVIDIA's cudart does not have, forwarded to the ones it does. A launch
// stub and a launch site pass the configuration as six words and a
// size, where cudart takes two dim3 by value; this is where the dim3s
// are made. Linked when the program links against the toolkit's cudart
// (-cudart static or shared) rather than vcx's own, which defines these
// itself.

#include <cuda_runtime.h>

extern "C" unsigned __cudaPushCallConfiguration(dim3 grid, dim3 block, size_t shmem, void *stream);

extern "C" unsigned __vcx_cudaPushCallConfiguration(unsigned gx, unsigned gy, unsigned gz, unsigned bx, unsigned by,
                                                    unsigned bz, size_t shmem, void *stream) {
  return __cudaPushCallConfiguration(dim3(gx, gy, gz), dim3(bx, by, bz), shmem, stream);
}

extern "C" cudaError_t __vcx_cudaLaunch(const void *func, unsigned gx, unsigned gy, unsigned gz, unsigned bx, unsigned by,
                                        unsigned bz, void **args, size_t shmem, cudaStream_t stream) {
  return cudaLaunchKernel(func, dim3(gx, gy, gz), dim3(bx, by, bz), args, shmem, stream);
}
