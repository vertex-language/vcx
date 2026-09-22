// An exclusive prefix sum in shared memory (Blelloch).
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

__global__ void scan(int* v, int n) {
    extern __shared__ int s[];
    int t = threadIdx.x;
    s[2 * t] = v[2 * t];
    s[2 * t + 1] = v[2 * t + 1];
    int offset = 1;
    for (int d = n >> 1; d > 0; d >>= 1) {
        __syncthreads();
        if (t < d) s[offset * (2 * t + 2) - 1] += s[offset * (2 * t + 1) - 1];
        offset <<= 1;
    }
    if (t == 0) s[n - 1] = 0;
    for (int d = 1; d < n; d <<= 1) {
        offset >>= 1;
        __syncthreads();
        if (t < d) {
            int ai = offset * (2 * t + 1) - 1, bi = offset * (2 * t + 2) - 1;
            int x = s[ai];
            s[ai] = s[bi];
            s[bi] += x;
        }
    }
    __syncthreads();
    v[2 * t] = s[2 * t];
    v[2 * t + 1] = s[2 * t + 1];
}

int main() {
    const int n = 512;
    int h[n];
    for (int i = 0; i < n; ++i) h[i] = i % 9 + 1;
    int* d;
    CHECK(cudaMalloc(&d, sizeof h));
    CHECK(cudaMemcpy(d, h, sizeof h, cudaMemcpyHostToDevice));
    scan<<<1, n / 2, n * sizeof(int)>>>(d, n);
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    std::printf("%d %d %d %d\n", h[0], h[1], h[100], h[511]);
    return 0;
}
