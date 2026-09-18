// __shared__ storage, __syncthreads, and the atomics over __hip_atomic_*.
// grid: 2
// block: 8
// expect: 36 16 15
#include <hip/hip_runtime.h>

__global__ void test(int *out) {
  __shared__ int tile[8];
  int t = threadIdx.x;
  tile[t] = t + 1;
  __syncthreads();
  for (int s = 4; s > 0; s >>= 1) {
    if (t < s) tile[t] += tile[t + s];
    __syncthreads();
  }
  if (t == 0 && blockIdx.x == 0) out[0] = tile[0];
  atomicAdd(&out[1], 1);
  atomicMax(&out[2], (int)(blockIdx.x * blockDim.x + t));
}
