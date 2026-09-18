// Warp primitives: a shuffle-down reduction over one warp of 32 lanes
// summing the lane ids, a ballot of the even lanes, and the population.
// grid: 1
// block: 32
// expect: 496 0x55555555 16
__global__ void test(int *out) {
  int lane = threadIdx.x;
  int v = lane;
  for (int off = 16; off > 0; off >>= 1) v += __shfl_down_sync(0xffffffffu, v, off);
  unsigned even = __ballot_sync(0xffffffffu, (lane & 1) == 0);
  if (lane == 0) {
    out[0] = v;
    out[1] = (int)even;
    out[2] = __popc(even);
  }
}
