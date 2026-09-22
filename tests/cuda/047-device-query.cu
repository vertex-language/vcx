// Querying the device: a count and properties that must hold on any GPU.
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

int main() {
    int count = 0;
    CHECK(cudaGetDeviceCount(&count));
    int dev = -1;
    CHECK(cudaGetDevice(&dev));
    cudaDeviceProp p;
    CHECK(cudaGetDeviceProperties(&p, dev));
    std::printf("%d %d %d %d %d\n", count > 0, dev, p.warpSize, p.maxThreadsPerBlock >= 1024,
                p.major >= 3);
    return 0;
}
