// Device atomics: a histogram, many threads to a bin.
//! kernel histogram
//! grid 4096 256
//! buffer uint 4096 mod 16
//! buffer uint 16 zero
#include <metal_stdlib>
using namespace metal;

kernel void histogram(device const uint* in [[buffer(0)]], device atomic_uint* bins [[buffer(1)]],
                      uint i [[thread_position_in_grid]]) {
    atomic_fetch_add_explicit(&bins[in[i]], 1u, memory_order_relaxed);
}
