// User-defined literals for integers, floats and strings.
#include <cstddef>
#include <cstdio>

constexpr unsigned long long operator""_kb(unsigned long long v) { return v * 1024; }
constexpr long double operator""_deg(long double v) { return v * 3.14159265358979L / 180; }
constexpr std::size_t operator""_len(const char*, std::size_t n) { return n; }

int main() {
    std::printf("%llu %.4Lf %zu\n", 4_kb, 180.0_deg, "hello"_len);
    return 0;
}
