// explicit constructors, and explicit(bool) chosen by a condition.
#include <cstdio>
#include <type_traits>

struct Meters {
    explicit Meters(double v) : v(v) {}
    double v;
};

template <typename T>
struct Wrapper {
    template <typename U>
    explicit(!std::is_convertible_v<U, T>) Wrapper(U u) : v(static_cast<T>(u)) {}
    T v;
};

double length(Meters m) { return m.v; }
int get(Wrapper<int> w) { return w.v; }

int main() {
    Meters m(2.5);
    Wrapper<int> a = 7;            // int converts implicitly
    Wrapper<int> b(Meters(3).v);   // double converts too
    std::printf("%g %d %d %d\n", length(m), get(a), b.v, get(40));
    std::printf("%d %d\n", std::is_convertible_v<double, Meters>, std::is_convertible_v<int, Wrapper<int>>);
    return 0;
}
