// The curiously recurring template pattern: static polymorphism.
#include <cstdio>

template <typename Derived>
struct Shape {
    double twice_area() const { return 2 * static_cast<const Derived*>(this)->area(); }
};

struct Square : Shape<Square> {
    double s;
    double area() const { return s * s; }
};

struct Tri : Shape<Tri> {
    double b, h;
    double area() const { return b * h / 2; }
};

int main() {
    Square sq{{}, 3};
    Tri tr{{}, 4, 5};
    std::printf("%g %g\n", sq.twice_area(), tr.twice_area());
    return 0;
}
