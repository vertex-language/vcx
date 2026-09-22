// Signed and unsigned integer arithmetic on the device.
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
    CHECK(hipMalloc(&din, sizeof in));
    CHECK(hipMalloc(&dout, 48 * sizeof(int)));
    CHECK(hipMemcpy(din, in, sizeof in, hipMemcpyHostToDevice));
    ops<<<1, 8>>>(din, dout);
    int out[48];
    CHECK(hipMemcpy(out, dout, sizeof out, hipMemcpyDeviceToHost));
    for (int v : out) std::printf("%d ", v);
    std::printf("\n");
    return 0;
}
