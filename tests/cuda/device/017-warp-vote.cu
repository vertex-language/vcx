// Warp votes and lane arithmetic: any, all, a ballot with a partial
// mask, a butterfly xor-shuffle reduction, and the bit intrinsics.
// grid: 1
// block: 32
// expect: 1 0 0x0000000f 496 5 27 3 0xc0000000
__global__ void test(int *out) {
  int lane = threadIdx.x;
  int any = __any_sync(0xffffffffu, lane == 7);
  int all = __all_sync(0xffffffffu, lane < 31);
  unsigned low = __ballot_sync(0x0000000fu, lane < 4);
  int v = lane;
  for (int m = 16; m > 0; m >>= 1) v += __shfl_xor_sync(0xffffffffu, v, m);
  if (lane == 0) {
    out[0] = any;
    out[1] = all;
    out[2] = (int)low;
    out[3] = v;
    out[4] = __popc(0x1fu);
    out[5] = __clz(16);
    out[6] = __ffs(4);
    out[7] = (int)__brev(3u);
  }
}
