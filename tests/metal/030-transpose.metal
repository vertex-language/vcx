// A matrix transpose through a threadgroup tile, on a 2D grid.
//! kernel transpose
//! grid 32 32 8 8
//! buffer uint 1024 iota
//! buffer uint 1024 zero
#include <metal_stdlib>
using namespace metal;

kernel void transpose(device const uint* in [[buffer(0)]], device uint* out [[buffer(1)]],
                      uint2 pos [[thread_position_in_grid]], uint2 lid [[thread_position_in_threadgroup]],
                      uint2 group [[threadgroup_position_in_grid]]) {
    threadgroup uint tile[8][9];
    tile[lid.y][lid.x] = in[pos.y * 32 + pos.x];
    threadgroup_barrier(mem_flags::mem_threadgroup);
    uint ox = group.y * 8 + lid.x, oy = group.x * 8 + lid.y;
    out[oy * 32 + ox] = tile[lid.x][lid.y];
}
