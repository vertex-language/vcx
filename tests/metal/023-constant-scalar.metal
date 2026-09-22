// A scalar bound by reference, and constant pointers.
//! kernel saxpy
//! grid 512 128
//! buffer float 512 zero
//! buffer float 1 const 2.5
//! buffer float 512 rand
//! buffer float 512 rand
#include <metal_stdlib>
using namespace metal;

kernel void saxpy(device float* out [[buffer(0)]], constant float& a [[buffer(1)]],
                  constant float* x [[buffer(2)]], constant float* y [[buffer(3)]],
                  uint i [[thread_position_in_grid]]) {
    out[i] = a * x[i] + y[i];
}
