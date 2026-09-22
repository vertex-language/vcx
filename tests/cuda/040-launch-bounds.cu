// __launch_bounds__ on a kernel.
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

__global__ void __launch_bounds__(128, 2) triple(int* v) {
    int i = blockIdx.x * blockDim.x + threadIdx.x;
    v[i] = i * 3;
}

int main() {
    const int n = 512;
    int* d;
    CHECK(cudaMalloc(&d, n * sizeof(int)));
    triple<<<n / 128, 128>>>(d);
    int h[n];
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    std::printf("%d %d\n", h[1], h[511]);
    return 0;
}
