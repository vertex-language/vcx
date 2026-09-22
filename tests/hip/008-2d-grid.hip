// A 2D grid and 2D blocks, with dim3.
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

__global__ void coords(int* out, int w, int h) {
    int x = blockIdx.x * blockDim.x + threadIdx.x;
    int y = blockIdx.y * blockDim.y + threadIdx.y;
    if (x < w && y < h) out[y * w + x] = x * 100 + y;
}

int main() {
    const int w = 37, h = 19;
    int* d;
    CHECK(hipMalloc(&d, w * h * sizeof(int)));
    dim3 block(8, 4);
    dim3 grid((w + block.x - 1) / block.x, (h + block.y - 1) / block.y);
    coords<<<grid, block>>>(d, w, h);
    int out[w * h];
    CHECK(hipMemcpy(out, d, sizeof out, hipMemcpyDeviceToHost));
    long sum = 0;
    for (int v : out) sum += v;
    std::printf("%u %u %d %ld\n", grid.x, grid.y, out[18 * w + 36], sum);
    CHECK(hipFree(d));
    return 0;
}
