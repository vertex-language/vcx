// simd_broadcast_first, simd_ballot, simd_any and simd_all.
//! kernel k
//! grid 256 64
//! buffer uint 256 mod 8
//! buffer uint 1024 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const uint* in [[buffer(0)]], device uint* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    uint v = in[i];
    out[i * 4 + 0] = simd_broadcast_first(v);
    out[i * 4 + 1] = uint(ulong(simd_ballot(v == 3)) & 0xffffffffu);
    out[i * 4 + 2] = simd_any(v == 7) ? 1u : 0u;
    out[i * 4 + 3] = simd_all(v < 8) ? 1u : 0u;
}
