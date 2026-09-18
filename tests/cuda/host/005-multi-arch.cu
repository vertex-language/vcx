// Several --offload-arch values in one fat binary: the runtime loads the
// newest the device runs, and __CUDA_ARCH__ told each pass which it was.
// arch: sm_52 sm_75 sm_90
// expect: arch = 750
#include <cuda_runtime.h>
#include <stdio.h>

__global__ void which(int *out) {
#if defined(__CUDA_ARCH__)
  *out = __CUDA_ARCH__;
#else
  *out = -1;
#endif
}

int main() {
  int *d, v = 0;
  cudaMalloc((void **)&d, sizeof(int));
  which<<<1, 1>>>(d);
  if (cudaDeviceSynchronize() != cudaSuccess) {
    printf("launch failed: %s\n", cudaGetErrorString(cudaGetLastError()));
    return 1;
  }
  cudaMemcpy(&v, d, sizeof v, cudaMemcpyDeviceToHost);
  printf("arch = %d\n", v);
  return 0;
}
