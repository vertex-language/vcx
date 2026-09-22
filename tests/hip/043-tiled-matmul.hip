// Matrix multiplication with shared-memory tiles.
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

constexpr int TILE = 16;

__global__ void matmul(const int* a, const int* b, int* c, int n) {
    __shared__ int ta[TILE][TILE], tb[TILE][TILE];
    int row = blockIdx.y * TILE + threadIdx.y, col = blockIdx.x * TILE + threadIdx.x;
    int sum = 0;
    for (int t = 0; t < n / TILE; ++t) {
        ta[threadIdx.y][threadIdx.x] = a[row * n + t * TILE + threadIdx.x];
        tb[threadIdx.y][threadIdx.x] = b[(t * TILE + threadIdx.y) * n + col];
        __syncthreads();
        for (int k = 0; k < TILE; ++k) sum += ta[threadIdx.y][k] * tb[k][threadIdx.x];
        __syncthreads();
    }
    c[row * n + col] = sum;
}

int main() {
    const int n = 64;
    static int a[n * n], b[n * n], c[n * n];
    for (int i = 0; i < n * n; ++i) {
        a[i] = i % 7 - 3;
        b[i] = i % 5 - 2;
    }
    int *da, *db, *dc;
    CHECK(hipMalloc(&da, sizeof a));
    CHECK(hipMalloc(&db, sizeof b));
    CHECK(hipMalloc(&dc, sizeof c));
    CHECK(hipMemcpy(da, a, sizeof a, hipMemcpyHostToDevice));
    CHECK(hipMemcpy(db, b, sizeof b, hipMemcpyHostToDevice));
    matmul<<<dim3(n / TILE, n / TILE), dim3(TILE, TILE)>>>(da, db, dc, n);
    CHECK(hipMemcpy(c, dc, sizeof c, hipMemcpyDeviceToHost));
    long long trace = 0, sum = 0;
    for (int i = 0; i < n; ++i) trace += c[i * n + i];
    for (int v : c) sum += v;
    std::printf("%lld %lld %d\n", trace, sum, c[5 * n + 9]);
    return 0;
}
