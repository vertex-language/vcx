// A __shared__ array and __syncthreads: reversing a block.
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

__global__ void reverse(int* v) {
    __shared__ int tile[256];
    int t = threadIdx.x;
    tile[t] = v[blockIdx.x * blockDim.x + t];
    __syncthreads();
    v[blockIdx.x * blockDim.x + t] = tile[blockDim.x - 1 - t];
}

int main() {
    const int n = 512;
    int h[n];
    for (int i = 0; i < n; ++i) h[i] = i;
    int* d;
    CHECK(hipMalloc(&d, sizeof h));
    CHECK(hipMemcpy(d, h, sizeof h, hipMemcpyHostToDevice));
    reverse<<<2, 256>>>(d);
    CHECK(hipMemcpy(h, d, sizeof h, hipMemcpyDeviceToHost));
    std::printf("%d %d %d %d\n", h[0], h[255], h[256], h[511]);
    CHECK(hipFree(d));
    return 0;
}
