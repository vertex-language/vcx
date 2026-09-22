// if, else if, else, and an early return.
//! kernel k
//! grid 300 64
//! buffer int 256 rand
//! buffer int 256 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const int* a [[buffer(0)]], device int* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    if (i >= 256)
        return;
    int v = a[i];
    int r;
    if (v < -1000000)
        r = 1;
    else if (v < 0)
        r = 2;
    else if (v == 0)
        r = 3;
    else
        r = 4;
    out[i] = r;
}
