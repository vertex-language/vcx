// SIMD-group reductions: sum, min, max, and, or.
//! kernel k
//! grid 256 64
//! buffer uint 256 mod 1000
//! buffer uint 1280 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const uint* in [[buffer(0)]], device uint* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    uint v = in[i];
    out[i * 5 + 0] = simd_sum(v);
    out[i * 5 + 1] = simd_min(v);
    out[i * 5 + 2] = simd_max(v);
    out[i * 5 + 3] = simd_and(v | 0xf000u);
    out[i * 5 + 4] = simd_or(v & 0x0f0u);
}
