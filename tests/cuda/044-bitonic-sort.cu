// A bitonic sort of one block in shared memory.
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

__global__ void bitonic(int* v) {
    __shared__ int s[1024];
    unsigned t = threadIdx.x;
    s[t] = v[t];
    __syncthreads();
    for (unsigned k = 2; k <= blockDim.x; k <<= 1) {
        for (unsigned j = k >> 1; j > 0; j >>= 1) {
            unsigned p = t ^ j;
            if (p > t) {
                bool up = (t & k) == 0;
                if ((s[t] > s[p]) == up) {
                    int x = s[t];
                    s[t] = s[p];
                    s[p] = x;
                }
            }
            __syncthreads();
        }
    }
    v[t] = s[t];
}

int main() {
    const int n = 1024;
    int h[n];
    unsigned x = 12345;
    for (int i = 0; i < n; ++i) {
        x = x * 1664525u + 1013904223u;
        h[i] = (int)(x >> 16) - 32768;
    }
    int* d;
    CHECK(cudaMalloc(&d, sizeof h));
    CHECK(cudaMemcpy(d, h, sizeof h, cudaMemcpyHostToDevice));
    bitonic<<<1, n>>>(d);
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    int sorted = 1;
    for (int i = 1; i < n; ++i) sorted &= h[i - 1] <= h[i];
    std::printf("%d %d %d %d\n", sorted, h[0], h[512], h[1023]);
    return 0;
}
