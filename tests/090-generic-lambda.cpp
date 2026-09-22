// A generic lambda, instantiated for two types.
#include <cstdio>

int main() {
    auto twice = [](auto v) { return v + v; };
    std::printf("%d %g\n", twice(21), twice(1.25));
    return 0;
}
