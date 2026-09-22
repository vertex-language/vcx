// Argument-dependent lookup finds a function in its argument's namespace.
#include <cstdio>

namespace geom {
struct Vec {
    int x, y;
};
int dot(Vec a, Vec b) { return a.x * b.x + a.y * b.y; }
Vec operator+(Vec a, Vec b) { return {a.x + b.x, a.y + b.y}; }
void swap(Vec& a, Vec& b) {
    std::printf("geom::swap\n");
    Vec t = a;
    a = b;
    b = t;
}
}  // namespace geom

template <typename T>
void generic_swap(T& a, T& b) {
    swap(a, b);  // found by ADL
}

int main() {
    geom::Vec a{1, 2}, b{3, 4};
    geom::Vec c = a + b;
    generic_swap(a, b);
    std::printf("%d %d %d\n", dot(a, b), c.x, a.x);
    return 0;
}
