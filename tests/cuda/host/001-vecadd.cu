// The whole program: memory on the device, a launch, the answer back.
// expect: c[0] = 0, c[1] = 3, c[1023] = 3069, sum = 1571328
#include <cuda_runtime.h>
#include <stdio.h>

__global__ void vecadd(const int *a, const int *b, int *c, int n) {
  int i = blockIdx.x * blockDim.x + threadIdx.x;
  if (i < n) c[i] = a[i] + b[i];
}

int main() {
  const int n = 1024;
  int a[n], b[n], c[n];
  for (int i = 0; i < n; i++) {
    a[i] = i;
    b[i] = 2 * i;
    c[i] = -1;
  }
  int *da, *db, *dc;
  if (cudaMalloc((void **)&da, n * sizeof(int)) != cudaSuccess) return 1;
  if (cudaMalloc((void **)&db, n * sizeof(int)) != cudaSuccess) return 1;
  if (cudaMalloc((void **)&dc, n * sizeof(int)) != cudaSuccess) return 1;
  cudaMemcpy(da, a, n * sizeof(int), cudaMemcpyHostToDevice);
  cudaMemcpy(db, b, n * sizeof(int), cudaMemcpyHostToDevice);
  vecadd<<<(n + 255) / 256, 256>>>(da, db, dc, n);
  cudaError_t e = cudaDeviceSynchronize();
  if (e != cudaSuccess) {
    printf("launch failed: %s\n", cudaGetErrorString(e));
    return 1;
  }
  cudaMemcpy(c, dc, n * sizeof(int), cudaMemcpyDeviceToHost);
  long sum = 0;
  for (int i = 0; i < n; i++) sum += c[i];
  printf("c[0] = %d, c[1] = %d, c[1023] = %d, sum = %ld\n", c[0], c[1], c[1023], sum);
  cudaFree(da);
  cudaFree(db);
  cudaFree(dc);
  return 0;
}
