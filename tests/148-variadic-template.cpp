// A variadic template, recursing over its pack.
#include <cstdio>

int sum() { return 0; }

template <typename T, typename... Rest>
int sum(T first, Rest... rest) { return first + sum(rest...); }

template <typename... Ts>
int count(Ts...) { return sizeof...(Ts); }

int main() {
    std::printf("%d %d %d\n", sum(1, 2, 3, 4, 5), count(), count('a', 2.0, "x"));
    return 0;
}
