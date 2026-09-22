// A tree reduction in shared memory, one partial sum per block.
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

__global__ void reduce(const int* in, int* out) {
    __shared__ int s[256];
    int t = threadIdx.x;
    s[t] = in[blockIdx.x * blockDim.x + t];
    __syncthreads();
    for (int stride = blockDim.x / 2; stride > 0; stride >>= 1) {
        if (t < stride) s[t] += s[t + stride];
        __syncthreads();
    }
    if (t == 0) out[blockIdx.x] = s[0];
}

int main() {
    const int n = 256 * 8;
    static int h[n];
    for (int i = 0; i < n; ++i) h[i] = (i * 7) % 101;
    int *din, *dout;
    CHECK(cudaMalloc(&din, sizeof h));
    CHECK(cudaMalloc(&dout, 8 * sizeof(int)));
    CHECK(cudaMemcpy(din, h, sizeof h, cudaMemcpyHostToDevice));
    reduce<<<8, 256>>>(din, dout);
    int partial[8];
    CHECK(cudaMemcpy(partial, dout, sizeof partial, cudaMemcpyDeviceToHost));
    int total = 0;
    for (int v : partial) {
        std::printf("%d ", v);
        total += v;
    }
    std::printf("= %d\n", total);
    return 0;
}
