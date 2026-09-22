// __ballot, __any and __all; the ballot is 64 bits wide.
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

__global__ void votes(unsigned long long* out) {
    int lane = threadIdx.x;
    unsigned long long b = __ballot(lane % 3 == 0);
    int any = __any(lane == 17);
    int all = __all(lane < 32);
    int none = __any(lane > 40);
    if (lane == 0) {
        out[0] = b;
        out[1] = any;
        out[2] = all;
        out[3] = none;
    }
}

int main() {
    unsigned long long* d;
    CHECK(hipMalloc(&d, 4 * sizeof(unsigned long long)));
    votes<<<1, 32>>>(d);
    unsigned long long h[4];
    CHECK(hipMemcpy(h, d, sizeof h, hipMemcpyDeviceToHost));
    std::printf("%llx %llu %llu %llu %zu\n", h[0], h[1], h[2], h[3], sizeof(__ballot(1)));
    CHECK(hipFree(d));
    return 0;
}
