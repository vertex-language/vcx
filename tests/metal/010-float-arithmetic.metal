// Float add, subtract, multiply, and negation.
//! kernel k
//! grid 256 64
//! buffer float 256 rand
//! buffer float 256 rand
//! buffer float 1024 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const float* a [[buffer(0)]], device const float* b [[buffer(1)]],
              device float* out [[buffer(2)]], uint i [[thread_position_in_grid]]) {
    out[i * 4 + 0] = a[i] + b[i];
    out[i * 4 + 1] = a[i] - b[i];
    out[i * 4 + 2] = a[i] * b[i];
    out[i * 4 + 3] = -a[i];
}
