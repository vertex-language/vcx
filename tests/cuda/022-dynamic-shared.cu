// extern __shared__ memory sized at launch.
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

__global__ void rotate(int* v, int shift) {
    extern __shared__ int buf[];
    int t = threadIdx.x, n = blockDim.x;
    buf[t] = v[t];
    __syncthreads();
    v[t] = buf[(t + shift) % n];
}

int main() {
    const int n = 100;
    int h[n];
    for (int i = 0; i < n; ++i) h[i] = i * 10;
    int* d;
    CHECK(cudaMalloc(&d, sizeof h));
    CHECK(cudaMemcpy(d, h, sizeof h, cudaMemcpyHostToDevice));
    rotate<<<1, n, n * sizeof(int)>>>(d, 37);
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    std::printf("%d %d %d\n", h[0], h[62], h[63]);
    CHECK(cudaFree(d));
    return 0;
}
