// An array in a thread's own memory, indexed by data.
//! kernel k
//! grid 128 32
//! buffer uint 128 rand
//! buffer uint 128 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const uint* in [[buffer(0)]], device uint* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    uint digits[10] = {};
    uint v = in[i];
    for (int k = 0; k < 10; ++k) {
        digits[k] = v % 10;
        v /= 10;
    }
    uint r = 0;
    for (int k = 0; k < 10; ++k) r = r * 10 + digits[k];
    out[i] = r ^ digits[in[i] % 10];
}
