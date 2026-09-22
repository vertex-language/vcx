// A lambda defined and called inside a kernel.
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

__global__ void apply(int* v) {
    int base = 7;
    auto f = [base](int x) { return x * x + base; };
    auto g = [&](int x) { return f(x) - f(x - 1); };
    v[threadIdx.x] = g(threadIdx.x + 1);
}

int main() {
    int* d;
    CHECK(hipMalloc(&d, 10 * sizeof(int)));
    apply<<<1, 10>>>(d);
    int h[10];
    CHECK(hipMemcpy(h, d, sizeof h, hipMemcpyDeviceToHost));
    for (int v : h) std::printf("%d ", v);
    std::printf("\n");
    return 0;
}
