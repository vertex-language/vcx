// A grid-stride loop covers more elements than threads.
#include <cstdio>
#include <cuda_runtime.h>

#define CHECK(x)                                                        \
    do {                                                                \
        cudaError_t e_ = (x);                                           \
        if (e_ != cudaSuccess) {                                        \
            std::printf("CUDA error %d at line %d\n", (int)e_, __LINE__); \
            return 1;                                                   \
        }                                                               \
    } while (0)

__global__ void fill(long long* v, int n) {
    for (int i = blockIdx.x * blockDim.x + threadIdx.x; i < n; i += gridDim.x * blockDim.x)
        v[i] = (long long)i * 3 + 1;
}

int main() {
    const int n = 100000;
    long long* d;
    CHECK(cudaMalloc(&d, n * sizeof(long long)));
    fill<<<4, 64>>>(d, n);
    static long long h[n];
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    long long sum = 0;
    for (long long v : h) sum += v;
    std::printf("%lld %lld\n", h[n - 1], sum);
    CHECK(cudaFree(d));
    return 0;
}
