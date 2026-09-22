// Events recorded and synchronized; the elapsed time is not printed.
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

__global__ void spin(int* v) {
    int x = threadIdx.x;
    for (int i = 0; i < 1000; ++i) x = x * 1103515245 + 12345;
    v[threadIdx.x] = x & 0xff;
}

int main() {
    hipEvent_t start, stop;
    CHECK(hipEventCreate(&start));
    CHECK(hipEventCreate(&stop));
    int* d;
    CHECK(hipMalloc(&d, 32 * sizeof(int)));
    CHECK(hipEventRecord(start, 0));
    spin<<<1, 32>>>(d);
    CHECK(hipEventRecord(stop, 0));
    CHECK(hipEventSynchronize(stop));
    float ms = -1;
    CHECK(hipEventElapsedTime(&ms, start, stop));
    int h[32];
    CHECK(hipMemcpy(h, d, sizeof h, hipMemcpyDeviceToHost));
    std::printf("%d %d %d\n", ms >= 0, h[0], h[31]);
    CHECK(hipEventDestroy(start));
    CHECK(hipEventDestroy(stop));
    return 0;
}
