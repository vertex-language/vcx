// hipMemset, and device-to-device hipMemcpy.
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

__global__ void bump(unsigned char* v) { v[threadIdx.x] += threadIdx.x; }

int main() {
    const int n = 64;
    unsigned char *a, *b;
    CHECK(hipMalloc(&a, n));
    CHECK(hipMalloc(&b, n));
    CHECK(hipMemset(a, 0x10, n));
    bump<<<1, n>>>(a);
    CHECK(hipMemcpy(b, a, n, hipMemcpyDeviceToDevice));
    CHECK(hipMemset(a, 0, n));
    unsigned char h[n], z[n];
    CHECK(hipMemcpy(h, b, n, hipMemcpyDeviceToHost));
    CHECK(hipMemcpy(z, a, n, hipMemcpyDeviceToHost));
    std::printf("%d %d %d\n", h[0], h[63], z[10]);
    return 0;
}
