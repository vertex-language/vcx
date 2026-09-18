#include <hip/hip_runtime.h>
// The device math library's polynomials: exp, log, sin, cos, pow, atan2,
// in float and double, each scaled to an integer with a margin the
// library's few-ulp accuracy is well inside.
// grid: 1
// block: 1
// expect: 271828 69315 84147 -41615 15574 8000 9164 3141593 2718282 -958924 1000000
__global__ void test(int *out) {
  out[0] = (int)(expf(1.0f) * 100000.0f + 0.5f);
  out[1] = (int)(logf(2.0f) * 100000.0f + 0.5f);
  out[2] = (int)(sinf(1.0f) * 100000.0f + 0.5f);
  out[3] = (int)(cosf(2.0f) * 100000.0f - 0.5f);
  out[4] = (int)(tanf(1.0f) * 10000.0f + 0.5f);
  out[5] = (int)(powf(20.0f, 3.0f) + 0.5f);
  out[6] = (int)(exp2f(3.196f) * 1000.0f + 0.5f);
  out[7] = (int)(atan2(1.0, 0.0) * 2000000.0 + 0.5);
  out[8] = (int)(exp(1.0) * 1000000.0 + 0.5);
  out[9] = (int)(sin(5.0) * 1000000.0 - 0.5);
  out[10] = (int)(pow(10.0, 6.0) + 0.5);
}
