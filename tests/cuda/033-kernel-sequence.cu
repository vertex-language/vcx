// Several kernels in sequence on the same data.
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

__global__ void init(int* v) { v[threadIdx.x] = threadIdx.x; }
__global__ void square(int* v) { v[threadIdx.x] *= v[threadIdx.x]; }
__global__ void neighbor(const int* in, int* out) {
    int t = threadIdx.x;
    out[t] = in[t] + (t + 1 < blockDim.x ? in[t + 1] : 0);
}

int main() {
    const int n = 16;
    int *a, *b;
    CHECK(cudaMalloc(&a, n * sizeof(int)));
    CHECK(cudaMalloc(&b, n * sizeof(int)));
    init<<<1, n>>>(a);
    square<<<1, n>>>(a);
    neighbor<<<1, n>>>(a, b);
    int h[n];
    CHECK(cudaMemcpy(h, b, sizeof h, cudaMemcpyDeviceToHost));
    for (int v : h) std::printf("%d ", v);
    std::printf("\n");
    return 0;
}
