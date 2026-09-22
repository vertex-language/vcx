// A member function template, and one called with explicit arguments.
#include <cstdio>

struct Converter {
    double scale;
    template <typename T>
    T apply(T v) const { return static_cast<T>(v * scale); }
    template <typename To, typename From>
    To as(From v) const { return static_cast<To>(v * scale); }
};

int main() {
    Converter c{2.5};
    std::printf("%d %g %d\n", c.apply(4), c.apply(1.0), c.template as<int>(3.3));
    return 0;
}
