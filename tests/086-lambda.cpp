// A lambda with no captures, called directly and stored.
#include <cstdio>

int main() {
    auto square = [](int v) { return v * v; };
    std::printf("%d %d\n", square(9), [](int a, int b) { return a - b; }(10, 4));
    return 0;
}
