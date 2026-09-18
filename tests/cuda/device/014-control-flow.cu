// Divergent control flow: a switch, loops with break and continue, an
// early return in some threads, and a barrier every thread reaches.
// grid: 1
// block: 8
// expect: 100 11 22 33 4 500 6 7  36
__global__ void test(int *out) {
  int t = threadIdx.x;
  int v = 0;
  switch (t) {
  case 0: v = 100; break;
  case 1: case 2: case 3: v = t * 11; break;
  case 5: v = 500; break;
  default: v = t;
  }
  __shared__ int sum[8];
  sum[t] = t + 1;
  __syncthreads();
  if (t == 0) {
    int s = 0;
    for (int i = 0;; i++) {
      if (i >= 8) break;
      if (i == 100) continue;
      s += sum[i];
    }
    out[8] = s;
  }
  if (t == 4) {
    out[t] = v;
    return;
  }
  out[t] = v;
}
