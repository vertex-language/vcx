// Conversions between int, uint and float.
//! kernel k
//! grid 256 64
//! buffer float 256 rand
//! buffer int 256 rand
//! buffer int 256 zero
//! buffer float 512 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const float* f [[buffer(0)]], device const int* n [[buffer(1)]],
              device int* ints [[buffer(2)]], device float* floats [[buffer(3)]],
              uint i [[thread_position_in_grid]]) {
    ints[i] = int(f[i] * 2000.0f - 1000.0f);
    floats[i * 2 + 0] = float(n[i]);
    floats[i * 2 + 1] = float(uint(n[i]));
}
