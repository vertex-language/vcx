// SIMD-group shuffles: by lane, up, down and xor.
//! kernel k
//! grid 256 64
//! buffer uint 256 rand
//! buffer uint 1024 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const uint* in [[buffer(0)]], device uint* out [[buffer(1)]],
              uint i [[thread_position_in_grid]], uint lane [[thread_index_in_simdgroup]]) {
    uint v = in[i];
    out[i * 4 + 0] = simd_shuffle(v, ushort((lane + 5) & 31));
    out[i * 4 + 1] = simd_shuffle_up(v, ushort(3));
    out[i * 4 + 2] = simd_shuffle_down(v, ushort(2));
    out[i * 4 + 3] = simd_shuffle_xor(v, ushort(1));
}
