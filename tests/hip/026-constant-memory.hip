// __constant__ memory written with hipMemcpyToSymbol.
#include <cstdio>
#include <hip/hip_runtime.h>

#define CHECK(x)                                                       \
    do {                                                               \
        hipError_t e_ = (x);                                           \
        if (e_ != hipSuccess) {                                        \
            std::printf("HIP error %d at line %d\n", (int)e_, __LINE__); \
            return 1;                                                  \
        }                                                              \
    } while (0)

__constant__ float coeffs[4];

__global__ void poly(const float* x, float* y) {
    int i = threadIdx.x;
    float v = x[i];
    y[i] = coeffs[0] + v * (coeffs[1] + v * (coeffs[2] + v * coeffs[3]));
}

int main() {
    float c[4] = {1.0f, 0.5f, 0.25f, 0.125f};
    CHECK(hipMemcpyToSymbol(HIP_SYMBOL(coeffs), c, sizeof c));
    float hx[8];
    for (int i = 0; i < 8; ++i) hx[i] = (float)i;
    float *x, *y;
    CHECK(hipMalloc(&x, sizeof hx));
    CHECK(hipMalloc(&y, sizeof hx));
    CHECK(hipMemcpy(x, hx, sizeof hx, hipMemcpyHostToDevice));
    poly<<<1, 8>>>(x, y);
    float hy[8];
    CHECK(hipMemcpy(hy, y, sizeof hy, hipMemcpyDeviceToHost));
    for (float v : hy) std::printf("%g ", v);
    std::printf("\n");
    return 0;
}
