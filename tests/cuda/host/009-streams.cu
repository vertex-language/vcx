// Streams and events: two kernels on two streams with async copies
// through pinned memory, an event recorded and waited on, and the
// elapsed time between two events being a non-negative number.
// expect: a = 10 20 30 40 b = 1 4 9 16 elapsed ok = 1
#include <cuda_runtime.h>
#include <stdio.h>

__global__ void tens(int *p) { p[threadIdx.x] = (threadIdx.x + 1) * 10; }
__global__ void squares(int *p) { p[threadIdx.x] = (threadIdx.x + 1) * (threadIdx.x + 1); }

int main() {
  int *ha, *hb;
  cudaMallocHost((void **)&ha, 4 * sizeof(int));
  cudaMallocHost((void **)&hb, 4 * sizeof(int));
  int *da, *db;
  cudaMalloc((void **)&da, 4 * sizeof(int));
  cudaMalloc((void **)&db, 4 * sizeof(int));
  cudaStream_t s1, s2;
  cudaStreamCreate(&s1);
  cudaStreamCreate(&s2);
  cudaEvent_t start, stop;
  cudaEventCreate(&start);
  cudaEventCreate(&stop);
  cudaEventRecord(start, s1);
  tens<<<1, 4, 0, s1>>>(da);
  squares<<<1, 4, 0, s2>>>(db);
  cudaMemcpyAsync(ha, da, 4 * sizeof(int), cudaMemcpyDeviceToHost, s1);
  cudaMemcpyAsync(hb, db, 4 * sizeof(int), cudaMemcpyDeviceToHost, s2);
  cudaEventRecord(stop, s1);
  cudaEventSynchronize(stop);
  cudaStreamSynchronize(s2);
  float ms = -1.0f;
  cudaEventElapsedTime(&ms, start, stop);
  printf("a = %d %d %d %d b = %d %d %d %d elapsed ok = %d\n", ha[0], ha[1], ha[2], ha[3], hb[0], hb[1], hb[2], hb[3], ms >= 0.0f ? 1 : 0);
  cudaStreamDestroy(s1);
  cudaStreamDestroy(s2);
  cudaEventDestroy(start);
  cudaEventDestroy(stop);
  cudaFreeHost(ha);
  cudaFreeHost(hb);
  cudaFree(da);
  cudaFree(db);
  return 0;
}
