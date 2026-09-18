// Errors: a launch with a block too large is refused and reported by
// cudaGetLastError, which then clears; a good launch after it succeeds;
// the device is counted and named without crashing.
// sanitizer: skip -- the bad launch is the point
// expect: bad launch: invalid argument
// expect: cleared: no error
// expect: good = 7
// expect: devices >= 1: 1 name nonempty: 1 major >= 5: 1
#include <cuda_runtime.h>
#include <stdio.h>
#include <string.h>

__global__ void seven(int *p) { *p = 7; }

int main() {
  int *d, v = 0;
  cudaMalloc((void **)&d, sizeof(int));
  seven<<<1, 4096>>>(d);
  cudaError_t e = cudaGetLastError();
  printf("bad launch: %s\n", cudaGetErrorString(e));
  printf("cleared: %s\n", cudaGetErrorString(cudaGetLastError()));
  seven<<<1, 1>>>(d);
  cudaDeviceSynchronize();
  cudaMemcpy(&v, d, sizeof v, cudaMemcpyDeviceToHost);
  printf("good = %d\n", v);
  int n = 0;
  cudaGetDeviceCount(&n);
  cudaDeviceProp prop;
  memset(&prop, 0, sizeof prop);
  cudaGetDeviceProperties(&prop, 0);
  printf("devices >= 1: %d name nonempty: %d major >= 5: %d\n", n >= 1, prop.name[0] != 0, prop.major >= 5);
  return 0;
}
