// cudaMemset, and device-to-device cudaMemcpy.
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

__global__ void bump(unsigned char* v) { v[threadIdx.x] += threadIdx.x; }

int main() {
    const int n = 64;
    unsigned char *a, *b;
    CHECK(cudaMalloc(&a, n));
    CHECK(cudaMalloc(&b, n));
    CHECK(cudaMemset(a, 0x10, n));
    bump<<<1, n>>>(a);
    CHECK(cudaMemcpy(b, a, n, cudaMemcpyDeviceToDevice));
    CHECK(cudaMemset(a, 0, n));
    unsigned char h[n];
    CHECK(cudaMemcpy(h, b, n, cudaMemcpyDeviceToHost));
    unsigned char z[n];
    CHECK(cudaMemcpy(z, a, n, cudaMemcpyDeviceToHost));
    std::printf("%d %d %d\n", h[0], h[63], z[10]);
    return 0;
}
