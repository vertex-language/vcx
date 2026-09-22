// atomicAdd on floats, with values whose sum is exact in any order.
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

__global__ void accumulate(float* sum) { atomicAdd(sum, (float)(threadIdx.x % 4) * 0.25f); }

int main() {
    float* d;
    CHECK(hipMalloc(&d, sizeof(float)));
    CHECK(hipMemset(d, 0, sizeof(float)));
    accumulate<<<16, 128>>>(d);
    float h;
    CHECK(hipMemcpy(&h, d, sizeof h, hipMemcpyDeviceToHost));
    std::printf("%g\n", h);
    CHECK(hipFree(d));
    return 0;
}
