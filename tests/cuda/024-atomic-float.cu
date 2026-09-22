// atomicAdd on floats, with values whose sum is exact in any order.
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

__global__ void accumulate(float* sum) { atomicAdd(sum, (float)(threadIdx.x % 4) * 0.25f); }

int main() {
    float* d;
    CHECK(cudaMalloc(&d, sizeof(float)));
    CHECK(cudaMemset(d, 0, sizeof(float)));
    accumulate<<<16, 128>>>(d);
    float h;
    CHECK(cudaMemcpy(&h, d, sizeof h, cudaMemcpyDeviceToHost));
    std::printf("%g\n", h);
    CHECK(cudaFree(d));
    return 0;
}
