// A kernel writes one value, and the host copies it back.
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

__global__ void write(int* out) { *out = 42; }

int main() {
    int* d;
    CHECK(hipMalloc(&d, sizeof(int)));
    write<<<1, 1>>>(d);
    int h = 0;
    CHECK(hipMemcpy(&h, d, sizeof h, hipMemcpyDeviceToHost));
    CHECK(hipFree(d));
    std::printf("%d\n", h);
    return 0;
}
