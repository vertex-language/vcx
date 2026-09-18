// Narrow and wide integer types on the device: bytes, shorts, bools,
// 64-bit division and shifts, and unsigned compares.
// grid: 1
// block: 1
// expect: 200 -56 65535 -1 1 305419896 -2 0 1000000 7 1
__global__ void test(int *out) {
  unsigned char uc = 200;
  signed char sc = (signed char)uc;
  unsigned short us = 65535;
  short ss = (short)us;
  bool b = uc > 100;
  unsigned long long big = 0x12345678ull << 32 | 0x9abcdef0ull;
  long long neg = -1000000000000ll;
  unsigned u = 3000000000u;
  out[0] = uc;
  out[1] = sc;
  out[2] = us;
  out[3] = ss;
  out[4] = b ? 1 : 0;
  out[5] = (int)(big >> 32);
  out[6] = (int)(neg / 500000000000ll);
  out[7] = (int)(big % 5);
  out[8] = (int)(neg / -1000000ll);
  out[9] = (int)(u / 400000000u);
  out[10] = (u > 2000000000u) ? 1 : 0;
}
