// The smallest kernel: every thread stores a constant.
//! kernel k
//! grid 64 64
//! buffer uint 64 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device uint* out [[buffer(0)]], uint i [[thread_position_in_grid]]) {
    out[i] = 7;
}
