// The device math library: exp, log, sin, cos, pow, printed to four places.
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

__global__ void ops(float* out) {
    float x = 0.25f * (threadIdx.x + 1);
    out[threadIdx.x * 6 + 0] = expf(x);
    out[threadIdx.x * 6 + 1] = logf(x);
    out[threadIdx.x * 6 + 2] = sinf(x);
    out[threadIdx.x * 6 + 3] = cosf(x);
    out[threadIdx.x * 6 + 4] = powf(x, 1.5f);
    out[threadIdx.x * 6 + 5] = tanhf(x);
}

int main() {
    float* d;
    CHECK(cudaMalloc(&d, 48 * sizeof(float)));
    ops<<<1, 8>>>(d);
    float h[48];
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    for (float v : h) std::printf("%.4f ", v);
    std::printf("\n");
    CHECK(cudaFree(d));
    return 0;
}
