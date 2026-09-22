// while and do-while loops.
//! kernel k
//! grid 128 32
//! buffer uint 128 mod 1000
//! buffer uint 256 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const uint* a [[buffer(0)]], device uint* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    uint n = a[i] + 1, steps = 0;
    while (n != 1) {
        n = (n & 1) ? 3 * n + 1 : n / 2;
        ++steps;
    }
    uint digits = 0, v = a[i] * 97;
    do {
        ++digits;
        v /= 10;
    } while (v != 0);
    out[i * 2 + 0] = steps;
    out[i * 2 + 1] = digits;
}
