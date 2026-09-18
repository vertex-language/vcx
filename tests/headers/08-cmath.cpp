// Does <cmath> compile, and do its classifications and functions answer?
//
// The header's classifications go through __builtin_bit_cast in MSVC's
// STL -- a float's bits read as an integer -- which is what this file is
// here to keep compiling. The program adds up what the functions return
// for a few constants, scaled to integers.

#include <cmath>

int main() {
    double zero = 0.0;                             // cl refuses a literal 0.0 / 0.0
    double d = std::sqrt(16.0);                    // 4
    float f = std::fabs(-2.5f);                    // 2.5
    int nan = std::isnan(zero / zero) ? 1 : 0;     // 1
    int inf = std::isinf(1.0 / zero) ? 1 : 0;      // 1
    int fin = std::isfinite(1.5) ? 1 : 0;          // 1
    int sgn = std::signbit(-0.0) ? 1 : 0;          // 1
    return (int)d + (int)(f * 2) + nan + inf + fin + sgn + (int)std::floor(2.7) + (int)std::ceil(2.1);  // 4+5+1+1+1+1+2+3 = 18
}
