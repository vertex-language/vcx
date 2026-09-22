// The conditional operator and bool values.
//! kernel k
//! grid 256 64
//! buffer int 256 rand
//! buffer int 512 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const int* a [[buffer(0)]], device int* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    int v = a[i];
    bool negative = v < 0;
    bool even = (v & 1) == 0;
    out[i * 2 + 0] = negative ? -v : v;
    out[i * 2 + 1] = (negative && even) ? 1 : (negative || even) ? 2 : 3;
}
