// The builtin variables on AMD: every thread of a 2-block launch writes
// its global index, blockDim and gridDim into its slot.
// grid: 2
// block: 4
// expect: 0 1 2 3 4 5 6 7  4 4 4 4 4 4 4 4  2 2 2 2 2 2 2 2
#include <hip/hip_runtime.h>

__global__ void test(int *out) {
  int i = blockIdx.x * blockDim.x + threadIdx.x;
  out[i] = i;
  out[8 + i] = blockDim.x;
  out[16 + i] = gridDim.x;
}
