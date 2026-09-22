// A lambda used in a constant expression.
#include <cstdio>

int main() {
    constexpr auto square = [](int v) { return v * v; };
    constexpr int table[] = {square(1), square(2), square(3), square(4)};
    static_assert(square(12) == 144);
    int sum = 0;
    for (int v : table) sum += v;
    std::printf("%d\n", sum);
    return 0;
}
