// Initialized __constant__ and __device__ data read on the device, a
// local array, and a struct in registers.
// grid: 1
// block: 4
// expect: 10 21 32 43 60
__constant__ int table[4] = {10, 20, 30, 40};
__device__ int offsets[4] = {0, 1, 2, 3};

struct Pair {
  int a, b;
};

__global__ void test(int *out) {
  int t = threadIdx.x;
  int local[4] = {1, 2, 3, 4};
  Pair p = {table[t], offsets[t]};
  out[t] = p.a + p.b;
  if (t == 0) {
    int s = 0;
    for (int i = 0; i < 4; i++) s += local[i] * 6;
    out[4] = s;
  }
}
