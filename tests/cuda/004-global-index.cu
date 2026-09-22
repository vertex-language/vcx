// blockIdx, blockDim and threadIdx make a global index.
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

__global__ void index(int* out) {
    int i = blockIdx.x * blockDim.x + threadIdx.x;
    out[i] = blockIdx.x * 1000 + threadIdx.x;
}

int main() {
    const int blocks = 4, threads = 8, n = blocks * threads;
    int* d;
    CHECK(cudaMalloc(&d, n * sizeof(int)));
    index<<<blocks, threads>>>(d);
    int h[n];
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    for (int i = 0; i < n; i += 5) std::printf("%d ", h[i]);
    std::printf("\n");
    CHECK(cudaFree(d));
    return 0;
}
