// atomicAdd on integers: a histogram, many threads to a bin.
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

__global__ void histogram(const unsigned char* data, int n, unsigned* bins) {
    for (int i = blockIdx.x * blockDim.x + threadIdx.x; i < n; i += gridDim.x * blockDim.x)
        atomicAdd(&bins[data[i] % 16], 1u);
}

int main() {
    const int n = 65536;
    static unsigned char h[n];
    for (int i = 0; i < n; ++i) h[i] = (unsigned char)((i * 2654435761u) >> 24);
    unsigned char* d;
    unsigned* bins;
    CHECK(hipMalloc(&d, n));
    CHECK(hipMalloc(&bins, 16 * sizeof(unsigned)));
    CHECK(hipMemset(bins, 0, 16 * sizeof(unsigned)));
    CHECK(hipMemcpy(d, h, n, hipMemcpyHostToDevice));
    histogram<<<32, 256>>>(d, n, bins);
    unsigned out[16];
    CHECK(hipMemcpy(out, bins, sizeof out, hipMemcpyDeviceToHost));
    unsigned total = 0;
    for (unsigned v : out) {
        std::printf("%u ", v);
        total += v;
    }
    std::printf("= %u\n", total);
    return 0;
}
