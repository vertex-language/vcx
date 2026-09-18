// Atomics on global memory: 64 threads each add 1 and take the max of
// their id; the count and the maximum come back.
// grid: 2
// block: 32
// expect: 64 63
__global__ void test(int *out) {
  int i = blockIdx.x * blockDim.x + threadIdx.x;
  atomicAdd(&out[0], 1);
  atomicMax(&out[1], i);
}
