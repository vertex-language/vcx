// Structs passed and returned by value.
#include <cstdio>

struct Vec {
    double x, y;
};

Vec add(Vec a, Vec b) { return {a.x + b.x, a.y + b.y}; }
Vec scale(Vec v, double k) { return {v.x * k, v.y * k}; }

int main() {
    Vec v = scale(add({1, 2}, {3, 4}), 0.5);
    std::printf("%g %g\n", v.x, v.y);
    return 0;
}
