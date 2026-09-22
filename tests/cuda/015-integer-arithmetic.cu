// Signed and unsigned integer arithmetic on the device.
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

__global__ void ops(const int* in, int* out) {
    int i = threadIdx.x;
    int a = in[i], b = in[i + 8];
    unsigned ua = (unsigned)a;
    out[i * 6 + 0] = a / b;
    out[i * 6 + 1] = a % b;
    out[i * 6 + 2] = (int)(ua / 7u);
    out[i * 6 + 3] = a >> 3;
    out[i * 6 + 4] = (int)(ua >> 3);
    out[i * 6 + 5] = (a * 31) ^ (b << 2);
}

int main() {
    int in[16] = {100, -100, 7, -7, 2147483647, -2147483647, 12345, -1,
                  7, 7, -3, 3, 1000, 13, -11, 5};
    int *din, *dout;
    CHECK(cudaMalloc(&din, sizeof in));
    CHECK(cudaMalloc(&dout, 48 * sizeof(int)));
    CHECK(cudaMemcpy(din, in, sizeof in, cudaMemcpyHostToDevice));
    ops<<<1, 8>>>(din, dout);
    int out[48];
    CHECK(cudaMemcpy(out, dout, sizeof out, cudaMemcpyDeviceToHost));
    for (int v : out) std::printf("%d ", v);
    std::printf("\n");
    return 0;
}
