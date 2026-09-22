// Unsigned division and remainder, by variables and by constants.
//! kernel k
//! grid 256 64
//! buffer uint 256 rand
//! buffer uint 256 mod 5000
//! buffer uint 1024 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const uint* a [[buffer(0)]], device const uint* b [[buffer(1)]],
              device uint* out [[buffer(2)]], uint i [[thread_position_in_grid]]) {
    uint d = b[i] + 1;
    out[i * 4 + 0] = a[i] / d;
    out[i * 4 + 1] = a[i] % d;
    out[i * 4 + 2] = a[i] / 7u;
    out[i * 4 + 3] = a[i] % 1000u;
}
