// Two streams, asynchronous copies, and synchronizing each.
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

__global__ void scale(float* v, float k, int n) {
    int i = blockIdx.x * blockDim.x + threadIdx.x;
    if (i < n) v[i] *= k;
}

int main() {
    const int n = 4096;
    static float h[2][n];
    for (int i = 0; i < n; ++i) h[0][i] = h[1][i] = (float)i;
    hipStream_t s[2];
    float* d[2];
    for (int k = 0; k < 2; ++k) {
        CHECK(hipStreamCreate(&s[k]));
        CHECK(hipMalloc(&d[k], n * sizeof(float)));
        CHECK(hipMemcpyAsync(d[k], h[k], n * sizeof(float), hipMemcpyHostToDevice, s[k]));
        hipLaunchKernelGGL(scale, dim3(n / 256), dim3(256), 0, s[k], d[k], k ? 3.0f : 0.5f, n);
        CHECK(hipMemcpyAsync(h[k], d[k], n * sizeof(float), hipMemcpyDeviceToHost, s[k]));
    }
    for (int k = 0; k < 2; ++k) {
        CHECK(hipStreamSynchronize(s[k]));
        CHECK(hipStreamDestroy(s[k]));
    }
    std::printf("%g %g %g %g\n", h[0][1], h[0][n - 1], h[1][1], h[1][n - 1]);
    return 0;
}
