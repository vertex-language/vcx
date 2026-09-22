// __ballot_sync, __any_sync and __all_sync.
#include <cstdio>
#include <cuda_runtime.h>

#define CHECK(x)                                                        \
    do {                                                                \
        cudaError_t e_ = (x);                                           \
        if (e_ != cudaSuccess) {                                        \
            std::printf("CUDA error %d at line %d\n", (int)e_, __LINE__); \
            return 1;                                                   \
        }                                                               \
    } while (0)

__global__ void votes(unsigned* out) {
    int lane = threadIdx.x;
    unsigned b = __ballot_sync(0xffffffffu, lane % 3 == 0);
    int any = __any_sync(0xffffffffu, lane == 17);
    int all = __all_sync(0xffffffffu, lane < 32);
    int none = __any_sync(0xffffffffu, lane > 40);
    if (lane == 0) {
        out[0] = b;
        out[1] = any;
        out[2] = all;
        out[3] = none;
    }
}

int main() {
    unsigned* d;
    CHECK(cudaMalloc(&d, 4 * sizeof(unsigned)));
    votes<<<1, 32>>>(d);
    unsigned h[4];
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    std::printf("%08x %u %u %u\n", h[0], h[1], h[2], h[3]);
    CHECK(cudaFree(d));
    return 0;
}
