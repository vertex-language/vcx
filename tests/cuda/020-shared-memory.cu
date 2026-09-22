// A __shared__ array and __syncthreads: reversing a block.
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

__global__ void reverse(int* v) {
    __shared__ int tile[256];
    int t = threadIdx.x;
    tile[t] = v[blockIdx.x * blockDim.x + t];
    __syncthreads();
    v[blockIdx.x * blockDim.x + t] = tile[blockDim.x - 1 - t];
}

int main() {
    const int n = 512;
    int h[n];
    for (int i = 0; i < n; ++i) h[i] = i;
    int* d;
    CHECK(cudaMalloc(&d, sizeof h));
    CHECK(cudaMemcpy(d, h, sizeof h, cudaMemcpyHostToDevice));
    reverse<<<2, 256>>>(d);
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    std::printf("%d %d %d %d\n", h[0], h[255], h[256], h[511]);
    CHECK(cudaFree(d));
    return 0;
}
