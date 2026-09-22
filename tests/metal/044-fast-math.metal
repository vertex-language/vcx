// The fast transcendentals, within the ulps two compilers' forms may differ by.
//! kernel k
//! grid 256 64
//! buffer float 256 rand
//! buffer float 1536 zero
//! ulp 4
#include <metal_stdlib>
using namespace metal;

kernel void k(device const float* in [[buffer(0)]], device float* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    float x = in[i] * 4.0f - 2.0f;
    device float* f = out + i * 6;
    f[0] = exp2(x);
    f[1] = log2(fabs(x) + 1.0f);
    f[2] = sin(x);
    f[3] = cos(x);
    f[4] = rsqrt(fabs(x) + 0.5f);
    f[5] = exp(x * 0.5f);
}
