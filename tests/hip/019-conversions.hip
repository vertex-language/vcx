// Conversions between integer and floating types on the device.
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

__global__ void conv(const float* in, int* ints, float* floats, long long* wide) {
    int i = threadIdx.x;
    ints[i] = (int)in[i];
    floats[i] = (float)(i * 1000003);
    wide[i] = (long long)((double)in[i] * 1e9);
}

int main() {
    float in[6] = {3.99f, -3.99f, 0.5f, -0.5f, 1e9f, -1234.5f};
    float* din;
    int* di;
    float* df;
    long long* dw;
    CHECK(hipMalloc(&din, sizeof in));
    CHECK(hipMalloc(&di, 6 * sizeof(int)));
    CHECK(hipMalloc(&df, 6 * sizeof(float)));
    CHECK(hipMalloc(&dw, 6 * sizeof(long long)));
    CHECK(hipMemcpy(din, in, sizeof in, hipMemcpyHostToDevice));
    conv<<<1, 6>>>(din, di, df, dw);
    int hi[6];
    float hf[6];
    long long hw[6];
    CHECK(hipMemcpy(hi, di, sizeof hi, hipMemcpyDeviceToHost));
    CHECK(hipMemcpy(hf, df, sizeof hf, hipMemcpyDeviceToHost));
    CHECK(hipMemcpy(hw, dw, sizeof hw, hipMemcpyDeviceToHost));
    for (int k = 0; k < 6; ++k) std::printf("%d %.1f %lld\n", hi[k], hf[k], hw[k]);
    return 0;
}
