// A struct passed to a kernel by value.
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

struct Params {
    int scale;
    int offset;
    float weights[3];
};

__global__ void apply(int* out, Params p) {
    int i = threadIdx.x;
    out[i] = i * p.scale + p.offset + (int)(p.weights[i % 3] * 4);
}

int main() {
    Params p{3, 100, {0.25f, 0.5f, 1.0f}};
    int* d;
    CHECK(cudaMalloc(&d, 9 * sizeof(int)));
    apply<<<1, 9>>>(d, p);
    int h[9];
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    for (int v : h) std::printf("%d ", v);
    std::printf("\n");
    CHECK(cudaFree(d));
    return 0;
}
