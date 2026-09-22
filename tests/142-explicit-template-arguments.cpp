// Template arguments given explicitly, and a return type that cannot be deduced.
#include <cstdio>

template <typename To, typename From>
To convert(From v) { return static_cast<To>(v); }

int main() {
    std::printf("%d %g\n", convert<int>(9.99), convert<double>(7) / 2);
    return 0;
}
