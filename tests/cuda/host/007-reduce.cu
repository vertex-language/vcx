// A block reduction the way the samples write it: warp shuffles, one
// word of shared memory per warp, a grid-stride loop, and atomicAdd of
// each block's partial sum into the result.
// expect: sum = 524800 max = 1024
#include <cuda_runtime.h>
#include <stdio.h>

__device__ int warpSum(int v) {
  for (int off = warpSize / 2; off > 0; off >>= 1) v += __shfl_down_sync(0xffffffffu, v, off);
  return v;
}

__global__ void reduce(const int *in, int n, int *sum, int *mx) {
  __shared__ int partial[32];
  int lane = threadIdx.x % warpSize, warp = threadIdx.x / warpSize;
  int local = 0, big = 0;
  for (int i = blockIdx.x * blockDim.x + threadIdx.x; i < n; i += gridDim.x * blockDim.x) {
    local += in[i];
    big = in[i] > big ? in[i] : big;
  }
  local = warpSum(local);
  if (lane == 0) partial[warp] = local;
  __syncthreads();
  if (warp == 0) {
    int v = threadIdx.x < blockDim.x / warpSize ? partial[lane] : 0;
    v = warpSum(v);
    if (lane == 0) atomicAdd(sum, v);
  }
  atomicMax(mx, big);
}

int main() {
  const int n = 1024;
  static int h[n];
  for (int i = 0; i < n; i++) h[i] = i + 1;
  int *d, *ds, *dm, s = 0, m = 0;
  cudaMalloc((void **)&d, sizeof h);
  cudaMalloc((void **)&ds, sizeof(int));
  cudaMalloc((void **)&dm, sizeof(int));
  cudaMemcpy(d, h, sizeof h, cudaMemcpyHostToDevice);
  cudaMemset(ds, 0, sizeof(int));
  cudaMemset(dm, 0, sizeof(int));
  reduce<<<4, 128>>>(d, n, ds, dm);
  if (cudaDeviceSynchronize() != cudaSuccess) {
    printf("launch failed: %s\n", cudaGetErrorString(cudaGetLastError()));
    return 1;
  }
  cudaMemcpy(&s, ds, sizeof s, cudaMemcpyDeviceToHost);
  cudaMemcpy(&m, dm, sizeof m, cudaMemcpyDeviceToHost);
  printf("sum = %d max = %d\n", s, m);
  return 0;
}
