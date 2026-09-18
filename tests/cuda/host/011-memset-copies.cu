// cudaMemset, device-to-device copies, a 2-D launch with dim3 arithmetic
// on the host, and a kernel template on a non-type parameter.
// expect: 0 0 0 0 | 5 6 7 8 | 5 6 7 8 | corners 0 3 12 15
#include <cuda_runtime.h>
#include <stdio.h>

template <int N> __global__ void addN(int *p) { p[threadIdx.x] += N; }
__global__ void grid2d(int *p, int w) {
  int x = blockIdx.x * blockDim.x + threadIdx.x;
  int y = blockIdx.y * blockDim.y + threadIdx.y;
  p[y * w + x] = y * w + x;
}

int main() {
  int *a, *b, *g, h[16];
  cudaMalloc((void **)&a, 4 * sizeof(int));
  cudaMalloc((void **)&b, 4 * sizeof(int));
  cudaMalloc((void **)&g, 16 * sizeof(int));
  cudaMemset(a, 0, 4 * sizeof(int));
  cudaMemcpy(h, a, 4 * sizeof(int), cudaMemcpyDeviceToHost);
  printf("%d %d %d %d | ", h[0], h[1], h[2], h[3]);
  int five[4] = {1, 2, 3, 4};
  cudaMemcpy(a, five, sizeof five, cudaMemcpyHostToDevice);
  addN<4><<<1, 4>>>(a);
  cudaMemcpy(b, a, 4 * sizeof(int), cudaMemcpyDeviceToDevice);
  cudaMemcpy(h, a, 4 * sizeof(int), cudaMemcpyDeviceToHost);
  printf("%d %d %d %d | ", h[0], h[1], h[2], h[3]);
  cudaMemcpy(h, b, 4 * sizeof(int), cudaMemcpyDeviceToHost);
  printf("%d %d %d %d | ", h[0], h[1], h[2], h[3]);
  const int w = 4;
  dim3 block(2, 2);
  dim3 grid((w + block.x - 1) / block.x, (w + block.y - 1) / block.y);
  grid2d<<<grid, block>>>(g, w);
  cudaMemcpy(h, g, sizeof h, cudaMemcpyDeviceToHost);
  printf("corners %d %d %d %d\n", h[0], h[3], h[12], h[15]);
  return 0;
}
