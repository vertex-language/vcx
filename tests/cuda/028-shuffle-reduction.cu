// A warp sum with __shfl_down_sync.
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

__global__ void warp_sum(const int* in, int* out) {
    int v = in[blockIdx.x * blockDim.x + threadIdx.x];
    for (int offset = 16; offset > 0; offset >>= 1) v += __shfl_down_sync(0xffffffffu, v, offset);
    if ((threadIdx.x & 31) == 0) out[(blockIdx.x * blockDim.x + threadIdx.x) / 32] = v;
}

int main() {
    const int n = 256;
    int h[n];
    for (int i = 0; i < n; ++i) h[i] = i % 13;
    int *din, *dout;
    CHECK(cudaMalloc(&din, sizeof h));
    CHECK(cudaMalloc(&dout, 8 * sizeof(int)));
    CHECK(cudaMemcpy(din, h, sizeof h, cudaMemcpyHostToDevice));
    warp_sum<<<2, 128>>>(din, dout);
    int out[8];
    CHECK(cudaMemcpy(out, dout, sizeof out, cudaMemcpyDeviceToHost));
    for (int v : out) std::printf("%d ", v);
    std::printf("\n");
    return 0;
}
