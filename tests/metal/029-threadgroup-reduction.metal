// A tree reduction in threadgroup memory, one sum per threadgroup.
//! kernel reduce
//! grid 1024 256
//! buffer uint 1024 mod 1000
//! buffer uint 4 zero
#include <metal_stdlib>
using namespace metal;

kernel void reduce(device const uint* in [[buffer(0)]], device uint* out [[buffer(1)]],
                   uint gid [[thread_position_in_grid]], uint lid [[thread_position_in_threadgroup]],
                   uint group [[threadgroup_position_in_grid]]) {
    threadgroup uint partial[256];
    partial[lid] = in[gid];
    threadgroup_barrier(mem_flags::mem_threadgroup);
    for (uint s = 128; s > 0; s >>= 1) {
        if (lid < s)
            partial[lid] += partial[lid + s];
        threadgroup_barrier(mem_flags::mem_threadgroup);
    }
    if (lid == 0)
        out[group] = partial[0];
}
