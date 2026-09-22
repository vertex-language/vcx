// A kernel template, instantiated for int and double.
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
    CHECK(cudaMalloc(&x, sizeof hx));
    CHECK(cudaMalloc(&y, sizeof hy));
    CHECK(cudaMemcpy(x, hx, sizeof hx, cudaMemcpyHostToDevice));
    CHECK(cudaMemcpy(y, hy, sizeof hy, cudaMemcpyHostToDevice));
    axpy<T><<<1, n>>>(a, x, y, n);
    CHECK(cudaMemcpy(hy, y, sizeof hy, cudaMemcpyDeviceToHost));
    for (T v : hy) std::printf(fmt, v);
    std::printf("\n");
    return 0;
}

int main() {
    if (run<int>(3, "%d ")) return 1;
    return run<double>(0.5, "%g ");
}
