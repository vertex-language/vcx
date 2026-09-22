// A struct of parameters bound by reference in constant memory.
//! kernel k
//! grid 300 64
//! buffer float 256 rand
//! buffer float 3 iota
//! buffer float 256 zero
#include <metal_stdlib>
using namespace metal;

struct Params {
    float scale;
    float offset;
    float limit;
};

kernel void k(device const float* in [[buffer(0)]], constant Params& p [[buffer(1)]],
              device float* out [[buffer(2)]], uint i [[thread_position_in_grid]]) {
    if (float(i) >= p.limit * 100.0f)
        return;
    out[i] = in[i] * (p.scale + 3.0f) - p.offset;
}
