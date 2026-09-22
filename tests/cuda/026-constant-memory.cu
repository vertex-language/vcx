// __constant__ memory written with cudaMemcpyToSymbol.
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

__constant__ float coeffs[4];

__global__ void poly(const float* x, float* y) {
    int i = threadIdx.x;
    float v = x[i];
    y[i] = coeffs[0] + v * (coeffs[1] + v * (coeffs[2] + v * coeffs[3]));
}

int main() {
    float c[4] = {1.0f, 0.5f, 0.25f, 0.125f};
    CHECK(cudaMemcpyToSymbol(coeffs, c, sizeof c));
    float hx[8];
    for (int i = 0; i < 8; ++i) hx[i] = (float)i;
    float *x, *y;
    CHECK(cudaMalloc(&x, sizeof hx));
    CHECK(cudaMalloc(&y, sizeof hx));
    CHECK(cudaMemcpy(x, hx, sizeof hx, cudaMemcpyHostToDevice));
    poly<<<1, 8>>>(x, y);
    float hy[8];
    CHECK(cudaMemcpy(hy, y, sizeof hy, cudaMemcpyDeviceToHost));
    for (float v : hy) std::printf("%g ", v);
    std::printf("\n");
    return 0;
}
