// cudaMallocManaged: one pointer the host and the device both use.
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

__global__ void inc(int* v, int n) {
    int i = blockIdx.x * blockDim.x + threadIdx.x;
    if (i < n) v[i] += i;
}

int main() {
    const int n = 1000;
    int* v;
    CHECK(cudaMallocManaged(&v, n * sizeof(int)));
    for (int i = 0; i < n; ++i) v[i] = 1;
    inc<<<(n + 255) / 256, 256>>>(v, n);
    CHECK(cudaDeviceSynchronize());
    long sum = 0;
    for (int i = 0; i < n; ++i) sum += v[i];
    std::printf("%ld %d\n", sum, v[999]);
    CHECK(cudaFree(v));
    return 0;
}
