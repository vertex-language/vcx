// Atomic min, max, and, or, xor and sub on device memory.
//! kernel k
//! grid 1024 256
//! buffer int 1024 rand
//! buffer int 6 const 0
#include <metal_stdlib>
using namespace metal;

kernel void k(device const int* in [[buffer(0)]], device atomic_int* acc [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    int v = in[i];
    atomic_fetch_min_explicit(&acc[0], v, memory_order_relaxed);
    atomic_fetch_max_explicit(&acc[1], v, memory_order_relaxed);
    atomic_fetch_or_explicit(&acc[2], 1 << (i % 31), memory_order_relaxed);
    atomic_fetch_and_explicit(&acc[3], v | 0x7ffffff0, memory_order_relaxed);
    atomic_fetch_xor_explicit(&acc[4], v & 0xff, memory_order_relaxed);
    atomic_fetch_sub_explicit(&acc[5], int(i & 3), memory_order_relaxed);
}
