// An empty kernel, launched and waited for.
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

__global__ void nothing() {}

int main() {
    nothing<<<1, 1>>>();
    CHECK(hipGetLastError());
    CHECK(hipDeviceSynchronize());
    std::printf("done\n");
    return 0;
}
