// __shfl_sync broadcasts a lane; __shfl_xor_sync swaps partners.
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

__global__ void shuffle(int* out) {
    int lane = threadIdx.x;
    int v = lane * lane;
    out[lane] = __shfl_sync(0xffffffffu, v, 5);
    out[32 + lane] = __shfl_xor_sync(0xffffffffu, v, 1);
    out[64 + lane] = __shfl_up_sync(0xffffffffu, v, 3);
}

int main() {
    int* d;
    CHECK(cudaMalloc(&d, 96 * sizeof(int)));
    shuffle<<<1, 32>>>(d);
    int h[96];
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    std::printf("%d %d %d %d %d %d\n", h[0], h[31], h[32], h[33], h[64 + 2], h[64 + 10]);
    CHECK(cudaFree(d));
    return 0;
}
