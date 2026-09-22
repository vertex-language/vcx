// A kernel writes one value, and the host copies it back.
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

__global__ void write(int* out) { *out = 42; }

int main() {
    int* d;
    CHECK(cudaMalloc(&d, sizeof(int)));
    write<<<1, 1>>>(d);
    int h = 0;
    CHECK(cudaMemcpy(&h, d, sizeof h, cudaMemcpyDeviceToHost));
    CHECK(cudaFree(d));
    std::printf("%d\n", h);
    return 0;
}
