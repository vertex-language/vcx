// A grid-stride loop covers more elements than threads.
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

__global__ void fill(long long* v, int n) {
    for (int i = blockIdx.x * blockDim.x + threadIdx.x; i < n; i += gridDim.x * blockDim.x)
        v[i] = (long long)i * 3 + 1;
}

int main() {
    const int n = 100000;
    long long* d;
    CHECK(hipMalloc(&d, n * sizeof(long long)));
    fill<<<4, 64>>>(d, n);
    static long long h[n];
    CHECK(hipMemcpy(h, d, sizeof h, hipMemcpyDeviceToHost));
    long long sum = 0;
    for (long long v : h) sum += v;
    std::printf("%lld %lld\n", h[n - 1], sum);
    CHECK(hipFree(d));
    return 0;
}
