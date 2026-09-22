// Events recorded and synchronized; the elapsed time is not printed.
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

__global__ void spin(int* v) {
    int x = threadIdx.x;
    for (int i = 0; i < 1000; ++i) x = x * 1103515245 + 12345;
    v[threadIdx.x] = x & 0xff;
}

int main() {
    cudaEvent_t start, stop;
    CHECK(cudaEventCreate(&start));
    CHECK(cudaEventCreate(&stop));
    int* d;
    CHECK(cudaMalloc(&d, 32 * sizeof(int)));
    CHECK(cudaEventRecord(start));
    spin<<<1, 32>>>(d);
    CHECK(cudaEventRecord(stop));
    CHECK(cudaEventSynchronize(stop));
    float ms = -1;
    CHECK(cudaEventElapsedTime(&ms, start, stop));
    int h[32];
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    std::printf("%d %d %d\n", ms >= 0, h[0], h[31]);
    CHECK(cudaEventDestroy(start));
    CHECK(cudaEventDestroy(stop));
    return 0;
}
