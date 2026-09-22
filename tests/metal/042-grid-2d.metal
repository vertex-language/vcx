// A 2D grid: positions and sizes as uint2, with partial threadgroups in both axes.
//! kernel grid2d
//! grid 37 19 8 4
//! buffer uint 703 zero
#include <metal_stdlib>
using namespace metal;

kernel void grid2d(device uint* out [[buffer(0)]], uint2 pos [[thread_position_in_grid]],
                   uint2 size [[threads_per_grid]], uint2 group [[threadgroup_position_in_grid]]) {
    out[pos.y * size.x + pos.x] = pos.x * 1000 + pos.y + group.x * 100000;
}
