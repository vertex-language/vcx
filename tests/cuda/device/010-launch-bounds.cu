// __launch_bounds__ on the kernel: .maxntid in the PTX, and the kernel
// still runs at a block within it.
// grid: 1
// block: 4
// expect: 0 2 4 6
__global__ void __launch_bounds__(128, 2) test(int *out) { out[threadIdx.x] = threadIdx.x * 2; }
