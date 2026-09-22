// break and continue in nested loops.
//! kernel k
//! grid 128 32
//! buffer uint 128 mod 50
//! buffer uint 128 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const uint* a [[buffer(0)]], device uint* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    uint sum = 0;
    for (uint x = 0; x < 20; ++x) {
        if (x % 3 == 0)
            continue;
        for (uint y = 0; y < 20; ++y) {
            if (y > a[i])
                break;
            sum += x * y;
        }
        if (sum > 5000)
            break;
    }
    out[i] = sum;
}
