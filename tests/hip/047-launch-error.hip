// A launch with too many threads per block fails, and the error is reported.
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

__global__ void k(int* v) { v[threadIdx.x] = 1; }

int main() {
    int* d;
    CHECK(hipMalloc(&d, 4096 * sizeof(int)));
    k<<<1, 4096>>>(d);
    hipError_t e = hipGetLastError();
    std::printf("launch failed: %d\n", e != hipSuccess);
    std::printf("cleared: %d\n", hipGetLastError() == hipSuccess);
    k<<<1, 32>>>(d);
    CHECK(hipDeviceSynchronize());
    std::printf("recovered\n");
    return 0;
}
