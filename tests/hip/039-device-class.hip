// A class with member functions used on the device.
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

struct Complex {
    float re, im;
    __host__ __device__ Complex operator*(const Complex& o) const {
        return {re * o.re - im * o.im, re * o.im + im * o.re};
    }
    __host__ __device__ Complex operator+(const Complex& o) const { return {re + o.re, im + o.im}; }
    __host__ __device__ float norm2() const { return re * re + im * im; }
};

__global__ void powers(Complex z, float* out) {
    Complex p{1, 0};
    for (int i = 0; i < (int)threadIdx.x; ++i) p = p * z;
    out[threadIdx.x] = (p + Complex{1, 1}).norm2();
}

int main() {
    float* d;
    CHECK(hipMalloc(&d, 8 * sizeof(float)));
    powers<<<1, 8>>>(Complex{0, 1}, d);
    float h[8];
    CHECK(hipMemcpy(h, d, sizeof h, hipMemcpyDeviceToHost));
    for (float v : h) std::printf("%g ", v);
    std::printf("\n");
    return 0;
}
