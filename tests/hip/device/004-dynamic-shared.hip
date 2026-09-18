#include <hip/hip_runtime.h>
// Dynamic shared memory: extern __shared__ sized by the launch, and a
// second extern __shared__ that names the same storage.
// grid: 1
// block: 8
// shmem: 64
// expect: 3 6 9 12 15 18 21 0
extern __shared__ int buf[];
extern __shared__ float same[];

__global__ void test(int *out) {
  int t = threadIdx.x;
  buf[t] = t * 3;
  __syncthreads();
  out[t] = buf[(t + 1) % blockDim.x];
  if (t == 0 && (float *)buf != same) out[7] = -1;
}
