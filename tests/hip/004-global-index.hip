// blockIdx, blockDim and threadIdx make a global index.
#include <cstdio>
#include <hip/hip_runtime.h>

#define CHECK(x)                                                       \
    do {                                                               \
        hipError_t e_ = (x);                                           \
        if (e_ != hipSuccess) {                                        \
            std::printf("HIP error %d at line %d\n", (int)e_, __LINE__); \
            return 1;                                                  \
        }                                                              \
    } while (0)

__global__ void index(int* out) {
    int i = blockIdx.x * blockDim.x + threadIdx.x;
    out[i] = blockIdx.x * 1000 + threadIdx.x;
}

int main() {
    const int blocks = 4, threads = 64, n = blocks * threads;
    int* d;
    CHECK(hipMalloc(&d, n * sizeof(int)));
    index<<<blocks, threads>>>(d);
    int h[n];
    CHECK(hipMemcpy(h, d, sizeof h, hipMemcpyDeviceToHost));
    for (int i = 0; i < n; i += 37) std::printf("%d ", h[i]);
    std::printf("\n");
    CHECK(hipFree(d));
    return 0;
}
