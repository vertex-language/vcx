// hipLaunchKernelGGL, HIP's launch spelling.
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

__global__ void fill(int* out, int k) { out[threadIdx.x] = threadIdx.x * k; }

int main() {
    const int n = 16;
    int* d;
    CHECK(hipMalloc(&d, n * sizeof(int)));
    hipLaunchKernelGGL(fill, dim3(1), dim3(n), 0, 0, d, 5);
    int h[n];
    CHECK(hipMemcpy(h, d, sizeof h, hipMemcpyDeviceToHost));
    for (int v : h) std::printf("%d ", v);
    std::printf("\n");
    CHECK(hipFree(d));
    return 0;
}
