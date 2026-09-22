// Signed and unsigned comparisons, stored as 0 or 1.
//! kernel k
//! grid 256 64
//! buffer int 256 rand
//! buffer int 256 rand
//! buffer uint 2048 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const int* a [[buffer(0)]], device const int* b [[buffer(1)]],
              device uint* out [[buffer(2)]], uint i [[thread_position_in_grid]]) {
    int x = a[i], y = b[i];
    uint ux = uint(x), uy = uint(y);
    device uint* o = out + i * 8;
    o[0] = x < y;
    o[1] = x <= y;
    o[2] = x > y;
    o[3] = x == y;
    o[4] = x != y;
    o[5] = ux < uy;
    o[6] = ux >= uy;
    o[7] = x == x;
}
