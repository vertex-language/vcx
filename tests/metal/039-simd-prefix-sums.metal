// SIMD-group prefix sums, inclusive and exclusive.
//! kernel k
//! grid 256 64
//! buffer uint 256 mod 100
//! buffer uint 512 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const uint* in [[buffer(0)]], device uint* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    uint v = in[i];
    out[i * 2 + 0] = simd_prefix_inclusive_sum(v);
    out[i * 2 + 1] = simd_prefix_exclusive_sum(v);
}
