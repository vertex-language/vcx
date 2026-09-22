// A function template and a class template, instantiated for two types.
//! kernel k
//! grid 128 32
//! buffer int 128 rand
//! buffer float 128 rand
//! buffer int 128 zero
//! buffer float 128 zero
#include <metal_stdlib>
using namespace metal;

template <typename T>
T clamp_to(T v, T lo, T hi) { return v < lo ? lo : v > hi ? hi : v; }

template <typename T, int N>
struct Window {
    T v[N];
    T sum() const {
        T s = 0;
        for (int k = 0; k < N; ++k) s += v[k];
        return s;
    }
};

kernel void k(device const int* a [[buffer(0)]], device const float* b [[buffer(1)]],
              device int* ai [[buffer(2)]], device float* bf [[buffer(3)]],
              uint i [[thread_position_in_grid]]) {
    Window<int, 3> w = {{a[i] >> 8, 7, -3}};
    ai[i] = clamp_to(w.sum(), -100000, 100000);
    bf[i] = clamp_to(b[i], 0.25f, 0.75f);
}
