// extern __shared__ memory sized at launch.
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

__global__ void rotate(int* v, int shift) {
    extern __shared__ int buf[];
    int t = threadIdx.x, n = blockDim.x;
    buf[t] = v[t];
    __syncthreads();
    v[t] = buf[(t + shift) % n];
}

int main() {
    const int n = 100;
    int h[n];
    for (int i = 0; i < n; ++i) h[i] = i * 10;
    int* d;
    CHECK(hipMalloc(&d, sizeof h));
    CHECK(hipMemcpy(d, h, sizeof h, hipMemcpyHostToDevice));
    hipLaunchKernelGGL(rotate, dim3(1), dim3(n), n * sizeof(int), 0, d, 37);
    CHECK(hipMemcpy(h, d, sizeof h, hipMemcpyDeviceToHost));
    std::printf("%d %d %d\n", h[0], h[62], h[63]);
    CHECK(hipFree(d));
    return 0;
}
