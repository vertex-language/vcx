// A kernel template, instantiated for int and double.
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

template <typename T>
__global__ void axpy(T a, const T* x, T* y, int n) {
    int i = blockIdx.x * blockDim.x + threadIdx.x;
    if (i < n) y[i] = a * x[i] + y[i];
}

template <typename T>
int run(T a, const char* fmt) {
    const int n = 8;
    T hx[n], hy[n];
    for (int i = 0; i < n; ++i) {
        hx[i] = (T)i;
        hy[i] = (T)(10 * i);
    }
    T *x, *y;
    CHECK(hipMalloc(&x, sizeof hx));
    CHECK(hipMalloc(&y, sizeof hy));
    CHECK(hipMemcpy(x, hx, sizeof hx, hipMemcpyHostToDevice));
    CHECK(hipMemcpy(y, hy, sizeof hy, hipMemcpyHostToDevice));
    axpy<T><<<1, n>>>(a, x, y, n);
    CHECK(hipMemcpy(hy, y, sizeof hy, hipMemcpyDeviceToHost));
    for (T v : hy) std::printf(fmt, v);
    std::printf("\n");
    return 0;
}

int main() {
    if (run<int>(3, "%d ")) return 1;
    return run<double>(0.5, "%g ");
}
