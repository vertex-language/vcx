// Arithmetic and comparison operators on a class.
#include <cstdio>

struct Frac {
    int n, d;
};

Frac operator+(Frac a, Frac b) { return {a.n * b.d + b.n * a.d, a.d * b.d}; }
Frac operator*(Frac a, Frac b) { return {a.n * b.n, a.d * b.d}; }
bool operator==(Frac a, Frac b) { return a.n * b.d == b.n * a.d; }

int main() {
    Frac h{1, 2}, t{1, 3};
    Frac s = h + t;
    Frac p = h * t;
    std::printf("%d/%d %d/%d %d\n", s.n, s.d, p.n, p.d, Frac{2, 4} == h);
    return 0;
}
