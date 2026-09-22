// A threadgroup array and a barrier: reversing each threadgroup's slice.
//! kernel k
//! grid 512 128
//! buffer uint 512 iota
#include <metal_stdlib>
using namespace metal;

kernel void k(device uint* v [[buffer(0)]], uint i [[thread_position_in_grid]],
              uint t [[thread_position_in_threadgroup]]) {
    threadgroup uint tile[128];
    tile[t] = v[i];
    threadgroup_barrier(mem_flags::mem_threadgroup);
    v[i] = tile[127 - t];
}
