// Float division, which fast math computes by a reciprocal on both compilers.
//! kernel k
//! grid 256 64
//! buffer float 256 rand
//! buffer float 256 rand
//! buffer float 256 zero
//! ulp 2
#include <metal_stdlib>
using namespace metal;

kernel void k(device const float* a [[buffer(0)]], device const float* b [[buffer(1)]],
              device float* out [[buffer(2)]], uint i [[thread_position_in_grid]]) {
    out[i] = a[i] / (b[i] + 0.5f);
}
