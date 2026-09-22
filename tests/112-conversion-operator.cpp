// Conversion operators and converting constructors.
#include <cstdio>

struct Meters {
    double v;
    Meters(double x) : v(x) {}
    explicit operator int() const { return (int)v; }
    operator bool() const { return v != 0; }
};

double twice(Meters m) { return m.v * 2; }

int main() {
    Meters m = 3.75;
    std::printf("%g %d %d %d\n", twice(1.5), static_cast<int>(m), bool(m), bool(Meters(0)));
    return 0;
}
