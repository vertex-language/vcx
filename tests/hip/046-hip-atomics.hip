// HIP's scoped atomics: __hip_atomic_fetch_add at workgroup and agent scope.
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

__global__ void count(int* global_total) {
    __shared__ int local;
    if (threadIdx.x == 0) local = 0;
    __syncthreads();
    __hip_atomic_fetch_add(&local, 1, __ATOMIC_RELAXED, __HIP_MEMORY_SCOPE_WORKGROUP);
    __syncthreads();
    if (threadIdx.x == 0)
        __hip_atomic_fetch_add(global_total, local, __ATOMIC_RELAXED, __HIP_MEMORY_SCOPE_AGENT);
}

int main() {
    int* d;
    CHECK(hipMalloc(&d, sizeof(int)));
    CHECK(hipMemset(d, 0, sizeof(int)));
    count<<<8, 200>>>(d);
    int h = 0;
    CHECK(hipMemcpy(&h, d, sizeof h, hipMemcpyDeviceToHost));
    std::printf("%d\n", h);
    return 0;
}
