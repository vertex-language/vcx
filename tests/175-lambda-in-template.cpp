// Lambdas passed to templates, and a lambda returning a lambda.
#include <cstdio>

template <typename F>
int apply_n(F f, int v, int n) {
    while (n--) v = f(v);
    return v;
}

auto adder(int k) {
    return [k](int v) { return v + k; };
}

int main() {
    std::printf("%d %d\n", apply_n([](int v) { return v * 2; }, 1, 10), apply_n(adder(7), 0, 6));
    return 0;
}
