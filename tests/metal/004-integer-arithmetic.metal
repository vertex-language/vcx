// Integer add, subtract and multiply, wrapping as unsigned.
//! kernel k
//! grid 256 64
//! buffer uint 256 rand
//! buffer uint 256 rand
//! buffer uint 768 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const uint* a [[buffer(0)]], device const uint* b [[buffer(1)]],
              device uint* out [[buffer(2)]], uint i [[thread_position_in_grid]]) {
    out[i * 3 + 0] = a[i] + b[i];
    out[i * 3 + 1] = a[i] - b[i];
    out[i * 3 + 2] = a[i] * b[i];
}
