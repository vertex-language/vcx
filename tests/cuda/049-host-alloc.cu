// Pinned host memory from cudaMallocHost.
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

__global__ void negate(int* v) { v[threadIdx.x] = -v[threadIdx.x]; }

int main() {
    const int n = 64;
    int* h;
    CHECK(cudaMallocHost(&h, n * sizeof(int)));
    for (int i = 0; i < n; ++i) h[i] = i + 1;
    int* d;
    CHECK(cudaMalloc(&d, n * sizeof(int)));
    CHECK(cudaMemcpy(d, h, n * sizeof(int), cudaMemcpyHostToDevice));
    negate<<<1, n>>>(d);
    CHECK(cudaMemcpy(h, d, n * sizeof(int), cudaMemcpyDeviceToHost));
    std::printf("%d %d\n", h[0], h[63]);
    CHECK(cudaFreeHost(h));
    CHECK(cudaFree(d));
    return 0;
}
