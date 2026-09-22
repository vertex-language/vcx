// __shfl broadcasts a lane; __shfl_xor swaps partners; __shfl_up shifts.
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

__global__ void shuffle(int* out) {
    int lane = threadIdx.x;
    int v = lane * lane;
    out[lane] = __shfl(v, 5);
    out[64 + lane] = __shfl_xor(v, 1);
    out[128 + lane] = __shfl_up(v, 3);
}

int main() {
    int* d;
    CHECK(hipMalloc(&d, 192 * sizeof(int)));
    shuffle<<<1, 32>>>(d);
    int h[192];
    CHECK(hipMemcpy(h, d, sizeof h, hipMemcpyDeviceToHost));
    std::printf("%d %d %d %d %d %d\n", h[0], h[31], h[64], h[65], h[128 + 2], h[128 + 10]);
    CHECK(hipFree(d));
    return 0;
}
