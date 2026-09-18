// The whole program on AMD: memory, a launch through hipLaunchKernelGGL
// and through <<<>>>, the answer back. Compiled for gfx942 here and not
// run: there is no AMD GPU on the machine that wrote this.
// expect: c[0] = 0, c[1] = 3, c[1023] = 3069
#include <hip/hip_runtime.h>
#include <stdio.h>

__global__ void vecadd(const int *a, const int *b, int *c, int n) {
  int i = blockIdx.x * blockDim.x + threadIdx.x;
  if (i < n) c[i] = a[i] + b[i];
}

int main() {
  const int n = 1024;
  static int a[n], b[n], c[n];
  for (int i = 0; i < n; i++) {
    a[i] = i;
    b[i] = 2 * i;
  }
  int *da, *db, *dc;
  hipMalloc((void **)&da, n * sizeof(int));
  hipMalloc((void **)&db, n * sizeof(int));
  hipMalloc((void **)&dc, n * sizeof(int));
  hipMemcpy(da, a, n * sizeof(int), hipMemcpyHostToDevice);
  hipMemcpy(db, b, n * sizeof(int), hipMemcpyHostToDevice);
  hipLaunchKernelGGL(vecadd, dim3((n + 255) / 256), dim3(256), 0, 0, da, db, dc, n);
  vecadd<<<(n + 255) / 256, 256>>>(da, db, dc, n);
  if (hipDeviceSynchronize() != hipSuccess) return 1;
  hipMemcpy(c, dc, n * sizeof(int), hipMemcpyDeviceToHost);
  printf("c[0] = %d, c[1] = %d, c[1023] = %d\n", c[0], c[1], c[1023]);
  return 0;
}
