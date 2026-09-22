// Integer functions: clz, ctz, popcount, mulhi, abs, min, max, clamp.
//! kernel bits
//! grid 256 64
//! buffer uint 256 rand
//! buffer uint 2048 zero
#include <metal_stdlib>
using namespace metal;

kernel void bits(device const uint* in [[buffer(0)]], device uint* out [[buffer(1)]],
                 uint i [[thread_position_in_grid]]) {
    uint v = in[i];
    int s = int(v);
    device uint* o = out + i * 8;
    o[0] = clz(v);
    o[1] = ctz(v | 0x80000000u);
    o[2] = popcount(v);
    o[3] = mulhi(v, 2654435761u);
    o[4] = uint(abs(s % 100000));
    o[5] = min(v, 12345u);
    o[6] = max(v >> 4, 1000u);
    o[7] = uint(clamp(s >> 16, -100, 100));
}
