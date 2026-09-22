// The vector types: float4, int2, make_float4.
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

__global__ void vec(const float4* in, float4* out, int2* idx) {
    int i = threadIdx.x;
    float4 v = in[i];
    out[i] = make_float4(v.w, v.z, v.y, v.x + v.y + v.z + v.w);
    idx[i] = make_int2(i, -i);
}

int main() {
    const int n = 4;
    float4 h[n];
    for (int i = 0; i < n; ++i) h[i] = make_float4(i, i + 0.5f, i * 2.0f, -i);
    float4 *din, *dout;
    int2* didx;
    CHECK(hipMalloc(&din, sizeof h));
    CHECK(hipMalloc(&dout, sizeof h));
    CHECK(hipMalloc(&didx, n * sizeof(int2)));
    CHECK(hipMemcpy(din, h, sizeof h, hipMemcpyHostToDevice));
    vec<<<1, n>>>(din, dout, didx);
    int2 idx[n];
    CHECK(hipMemcpy(h, dout, sizeof h, hipMemcpyDeviceToHost));
    CHECK(hipMemcpy(idx, didx, sizeof idx, hipMemcpyDeviceToHost));
    for (int i = 0; i < n; ++i) std::printf("%g %g %g %g %d %d\n", h[i].x, h[i].y, h[i].z, h[i].w, idx[i].x, idx[i].y);
    std::printf("%zu\n", sizeof(float4));
    return 0;
}
