// A program: a fixed-point Mandelbrot set, printed as ASCII art.
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

constexpr int W = 64, H = 24, MAXIT = 200;

// Fixed point with 24 fractional bits, so every step is exact integer arithmetic.
__global__ void mandel(unsigned char* out) {
    int x = blockIdx.x * blockDim.x + threadIdx.x;
    int y = blockIdx.y * blockDim.y + threadIdx.y;
    if (x >= W || y >= H) return;
    const long long one = 1LL << 24;
    long long cr = -2 * one + (3 * one / W) * x;
    long long ci = -one + (2 * one / H) * y;
    long long zr = 0, zi = 0;
    int it = 0;
    while (it < MAXIT) {
        long long zr2 = (zr * zr) >> 24, zi2 = (zi * zi) >> 24;
        if (zr2 + zi2 > 4 * one) break;
        zi = ((2 * zr * zi) >> 24) + ci;
        zr = zr2 - zi2 + cr;
        ++it;
    }
    out[y * W + x] = (unsigned char)it;
}

int main() {
    unsigned char* d;
    CHECK(cudaMalloc(&d, W * H));
    mandel<<<dim3((W + 15) / 16, (H + 7) / 8), dim3(16, 8)>>>(d);
    unsigned char h[W * H];
    CHECK(cudaMemcpy(h, d, sizeof h, cudaMemcpyDeviceToHost));
    const char* shade = " .:-=+*#%@";
    for (int y = 0; y < H; ++y) {
        for (int x = 0; x < W; ++x) {
            int it = h[y * W + x];
            std::putchar(it == MAXIT ? '@' : shade[it % 9]);
        }
        std::putchar('\n');
    }
    return 0;
}
