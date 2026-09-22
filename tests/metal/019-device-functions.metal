// Functions called from a kernel, nested and taking pointers.
//! kernel apply
//! grid 256 64
//! buffer float 256 rand
//! buffer float 256 zero
#include <metal_stdlib>
using namespace metal;

static float square(float x) { return x * x; }

float poly(float x) { return 1.0f + x * (2.0f + x * (3.0f + x)); }

void store_pair(device float* out, uint i, float a, float b) { out[i] = a + square(b); }

kernel void apply(device const float* in [[buffer(0)]], device float* out [[buffer(1)]],
                  uint i [[thread_position_in_grid]]) {
    store_pair(out, i, poly(in[i]), in[i] - 0.5f);
}
