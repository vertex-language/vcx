// Class template argument deduction, with a deduction guide.
#include <cstdio>

template <typename T>
struct Wrapper {
    T v;
    Wrapper(T x) : v(x) {}
};

template <typename T, typename U>
struct Pair {
    T a;
    U b;
};
template <typename T, typename U>
Pair(T, U) -> Pair<T, U>;

int main() {
    Wrapper w(2.5);
    Pair p{1, 'x'};
    std::printf("%g %d %c %zu\n", w.v, p.a, p.b, sizeof(p));
    return 0;
}
