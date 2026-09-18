// C++ in device code: a class template with a virtual-free hierarchy of
// functors, references, a lambda in a kernel, a constexpr function (host
// and device, which nvcc wants said), and
// a device function taking a struct by reference.
// expect: 20 42 6 100 7
#include <cuda_runtime.h>
#include <stdio.h>

template <class T> struct Accum {
  T total;
  __host__ __device__ Accum() : total(0) {}
  __host__ __device__ void add(const T &v) { total += v; }
  __host__ __device__ T get() const { return total; }
};

struct Scale {
  int k;
  __device__ int operator()(int v) const { return v * k; }
};

__host__ __device__ constexpr int square(int x) { return x * x; }

__device__ void bump(Accum<int> &a, int v) { a.add(v); }

__global__ void work(int *out) {
  Accum<int> acc;
  for (int i = 1; i <= 4; i++) bump(acc, i * 2);
  out[0] = acc.get();
  Scale s{6};
  out[1] = s(7);
  auto twice = [](int v) { return v * 2; };
  out[2] = twice(3);
  out[3] = square(10);
  const int &r = out[1];
  out[4] = r / 6;
}

int main() {
  int *d, h[5];
  cudaMalloc((void **)&d, sizeof h);
  work<<<1, 1>>>(d);
  if (cudaDeviceSynchronize() != cudaSuccess) {
    printf("launch failed: %s\n", cudaGetErrorString(cudaGetLastError()));
    return 1;
  }
  cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost);
  printf("%d %d %d %d %d\n", h[0], h[1], h[2], h[3], h[4]);
  return 0;
}
