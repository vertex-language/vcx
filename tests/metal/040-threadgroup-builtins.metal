// The threadgroup built-ins, over a grid whose last threadgroup is partial.
//! kernel k
//! grid 200 64
//! buffer uint 800 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device uint* out [[buffer(0)]], uint i [[thread_position_in_grid]],
              uint t [[thread_index_in_threadgroup]], uint g [[threadgroup_position_in_grid]],
              uint size [[threads_per_threadgroup]], uint groups [[threadgroups_per_grid]]) {
    out[i * 4 + 0] = t;
    out[i * 4 + 1] = g;
    out[i * 4 + 2] = size;
    out[i * 4 + 3] = groups;
}
