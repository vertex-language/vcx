// A 3D block: threadIdx.y and .z, blockDim in three axes.
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

__global__ void linear(int* out) {
    int t = threadIdx.x + blockDim.x * (threadIdx.y + blockDim.y * threadIdx.z);
    out[t] = threadIdx.x * 100 + threadIdx.y * 10 + threadIdx.z;
}

int main() {
    dim3 block(4, 3, 2);
    const int n = 24;
    int* d;
    CHECK(cudaMalloc(&d, n * sizeof(int)));
    linear<<<1, block>>>(d);
    int h[n];
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    for (int v : h) std::printf("%d ", v);
    std::printf("\n");
    CHECK(cudaFree(d));
    return 0;
}
