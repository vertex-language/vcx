// atomicMax, atomicMin, atomicCAS, atomicExch, atomicOr, atomicAnd.
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
    CHECK(cudaMalloc(&d, sizeof h));
    CHECK(cudaMemcpy(d, h, sizeof h, cudaMemcpyHostToDevice));
    ops<<<4, 256>>>(d);
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    std::printf("%d %d %d %d %d %d %d\n", h[0], h[1], h[2], h[3], h[4] != 0, h[5], h[6]);
    CHECK(cudaFree(d));
    return 0;
}
