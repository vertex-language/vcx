// Two streams, asynchronous copies, and synchronizing each.
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

__global__ void scale(float* v, float k, int n) {
    int i = blockIdx.x * blockDim.x + threadIdx.x;
    if (i < n) v[i] *= k;
}

int main() {
    const int n = 4096;
    static float h[2][n];
    for (int i = 0; i < n; ++i) h[0][i] = h[1][i] = (float)i;
    cudaStream_t s[2];
    float* d[2];
    for (int k = 0; k < 2; ++k) {
        CHECK(cudaStreamCreate(&s[k]));
        CHECK(cudaMalloc(&d[k], n * sizeof(float)));
        CHECK(cudaMemcpyAsync(d[k], h[k], n * sizeof(float), cudaMemcpyHostToDevice, s[k]));
        scale<<<n / 256, 256, 0, s[k]>>>(d[k], k ? 3.0f : 0.5f, n);
        CHECK(cudaMemcpyAsync(h[k], d[k], n * sizeof(float), cudaMemcpyDeviceToHost, s[k]));
    }
    for (int k = 0; k < 2; ++k) {
        CHECK(cudaStreamSynchronize(s[k]));
        CHECK(cudaStreamDestroy(s[k]));
    }
    std::printf("%g %g %g %g\n", h[0][1], h[0][n - 1], h[1][1], h[1][n - 1]);
    return 0;
}
