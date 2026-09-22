// A for loop whose trip count comes from the data.
//! kernel k
//! grid 256 32
//! buffer uint 256 mod 100
//! buffer uint 256 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const uint* n [[buffer(0)]], device uint* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    uint sum = 0;
    for (uint k = 0; k <= n[i]; ++k)
        sum += k * k + (k ^ i);
    out[i] = sum;
}
