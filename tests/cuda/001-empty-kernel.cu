// An empty kernel, launched and waited for.
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

__global__ void nothing() {}

int main() {
    nothing<<<1, 1>>>();
    CHECK(cudaGetLastError());
    CHECK(cudaDeviceSynchronize());
    std::printf("done\n");
    return 0;
}
