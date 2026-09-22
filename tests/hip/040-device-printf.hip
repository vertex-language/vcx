// printf from a kernel, by one thread so the order is fixed.
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

__global__ void hello(int n) {
    if (threadIdx.x == 0 && blockIdx.x == 0) {
        printf("hello from the device: %d %u %.2f %s\n", n, 4000000000u, 3.14159, "text");
        for (int i = 0; i < 3; ++i) printf("line %d\n", i);
    }
}

int main() {
    hello<<<2, 64>>>(7);
    CHECK(hipDeviceSynchronize());
    std::printf("back on the host\n");
    return 0;
}
