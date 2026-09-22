// double arithmetic on the device.
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

__global__ void ops(const double* in, double* out) {
    int i = threadIdx.x;
    double x = in[i];
    out[i * 4 + 0] = x / 3.0;
    out[i * 4 + 1] = sqrt(x * x + 1.0);
    out[i * 4 + 2] = (double)(long long)(x * 1e6);
    out[i * 4 + 3] = x * x - 2.0 * x;
}

int main() {
    double in[4] = {1.0, 2.0, 0.125, -7.25};  // squares exact, so a fused multiply-add rounds the same
    double *din, *dout;
    CHECK(cudaMalloc(&din, sizeof in));
    CHECK(cudaMalloc(&dout, 16 * sizeof(double)));
    CHECK(cudaMemcpy(din, in, sizeof in, cudaMemcpyHostToDevice));
    ops<<<1, 4>>>(din, dout);
    double out[16];
    CHECK(cudaMemcpy(out, dout, sizeof out, cudaMemcpyDeviceToHost));
    for (double v : out) std::printf("%.15g ", v);
    std::printf("\n");
    return 0;
}
