// A function template, its arguments deduced.
#include <cstdio>

template <typename T>
T largest(T a, T b) { return a > b ? a : b; }

int main() {
    std::printf("%d %g %c\n", largest(3, 9), largest(2.5, -1.0), largest('a', 'z'));
    return 0;
}
