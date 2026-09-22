// A __host__ __device__ function gives the same answer on both sides.
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

__host__ __device__ unsigned mix(unsigned v) {
    v ^= v >> 16;
    v *= 0x7feb352du;
    v ^= v >> 15;
    v *= 0x846ca68bu;
    v ^= v >> 16;
    return v;
}

__global__ void hash(unsigned* out) { out[threadIdx.x] = mix(threadIdx.x); }

int main() {
    unsigned* d;
    CHECK(hipMalloc(&d, 16 * sizeof(unsigned)));
    hash<<<1, 16>>>(d);
    unsigned h[16];
    CHECK(hipMemcpy(h, d, sizeof h, hipMemcpyDeviceToHost));
    int same = 0;
    for (unsigned i = 0; i < 16; ++i) same += h[i] == mix(i);
    std::printf("%u %d\n", h[5], same);
    CHECK(hipFree(d));
    return 0;
}
