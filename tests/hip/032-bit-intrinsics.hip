// __popc, __clz, __ffs, __brev and the 64-bit forms.
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

__global__ void bits(const unsigned* in, unsigned* out) {
    int i = threadIdx.x;
    unsigned v = in[i];
    out[i * 5 + 0] = __popc(v);
    out[i * 5 + 1] = __clz(v);
    out[i * 5 + 2] = __ffs(v);
    out[i * 5 + 3] = __brev(v);
    out[i * 5 + 4] = __popcll(((unsigned long long)v << 32) | v);
}

int main() {
    unsigned in[4] = {0u, 1u, 0x80000000u, 0xDEADBEEFu};
    unsigned *din, *dout;
    CHECK(hipMalloc(&din, sizeof in));
    CHECK(hipMalloc(&dout, 20 * sizeof(unsigned)));
    CHECK(hipMemcpy(din, in, sizeof in, hipMemcpyHostToDevice));
    bits<<<1, 4>>>(din, dout);
    unsigned out[20];
    CHECK(hipMemcpy(out, dout, sizeof out, hipMemcpyDeviceToHost));
    for (int k = 0; k < 20; ++k) std::printf("%x%c", out[k], k % 5 == 4 ? '\n' : ' ');
    return 0;
}
