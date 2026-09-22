// printf from a kernel, by one thread so the order is fixed.
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

__global__ void hello(int n) {
    if (threadIdx.x == 0 && blockIdx.x == 0) {
        printf("hello from the device: %d %u %.2f %s\n", n, 4000000000u, 3.14159, "text");
        for (int i = 0; i < 3; ++i) printf("line %d\n", i);
    }
}

int main() {
    hello<<<2, 32>>>(7);
    CHECK(cudaDeviceSynchronize());
    std::printf("back on the host\n");
    return 0;
}
