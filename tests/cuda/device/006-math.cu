// The device math library: the functions the hardware does in an
// instruction, each result scaled to an integer.
// grid: 1
// block: 1
// expect: 40 -30 5 -2 3 100 7 12 250
__global__ void test(int *out) {
  float x = 16.0f;
  out[0] = (int)(sqrtf(x) * 10.0f);
  out[1] = (int)(floorf(-2.5f) * 10.0f);
  out[2] = (int)fabsf(-5.0f);
  out[3] = (int)ceilf(-2.5f);
  out[4] = (int)truncf(3.9f);
  out[5] = (int)(fmaf(3.0f, 30.0f, 10.0f));
  out[6] = (int)fmaxf(7.0f, 2.0f);
  out[7] = (int)sqrt(144.0);
  out[8] = (int)(rsqrtf(0.16f) * 100.0f);
}
