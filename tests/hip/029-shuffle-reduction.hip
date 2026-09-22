// A wave sum with __shfl_down, over whatever the wave size is.
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

__global__ void wave_sum(const int* in, int* out) {
    int v = in[threadIdx.x];
    for (int offset = warpSize / 2; offset > 0; offset >>= 1) v += __shfl_down(v, offset);
    if (threadIdx.x % warpSize == 0) atomicAdd(out, v);
}

int main() {
    const int n = 256;
    int h[n];
    for (int i = 0; i < n; ++i) h[i] = i % 13;
    int *din, *dout;
    CHECK(hipMalloc(&din, sizeof h));
    CHECK(hipMalloc(&dout, sizeof(int)));
    CHECK(hipMemset(dout, 0, sizeof(int)));
    CHECK(hipMemcpy(din, h, sizeof h, hipMemcpyHostToDevice));
    wave_sum<<<1, n>>>(din, dout);
    int out = 0;
    CHECK(hipMemcpy(&out, dout, sizeof out, hipMemcpyDeviceToHost));
    std::printf("%d\n", out);
    return 0;
}
