// A constexpr function evaluated at compile time and at run time.
#include <cstdio>

constexpr long long fib(int n) {
    long long a = 0, b = 1;
    for (int i = 0; i < n; ++i) {
        long long t = a + b;
        a = b;
        b = t;
    }
    return a;
}

static_assert(fib(10) == 55);

int main(int argc, char**) {
    constexpr long long at_compile = fib(50);
    std::printf("%lld %lld\n", at_compile, fib(40 + argc));
    return 0;
}
