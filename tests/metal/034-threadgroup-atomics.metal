// Atomics on threadgroup memory: each group counts, one thread adds its count.
//! kernel count
//! grid 2048 256
//! buffer uint 2048 mod 4
//! buffer uint 4 zero
#include <metal_stdlib>
using namespace metal;

kernel void count(device const uint* in [[buffer(0)]], device atomic_uint* total [[buffer(1)]],
                  uint i [[thread_position_in_grid]], uint lid [[thread_position_in_threadgroup]]) {
    threadgroup atomic_uint local[4];
    if (lid < 4)
        atomic_store_explicit(&local[lid], 0u, memory_order_relaxed);
    threadgroup_barrier(mem_flags::mem_threadgroup);
    atomic_fetch_add_explicit(&local[in[i]], 1u, memory_order_relaxed);
    threadgroup_barrier(mem_flags::mem_threadgroup);
    if (lid < 4)
        atomic_fetch_add_explicit(&total[lid], atomic_load_explicit(&local[lid], memory_order_relaxed),
                                  memory_order_relaxed);
}
