// A __device__ global variable, read back with cudaMemcpyFromSymbol.
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

__device__ int counter;
__device__ int table[4] = {10, 20, 30, 40};

__global__ void count() {
    atomicAdd(&counter, table[threadIdx.x % 4]);
}

int main() {
    int zero = 0;
    CHECK(cudaMemcpyToSymbol(counter, &zero, sizeof zero));
    count<<<2, 64>>>();
    int h = 0;
    CHECK(cudaMemcpyFromSymbol(&h, counter, sizeof h));
    std::printf("%d\n", h);
    return 0;
}
