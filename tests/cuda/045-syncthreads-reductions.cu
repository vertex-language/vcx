// __syncthreads_count, __syncthreads_and and __syncthreads_or.
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

__global__ void votes(int* out) {
    int t = threadIdx.x;
    int count = __syncthreads_count(t % 5 == 0);
    int all = __syncthreads_and(t < 1000);
    int any = __syncthreads_or(t == 77);
    int none = __syncthreads_or(t > 5000);
    if (t == 0) {
        out[0] = count;
        out[1] = all;
        out[2] = any;
        out[3] = none;
    }
}

int main() {
    int* d;
    CHECK(cudaMalloc(&d, 4 * sizeof(int)));
    votes<<<1, 256>>>(d);
    int h[4];
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    std::printf("%d %d %d %d\n", h[0], h[1] != 0, h[2] != 0, h[3] != 0);
    return 0;
}
