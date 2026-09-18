// The reducing barriers: how many threads hold a predicate, whether all
// do, whether any does.
// grid: 1
// block: 32
// expect: 16 0 1 32 1 1
__global__ void test(int *out) {
  int t = threadIdx.x;
  int even = (t & 1) == 0;
  int n = __syncthreads_count(even);
  int all = __syncthreads_and(even);
  int any = __syncthreads_or(even);
  int n2 = __syncthreads_count(1);
  int all2 = __syncthreads_and(t < 32);
  int any2 = __syncthreads_or(t == 31);
  if (t == 0) {
    out[0] = n; out[1] = all; out[2] = any; out[3] = n2; out[4] = all2; out[5] = any2;
  }
}
