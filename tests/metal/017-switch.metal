// A switch with fallthrough, a default, and a return from a case.
//! kernel k
//! grid 400 64
//! buffer int 400 rand
//! buffer int 400 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const int* in [[buffer(0)]], device int* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    int v = in[i];
    int r = 0;
    switch (v & 7) {
    case 0:
        r = v >> 3;
        break;
    case 1:
        r += 5;
        [[fallthrough]];
    case 2:
        r -= v;
        break;
    case 5:
        out[i] = 12345;
        return;
    default:
        r = v / 7 + v % 5;
    }
    out[i] = r;
}
