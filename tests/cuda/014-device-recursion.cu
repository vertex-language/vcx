// A recursive __device__ function.
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

__device__ int ackermann_small(int m, int n) {
    if (m == 0) return n + 1;
    if (n == 0) return ackermann_small(m - 1, 1);
    return ackermann_small(m - 1, ackermann_small(m, n - 1));
}

__device__ long long fact(int n) { return n <= 1 ? 1 : n * fact(n - 1); }

__global__ void run(long long* out) {
    out[0] = ackermann_small(2, threadIdx.x);
    out[1] = fact(15);
}

int main() {
    long long* d;
    CHECK(cudaMalloc(&d, 2 * sizeof(long long)));
    run<<<1, 1>>>(d);
    long long h[2];
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    std::printf("%lld %lld\n", h[0], h[1]);
    CHECK(cudaFree(d));
    return 0;
}
