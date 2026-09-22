// __popc, __clz, __ffs, __brev and __byte_perm.
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

__global__ void bits(const unsigned* in, unsigned* out) {
    int i = threadIdx.x;
    unsigned v = in[i];
    out[i * 5 + 0] = __popc(v);
    out[i * 5 + 1] = __clz(v);
    out[i * 5 + 2] = __ffs(v);
    out[i * 5 + 3] = __brev(v);
    out[i * 5 + 4] = __byte_perm(v, 0x11223344u, 0x5140);
}

int main() {
    unsigned in[4] = {0u, 1u, 0x80000000u, 0xDEADBEEFu};
    unsigned *din, *dout;
    CHECK(cudaMalloc(&din, sizeof in));
    CHECK(cudaMalloc(&dout, 20 * sizeof(unsigned)));
    CHECK(cudaMemcpy(din, in, sizeof in, cudaMemcpyHostToDevice));
    bits<<<1, 4>>>(din, dout);
    unsigned out[20];
    CHECK(cudaMemcpy(out, dout, sizeof out, cudaMemcpyDeviceToHost));
    for (int k = 0; k < 20; ++k) std::printf("%x%c", out[k], k % 5 == 4 ? '\n' : ' ');
    return 0;
}
