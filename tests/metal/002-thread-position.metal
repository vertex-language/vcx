// Each thread writes its position in the grid.
//! kernel k
//! grid 256 32
//! buffer uint 256 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device uint* out [[buffer(0)]], uint i [[thread_position_in_grid]]) {
    out[i] = i * 3 + 1;
}
