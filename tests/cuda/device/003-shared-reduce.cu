// __shared__ storage and __syncthreads: a block of 8 sums 1..8 by a
// tree reduction in shared memory, and thread 0 writes the total.
// grid: 1
// block: 8
// expect: 36
__global__ void test(int *out) {
  __shared__ int tile[8];
  int t = threadIdx.x;
  tile[t] = t + 1;
  __syncthreads();
  for (int s = 4; s > 0; s >>= 1) {
    if (t < s) tile[t] += tile[t + s];
    __syncthreads();
  }
  if (t == 0) out[0] = tile[0];
}
