// Conversions between integer and floating types on the device.
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
    CHECK(cudaMalloc(&din, sizeof in));
    CHECK(cudaMalloc(&di, 6 * sizeof(int)));
    CHECK(cudaMalloc(&df, 6 * sizeof(float)));
    CHECK(cudaMalloc(&dw, 6 * sizeof(long long)));
    CHECK(cudaMemcpy(din, in, sizeof in, cudaMemcpyHostToDevice));
    conv<<<1, 6>>>(din, di, df, dw);
    int hi[6];
    float hf[6];
    long long hw[6];
    CHECK(cudaMemcpy(hi, di, sizeof hi, cudaMemcpyDeviceToHost));
    CHECK(cudaMemcpy(hf, df, sizeof hf, cudaMemcpyDeviceToHost));
    CHECK(cudaMemcpy(hw, dw, sizeof hw, cudaMemcpyDeviceToHost));
    for (int k = 0; k < 6; ++k) std::printf("%d %.1f %lld\n", hi[k], hf[k], hw[k]);
    return 0;
}
