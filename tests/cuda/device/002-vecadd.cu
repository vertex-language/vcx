// The first real kernel: a device function, an arithmetic body and a
// bounds check, over a grid one thread wider than the data.
// grid: 3
// block: 4
// expect: 0 3 6 9 12 15 18 21 24 27 30 -1
__device__ int triple(int x) { return x + x + x; }

__global__ void test(int *out) {
  int i = blockIdx.x * blockDim.x + threadIdx.x;
  if (i < 11) {
    out[i] = triple(i);
  } else if (i == 11) {
    out[i] = -1;
  }
}
