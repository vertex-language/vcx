// A __device__ function called from a kernel.
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

__device__ int collatz_steps(unsigned n) {
    int steps = 0;
    while (n != 1) {
        n = (n & 1) ? 3 * n + 1 : n / 2;
        ++steps;
    }
    return steps;
}

__global__ void steps(int* out) { out[threadIdx.x] = collatz_steps(threadIdx.x + 1); }

int main() {
    int* d;
    CHECK(cudaMalloc(&d, 64 * sizeof(int)));
    steps<<<1, 64>>>(d);
    int h[64];
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    int best = 0;
    for (int i = 0; i < 64; ++i)
        if (h[i] > h[best]) best = i;
    std::printf("%d %d %d\n", h[26], best + 1, h[best]);
    CHECK(cudaFree(d));
    return 0;
}
