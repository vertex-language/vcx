// A tiled matrix multiply: 2-D blocks, shared tiles, a template on the
// tile width, and a struct with __host__ __device__ operators used on
// both sides.
// expect: C[0][0] = 568 C[17][5] = 580 C[63][63] = 565 checksum = 2358330
#include <cuda_runtime.h>
#include <stdio.h>

struct Vec2 {
  float x, y;
  __host__ __device__ Vec2(float a, float b) : x(a), y(b) {}
  __host__ __device__ Vec2 operator+(const Vec2 &o) const { return Vec2(x + o.x, y + o.y); }
  __host__ __device__ float dot(const Vec2 &o) const { return x * o.x + y * o.y; }
};

template <int TILE>
__global__ void matmul(const float *A, const float *B, float *C, int n) {
  __shared__ float As[TILE][TILE];
  __shared__ float Bs[TILE][TILE];
  int row = blockIdx.y * TILE + threadIdx.y;
  int col = blockIdx.x * TILE + threadIdx.x;
  float acc = 0.0f;
  for (int t = 0; t < n / TILE; t++) {
    As[threadIdx.y][threadIdx.x] = A[row * n + t * TILE + threadIdx.x];
    Bs[threadIdx.y][threadIdx.x] = B[(t * TILE + threadIdx.y) * n + col];
    __syncthreads();
    for (int k = 0; k < TILE; k++) {
      Vec2 a(As[threadIdx.y][k], 0.0f), b(Bs[k][threadIdx.x], 0.0f);
      acc += a.dot(b);
    }
    __syncthreads();
  }
  C[row * n + col] = acc;
}

int main() {
  const int n = 64;
  static float A[n * n], B[n * n], C[n * n];
  for (int i = 0; i < n * n; i++) {
    A[i] = (float)(i % 7);
    B[i] = (float)(i % 5 + 1);
  }
  float *dA, *dB, *dC;
  cudaMalloc((void **)&dA, sizeof A);
  cudaMalloc((void **)&dB, sizeof B);
  cudaMalloc((void **)&dC, sizeof C);
  cudaMemcpy(dA, A, sizeof A, cudaMemcpyHostToDevice);
  cudaMemcpy(dB, B, sizeof B, cudaMemcpyHostToDevice);
  dim3 block(16, 16), grid(n / 16, n / 16);
  matmul<16><<<grid, block>>>(dA, dB, dC, n);
  if (cudaDeviceSynchronize() != cudaSuccess) {
    printf("launch failed: %s\n", cudaGetErrorString(cudaGetLastError()));
    return 1;
  }
  cudaMemcpy(C, dC, sizeof C, cudaMemcpyDeviceToHost);
  Vec2 sum(0.0f, 0.0f);
  for (int i = 0; i < n * n; i++) sum = sum + Vec2(C[i], 0.0f);
  printf("C[0][0] = %.0f C[17][5] = %.0f C[63][63] = %.0f checksum = %.0f\n", C[0], C[17 * n + 5], C[63 * n + 63], sum.x);
  return 0;
}
