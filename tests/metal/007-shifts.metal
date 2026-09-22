// Left shifts, logical right shifts, and arithmetic right shifts.
//! kernel k
//! grid 256 64
//! buffer uint 256 rand
//! buffer uint 768 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const uint* a [[buffer(0)]], device uint* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    uint s = i & 31;
    out[i * 3 + 0] = a[i] << s;
    out[i * 3 + 1] = a[i] >> s;
    out[i * 3 + 2] = uint(int(a[i]) >> s);
}
