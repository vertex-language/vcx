// std::array: size, indexing, iteration, comparison.
#include <array>
#include <cstdio>

int main() {
    std::array<int, 5> a{5, 3, 9, 1, 7};
    int sum = 0;
    for (int v : a) sum += v;
    std::array<int, 5> b = a;
    b[0] = 0;
    std::printf("%zu %d %d %d %d\n", a.size(), sum, a.front() + a.back(), a == b, b < a);
    return 0;
}
