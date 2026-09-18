// A 3-D grid of 3-D blocks: every thread's flat index from the six
// coordinates, and the extents seen from the device.
// grid: 2 3 2
// block: 2 2 2
// expect: 96 2 3 2 2 2 2 95
__global__ void test(int *out) {
  int bx = blockIdx.x, by = blockIdx.y, bz = blockIdx.z;
  int tx = threadIdx.x, ty = threadIdx.y, tz = threadIdx.z;
  int block = (bz * gridDim.y + by) * gridDim.x + bx;
  int thread = (tz * blockDim.y + ty) * blockDim.x + tx;
  int flat = block * (blockDim.x * blockDim.y * blockDim.z) + thread;
  atomicAdd(&out[0], 1);
  atomicMax(&out[7], flat);
  if (flat == 0) {
    out[1] = gridDim.x; out[2] = gridDim.y; out[3] = gridDim.z;
    out[4] = blockDim.x; out[5] = blockDim.y; out[6] = blockDim.z;
  }
}
