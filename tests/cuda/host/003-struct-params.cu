// Kernel parameters passed by value: a struct of mixed fields, a float4,
// and a dim3, read on the device and written back.
// expect: 7 42 3 1 2 3 4 5 6
#include <cuda_runtime.h>
#include <stdio.h>

struct Params {
  int a;
  char tag;
  double d;
  int *out;
};

__global__ void kernel(Params p, float4 v, dim3 extent) {
  if (threadIdx.x == 0) {
    p.out[0] = p.a;
    p.out[1] = p.tag;
    p.out[2] = (int)p.d;
    p.out[3] = (int)v.x;
    p.out[4] = (int)v.y;
    p.out[5] = (int)v.z;
    p.out[6] = (int)v.w;
    p.out[7] = extent.y;
    p.out[8] = extent.z;
  }
}

int main() {
  int *d;
  cudaMalloc((void **)&d, 9 * sizeof(int));
  Params p;
  p.a = 7;
  p.tag = 42;
  p.d = 3.5;
  p.out = d;
  kernel<<<1, 32>>>(p, make_float4(1.0f, 2.0f, 3.0f, 4.0f), dim3(4, 5, 6));
  if (cudaDeviceSynchronize() != cudaSuccess) {
    printf("launch failed: %s\n", cudaGetErrorString(cudaGetLastError()));
    return 1;
  }
  int r[9];
  cudaMemcpy(r, d, sizeof r, cudaMemcpyDeviceToHost);
  for (int i = 0; i < 9; i++) printf(i ? " %d" : "%d", r[i]);
  printf("\n");
  return 0;
}
