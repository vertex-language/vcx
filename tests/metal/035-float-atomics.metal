// Atomic float add, with values whose sum is exact in any order.
//! kernel k
//! grid 2048 256
//! buffer float 1 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device atomic_float* sum [[buffer(0)]], uint i [[thread_position_in_grid]]) {
    atomic_fetch_add_explicit(sum, float(i % 4) * 0.25f, memory_order_relaxed);
}
