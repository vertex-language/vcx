// atomicMax, atomicMin, atomicCAS, atomicExch, atomicOr, atomicAnd.
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

__global__ void ops(int* v) {
    int t = blockIdx.x * blockDim.x + threadIdx.x;
    atomicMax(&v[0], (t * 37) % 1001);
    atomicMin(&v[1], 500 - (t * 13) % 997);
    atomicOr(&v[2], 1 << (t % 31));
    atomicAnd(&v[3], ~(1 << (t % 16)));
    // One thread wins the compare-and-swap from zero.
    if (atomicCAS(&v[4], 0, t + 1) == 0) atomicAdd(&v[5], 1);
    atomicExch(&v[6], 7);
}

int main() {
    int h[7] = {-1, 1 << 30, 0, -1, 0, 0, 0};
    int* d;
    CHECK(hipMalloc(&d, sizeof h));
    CHECK(hipMemcpy(d, h, sizeof h, hipMemcpyHostToDevice));
    ops<<<4, 256>>>(d);
    CHECK(hipMemcpy(h, d, sizeof h, hipMemcpyDeviceToHost));
    std::printf("%d %d %d %d %d %d %d\n", h[0], h[1], h[2], h[3], h[4] != 0, h[5], h[6]);
    CHECK(hipFree(d));
    return 0;
}
