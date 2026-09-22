// Namespaces, constexpr functions, and function objects in device code (MSL has no lambdas).
//! kernel k
//! grid 128 32
//! buffer int 128 rand
//! buffer int 256 zero
#include <metal_stdlib>
using namespace metal;

namespace util {
constexpr int cube(int v) { return v * v * v; }
namespace detail {
inline int fold(int v) { return v ^ (v >> 7); }
}
}  // namespace util

struct Shift {
    int base;
    int operator()(int v) const { return (v >> 20) + base; }
};

template <typename F>
int twice_with(F f, int v) { return f(f(v)); }

kernel void k(device const int* in [[buffer(0)]], device int* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    Shift shift{util::cube(3)};
    out[i * 2 + 0] = twice_with(shift, in[i]);
    out[i * 2 + 1] = util::detail::fold(in[i]);
}
