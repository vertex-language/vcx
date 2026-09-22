// Pinned host memory from hipHostMalloc.
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

__global__ void negate(int* v) { v[threadIdx.x] = -v[threadIdx.x]; }

int main() {
    const int n = 64;
    int* h;
    CHECK(hipHostMalloc(&h, n * sizeof(int), hipHostMallocDefault));
    for (int i = 0; i < n; ++i) h[i] = i + 1;
    int* d;
    CHECK(hipMalloc(&d, n * sizeof(int)));
    CHECK(hipMemcpy(d, h, n * sizeof(int), hipMemcpyHostToDevice));
    negate<<<1, n>>>(d);
    CHECK(hipMemcpy(h, d, n * sizeof(int), hipMemcpyDeviceToHost));
    std::printf("%d %d\n", h[0], h[63]);
    CHECK(hipHostFree(h));
    CHECK(hipFree(d));
    return 0;
}
