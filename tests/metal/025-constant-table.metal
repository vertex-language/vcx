// A table in constant memory at program scope.
//! kernel lookup
//! grid 256 64
//! buffer uint 256 mod 8
//! buffer float 256 zero
#include <metal_stdlib>
using namespace metal;

constant float table[8] = {0.5f, 1.5f, -2.0f, 3.25f, 8.0f, -0.125f, 100.0f, 7.0f};
constant int offsets[] = {3, 1, 4, 1, 5, 9, 2, 6};

kernel void lookup(device const uint* index [[buffer(0)]], device float* out [[buffer(1)]],
                   uint i [[thread_position_in_grid]]) {
    out[i] = table[index[i]] * float(i) + float(offsets[(index[i] + 3) & 7]);
}
