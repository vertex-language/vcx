// __constant__ and __device__ objects, written and read by name from the
// host, a 2-D launch with dim3, and a kernel reached through a template.
// expect: table = 0 3 6 9 12 15 18 21
// expect: counter = 64
// expect: maxid = 63
#include <cuda_runtime.h>
#include <stdio.h>

__constant__ int scale;
__device__ int counter;
__device__ int maxid;

template <class T> __global__ void fill(T *out) {
  int x = blockIdx.x * blockDim.x + threadIdx.x;
  int y = blockIdx.y * blockDim.y + threadIdx.y;
  int i = y * (gridDim.x * blockDim.x) + x;
  if (i < 8) out[i] = (T)(i * scale);
  atomicAdd(&counter, 1);
  atomicMax(&maxid, i);
}

int main() {
  int three = 3;
  cudaMemcpyToSymbol(&scale, &three, sizeof three, 0, cudaMemcpyHostToDevice);
  int *d;
  cudaMalloc((void **)&d, 8 * sizeof(int));
  dim3 grid(2, 2), block(4, 4);
  fill<int><<<grid, block>>>(d);
  if (cudaDeviceSynchronize() != cudaSuccess) {
    printf("launch failed: %s\n", cudaGetErrorString(cudaGetLastError()));
    return 1;
  }
  int table[8];
  cudaMemcpy(table, d, sizeof table, cudaMemcpyDeviceToHost);
  printf("table =");
  for (int i = 0; i < 8; i++) printf(" %d", table[i]);
  printf("\n");
  int n = 0, m = 0;
  cudaMemcpyFromSymbol(&n, &counter, sizeof n, 0, cudaMemcpyDeviceToHost);
  cudaMemcpyFromSymbol(&m, &maxid, sizeof m, 0, cudaMemcpyDeviceToHost);
  printf("counter = %d\nmaxid = %d\n", n, m);
  cudaFree(d);
  return 0;
}
