// Signed division and remainder truncate toward zero.
//! kernel k
//! grid 256 64
//! buffer int 256 rand
//! buffer int 256 mod 1000
//! buffer int 512 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const int* a [[buffer(0)]], device const int* b [[buffer(1)]],
              device int* out [[buffer(2)]], uint i [[thread_position_in_grid]]) {
    int d = b[i] - 500;
    if (d == 0 || d == -1) d = 7;  // no zero divisor, and no INT_MIN / -1
    out[i * 2 + 0] = a[i] / d;
    out[i * 2 + 1] = a[i] % d;
}
