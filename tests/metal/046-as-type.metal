// as_type reinterprets bits between float and uint.
//! kernel k
//! grid 256 64
//! buffer float 256 rand
//! buffer uint 512 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const float* in [[buffer(0)]], device uint* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    float x = in[i] - 0.5f;
    uint bits = as_type<uint>(x);
    out[i * 2 + 0] = bits;
    out[i * 2 + 1] = as_type<uint>(as_type<float>(bits ^ 0x80000000u) * 2.0f);
}
