// A launch with too many threads per block fails, and the error is reported.
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

__global__ void k(int* v) { v[threadIdx.x] = 1; }

int main() {
    int* d;
    CHECK(cudaMalloc(&d, 4096 * sizeof(int)));
    k<<<1, 4096>>>(d);
    cudaError_t e = cudaGetLastError();
    std::printf("launch failed: %d\n", e != cudaSuccess);
    std::printf("cleared: %d\n", cudaGetLastError() == cudaSuccess);
    k<<<1, 32>>>(d);
    CHECK(cudaDeviceSynchronize());
    std::printf("recovered\n");
    return 0;
}
