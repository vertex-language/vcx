// A grid larger than the data: threads past the end do nothing.
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

__global__ void square(int* v, int n) {
    int i = blockIdx.x * blockDim.x + threadIdx.x;
    if (i < n) v[i] = i * i;
}

int main() {
    const int n = 1000, threads = 128, blocks = (n + threads - 1) / threads;
    int* d;
    CHECK(hipMalloc(&d, (n + 24) * sizeof(int)));
    CHECK(hipMemset(d, 0xff, (n + 24) * sizeof(int)));
    square<<<blocks, threads>>>(d, n);
    int h[n + 24];
    CHECK(hipMemcpy(h, d, sizeof h, hipMemcpyDeviceToHost));
    std::printf("%d %d %d %d\n", blocks, h[999], h[1000], h[1023]);
    CHECK(hipFree(d));
    return 0;
}
