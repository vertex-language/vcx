// The SIMD-group built-ins.
//! kernel k
//! grid 256 128
//! buffer uint 1024 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device uint* out [[buffer(0)]], uint i [[thread_position_in_grid]],
              uint lane [[thread_index_in_simdgroup]], uint sg [[simdgroup_index_in_threadgroup]],
              uint width [[threads_per_simdgroup]], uint count [[simdgroups_per_threadgroup]]) {
    out[i * 4 + 0] = lane;
    out[i * 4 + 1] = sg;
    out[i * 4 + 2] = width;
    out[i * 4 + 3] = count;
}
