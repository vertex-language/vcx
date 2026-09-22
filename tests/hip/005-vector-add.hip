// Vector addition: two inputs copied in, one output copied out.
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

__global__ void add(const float* a, const float* b, float* c, int n) {
    int i = blockIdx.x * blockDim.x + threadIdx.x;
    if (i < n) c[i] = a[i] + b[i];
}

int main() {
    const int n = 1024;
    float ha[n], hb[n], hc[n];
    for (int i = 0; i < n; ++i) {
        ha[i] = i * 0.5f;
        hb[i] = 1000 - i;
    }
    float *a, *b, *c;
    CHECK(hipMalloc(&a, sizeof ha));
    CHECK(hipMalloc(&b, sizeof hb));
    CHECK(hipMalloc(&c, sizeof hc));
    CHECK(hipMemcpy(a, ha, sizeof ha, hipMemcpyHostToDevice));
    CHECK(hipMemcpy(b, hb, sizeof hb, hipMemcpyHostToDevice));
    add<<<n / 256, 256>>>(a, b, c, n);
    CHECK(hipMemcpy(hc, c, sizeof hc, hipMemcpyDeviceToHost));
    double sum = 0;
    for (float v : hc) sum += v;
    std::printf("%g %g %g\n", hc[0], hc[n - 1], sum);
    hipFree(a);
    hipFree(b);
    hipFree(c);
    return 0;
}
