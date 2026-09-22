// Float functions whose results are exact, each on its own.
//! kernel k
//! grid 256 64
//! buffer float 256 rand
//! buffer float 3072 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const float* in [[buffer(0)]], device float* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    float x = in[i] * 10.0f - 5.0f;
    device float* e = out + i * 12;
    e[0] = sqrt(fabs(x));
    e[1] = floor(x);
    e[2] = ceil(x);
    e[3] = trunc(x);
    e[4] = rint(x);
    e[5] = fma(x, x, 1.0f);
    e[6] = fmin(x, 0.5f);
    e[7] = fmax(x, -0.5f);
    e[8] = clamp(x, -1.0f, 1.0f);
    e[9] = copysign(2.0f, x);
    e[10] = abs(x);
    e[11] = saturate(x);
}
