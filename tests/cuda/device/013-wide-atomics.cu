// 64-bit, float and bitwise atomics, compare-and-swap and exchange, all
// straight into the output: out[0] counts, out[2..3] hold a 64-bit sum,
// out[4] a float sum as bits, out[5] the OR of one bit per lane,
// out[6] a CAS-loop counter, out[7] an exchanged value's presence.
// grid: 2
// block: 32
// expect: 64 -63 2016 0 0x44fc0000 0x0000ffff 64 1
__global__ void test(int *out) {
  int i = blockIdx.x * blockDim.x + threadIdx.x;
  atomicAdd(&out[0], 1);
  atomicMin(&out[1], -i);
  atomicAdd((unsigned long long *)&out[2], (unsigned long long)i);
  atomicAdd((float *)&out[4], (float)i);
  atomicOr((unsigned *)&out[5], 1u << (i % 16));
  int seen = out[6], old;
  do {
    old = seen;
    seen = atomicCAS(&out[6], old, old + 1);
  } while (seen != old);
  if (i == 5) {
    int prev = atomicExch(&out[7], 1);
    (void)prev;
  }
}
