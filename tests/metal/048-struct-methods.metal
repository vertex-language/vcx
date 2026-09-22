// A struct with member functions and operators, used in a kernel.
//! kernel k
//! grid 64 32
//! buffer float 64 rand
//! buffer float 64 zero
#include <metal_stdlib>
using namespace metal;

struct Complex {
    float re, im;
    Complex operator*(Complex o) const { return {re * o.re - im * o.im, re * o.im + im * o.re}; }
    Complex operator+(Complex o) const { return {re + o.re, im + o.im}; }
    float norm2() const { return re * re + im * im; }
};

kernel void k(device const float* in [[buffer(0)]], device float* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    Complex z = {0.0f, 1.0f};
    Complex p = {1.0f, 0.0f};
    for (uint k = 0; k < i % 8; ++k) p = p * z;
    out[i] = (p + Complex{in[i], 1.0f}).norm2();
}
