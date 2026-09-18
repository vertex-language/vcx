// A class with __host__ __device__ methods used on both sides, a static
// __device__ helper, device-side arrays of structs, and a kernel that
// takes a pointer to them.
// expect: device 14 26 38 host 14 26 38
#include <cuda_runtime.h>
#include <stdio.h>

struct Particle {
  float x, v;
  __host__ __device__ void step(float dt) { x += v * dt; }
  __host__ __device__ int rounded() const { return (int)(x + 0.5f); }
};

static __device__ float speedup(float v) { return v * 2.0f; }

__global__ void advance(Particle *ps, int n, float dt) {
  int i = threadIdx.x;
  if (i < n) {
    ps[i].v = speedup(ps[i].v);
    ps[i].step(dt);
  }
}

int main() {
  Particle h[3] = {{10.0f, 1.0f}, {20.0f, 1.5f}, {30.0f, 2.0f}};
  Particle *d;
  cudaMalloc((void **)&d, sizeof h);
  cudaMemcpy(d, h, sizeof h, cudaMemcpyHostToDevice);
  advance<<<1, 3>>>(d, 3, 2.0f);
  cudaDeviceSynchronize();
  Particle r[3];
  cudaMemcpy(r, d, sizeof r, cudaMemcpyDeviceToHost);
  printf("device %d %d %d ", r[0].rounded(), r[1].rounded(), r[2].rounded());
  for (int i = 0; i < 3; i++) {
    h[i].v *= 2.0f;
    h[i].step(2.0f);
  }
  printf("host %d %d %d\n", h[0].rounded(), h[1].rounded(), h[2].rounded());
  return 0;
}
