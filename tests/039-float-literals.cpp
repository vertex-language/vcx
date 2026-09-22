// Floating literals: exponents, hex floats, suffixes.
#include <cstdio>

int main() {
    std::printf("%g %g %g %a\n", 1e3, 2.5e-3, 0x1.8p1, 1.0);
    std::printf("%zu %zu\n", sizeof 1.0f, sizeof 1.0);
    return 0;
}
