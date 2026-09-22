// warpSize on the device agrees with the device's properties.
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

__global__ void width(int* out) { out[0] = warpSize; }

int main() {
    int* d;
    CHECK(hipMalloc(&d, sizeof(int)));
    width<<<1, 1>>>(d);
    int h = 0;
    CHECK(hipMemcpy(&h, d, sizeof h, hipMemcpyDeviceToHost));
    hipDeviceProp_t p;
    CHECK(hipGetDeviceProperties(&p, 0));
    std::printf("%d %d\n", h == p.warpSize, h == 32 || h == 64);
    return 0;
}
