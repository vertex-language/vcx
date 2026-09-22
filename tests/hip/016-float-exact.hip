// Float operations whose results are exact: sqrt, fma, min, max, rounding.
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

__global__ void ops(const float* in, float* out) {
    int i = threadIdx.x;
    float x = in[i];
    out[i * 7 + 0] = sqrtf(fabsf(x));
    out[i * 7 + 1] = fmaf(x, 3.0f, 0.5f);
    out[i * 7 + 2] = fminf(x, 1.0f);
    out[i * 7 + 3] = fmaxf(x, -1.0f);
    out[i * 7 + 4] = floorf(x);
    out[i * 7 + 5] = ceilf(x);
    out[i * 7 + 6] = truncf(x) + rintf(x);
}

int main() {
    float in[8] = {4.0f, -2.25f, 0.5f, 100.0f, -0.75f, 2.5f, 9.0f, -16.0f};
    float *din, *dout;
    CHECK(hipMalloc(&din, sizeof in));
    CHECK(hipMalloc(&dout, 56 * sizeof(float)));
    CHECK(hipMemcpy(din, in, sizeof in, hipMemcpyHostToDevice));
    ops<<<1, 8>>>(din, dout);
    float out[56];
    CHECK(hipMemcpy(out, dout, sizeof out, hipMemcpyDeviceToHost));
    for (float v : out) std::printf("%g ", v);
    std::printf("\n");
    return 0;
}
