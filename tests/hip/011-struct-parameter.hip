// A struct passed to a kernel by value.
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

struct Params {
    int scale;
    int offset;
    float weights[3];
};

__global__ void apply(int* out, Params p) {
    int i = threadIdx.x;
    out[i] = i * p.scale + p.offset + (int)(p.weights[i % 3] * 4);
}

int main() {
    Params p{3, 100, {0.25f, 0.5f, 1.0f}};
    int* d;
    CHECK(hipMalloc(&d, 9 * sizeof(int)));
    apply<<<1, 9>>>(d, p);
    int h[9];
    CHECK(hipMemcpy(h, d, sizeof h, hipMemcpyDeviceToHost));
    for (int v : h) std::printf("%d ", v);
    std::printf("\n");
    CHECK(hipFree(d));
    return 0;
}
