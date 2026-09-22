// Two input buffers and one output, over a grid the threadgroups do not divide.
//! kernel vector_add
//! grid 1000 64
//! buffer float 1000 rand
//! buffer float 1000 iota
//! buffer float 1000 zero
#include <metal_stdlib>
using namespace metal;

kernel void vector_add(device const float* a [[buffer(0)]],
                       device const float* b [[buffer(1)]],
                       device float* out [[buffer(2)]],
                       uint i [[thread_position_in_grid]]) {
    out[i] = a[i] + b[i];
}
