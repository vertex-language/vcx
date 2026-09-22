// A program: a fixed-point Mandelbrot set on a 2D grid.
//! kernel mandel
//! grid 64 24 16 8
//! buffer uint 1536 zero
#include <metal_stdlib>
using namespace metal;

constant int MAXIT = 200;

// Fixed point with 16 fractional bits, so every step is exact integer arithmetic.
kernel void mandel(device uint* out [[buffer(0)]], uint2 pos [[thread_position_in_grid]]) {
    const int one = 1 << 16;
    int cr = -2 * one + (3 * one / 64) * int(pos.x);
    int ci = -one + (2 * one / 24) * int(pos.y);
    int zr = 0, zi = 0, it = 0;
    while (it < MAXIT) {
        int zr2 = (zr >> 8) * (zr >> 8), zi2 = (zi >> 8) * (zi >> 8);
        if (zr2 + zi2 > 4 * one)
            break;
        zi = 2 * ((zr >> 8) * (zi >> 8)) + ci;
        zr = zr2 - zi2 + cr;
        ++it;
    }
    out[pos.y * 64 + pos.x] = uint(it);
}
