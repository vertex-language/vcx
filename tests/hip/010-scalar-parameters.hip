// Kernel parameters of every scalar type.
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

__global__ void pack(double* out, char c, short s, int i, long long l, float f, double d, bool b,
                     unsigned u) {
    out[0] = c;
    out[1] = s;
    out[2] = i;
    out[3] = (double)l;
    out[4] = f;
    out[5] = d;
    out[6] = b;
    out[7] = u;
}

int main() {
    double* d;
    CHECK(hipMalloc(&d, 8 * sizeof(double)));
    pack<<<1, 1>>>(d, 'A', -1234, 99999, 1LL << 40, 0.25f, -2.5, true, 4000000000u);
    double h[8];
    CHECK(hipMemcpy(h, d, sizeof h, hipMemcpyDeviceToHost));
    for (double v : h) std::printf("%.17g ", v);
    std::printf("\n");
    CHECK(hipFree(d));
    return 0;
}
