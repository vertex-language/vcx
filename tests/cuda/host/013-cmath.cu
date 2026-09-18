// <cmath> on both sides: std::sqrt, std::exp, std::sin and the
// classifications in a kernel and in main, each scaled to an integer.
// expect: device 4000 2718 841 1 host 4000 2718 841 1
#include <cuda_runtime.h>
#include <cmath>
#include <stdio.h>

__host__ __device__ void fill(int *out, float x) {
  out[0] = (int)(std::sqrt(x) * 1000.0f + 0.5f);
  out[1] = (int)(std::exp(1.0f) * 1000.0f + 0.5f);
  out[2] = (int)(std::sin(1.0) * 1000.0 + 0.5);
  out[3] = std::isfinite(x) && !std::isnan(x) ? 1 : 0;
}

__global__ void kernel(int *out, float x) { fill(out, x); }

int main() {
  int *d, h[4], m[4];
  cudaMalloc((void **)&d, sizeof h);
  kernel<<<1, 1>>>(d, 16.0f);
  cudaDeviceSynchronize();
  cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost);
  fill(m, 16.0f);
  printf("device %d %d %d %d host %d %d %d %d\n", h[0], h[1], h[2], h[3], m[0], m[1], m[2], m[3]);
  return 0;
}
