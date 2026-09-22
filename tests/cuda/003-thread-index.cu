// Each thread writes its threadIdx.x.
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

__global__ void ids(int* out) { out[threadIdx.x] = threadIdx.x * 3; }

int main() {
    const int n = 32;
    int* d;
    CHECK(cudaMalloc(&d, n * sizeof(int)));
    ids<<<1, n>>>(d);
    int h[n];
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    for (int v : h) std::printf("%d ", v);
    std::printf("\n");
    CHECK(cudaFree(d));
    return 0;
}
