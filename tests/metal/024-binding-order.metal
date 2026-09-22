// Buffers bound out of parameter order, and one left for the compiler to number.
//! kernel k
//! grid 64 64
//! buffer uint 64 zero
//! buffer uint 64 iota
//! buffer uint 64 const 5
//! buffer uint 64 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const uint* scale [[buffer(2)]], device uint* out [[buffer(0)]],
              device const uint* in [[buffer(1)]], device uint* extra,
              uint i [[thread_position_in_grid]]) {
    out[i] = in[i] * scale[i];
    extra[i] = in[i] + 100;
}
