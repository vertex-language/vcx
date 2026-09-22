// MSL's vector types: arithmetic, swizzles, constructors and dot products.
//! kernel k
//! grid 64 32
//! buffer float 256 rand
//! buffer float 256 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device const float4* in [[buffer(0)]], device float4* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    float4 v = in[i];
    float3 a = v.xyz * 2.0f + v.w;
    float2 b = v.wx - v.yz;
    out[i] = float4(a.zyx, dot(b, b)) + float4(1.0f);
}
