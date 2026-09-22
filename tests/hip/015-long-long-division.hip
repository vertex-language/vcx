// 64-bit division and remainder, which AMD GPUs have no instruction for.
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

__global__ void ops(const long long* in, long long* out) {
    int i = threadIdx.x;
    long long a = in[i], b = in[i + 4];
    unsigned long long ua = (unsigned long long)a;
    out[i * 4 + 0] = a / b;
    out[i * 4 + 1] = a % b;
    out[i * 4 + 2] = (long long)(ua / 1000003ull);
    out[i * 4 + 3] = (long long)(ua % 97ull);
}

int main() {
    long long in[8] = {9000000000000000000LL, -123456789012345LL, 1LL << 40, -1,
                       7, -1000, 3, 9223372036854775807LL};
    long long *din, *dout;
    CHECK(hipMalloc(&din, sizeof in));
    CHECK(hipMalloc(&dout, 16 * sizeof(long long)));
    CHECK(hipMemcpy(din, in, sizeof in, hipMemcpyHostToDevice));
    ops<<<1, 4>>>(din, dout);
    long long out[16];
    CHECK(hipMemcpy(out, dout, sizeof out, hipMemcpyDeviceToHost));
    for (long long v : out) std::printf("%lld ", v);
    std::printf("\n");
    return 0;
}
