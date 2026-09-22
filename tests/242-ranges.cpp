// Ranges: views composed with pipes, and range algorithms.
#include <algorithm>
#include <cstdio>
#include <ranges>
#include <vector>

int main() {
    std::vector<int> v{5, 2, 8, 1, 9, 3, 7};
    auto odd_squares = v | std::views::filter([](int x) { return x % 2 == 1; }) |
                         std::views::transform([](int x) { return x * x; }) | std::views::take(3);
    for (int x : odd_squares) std::printf("%d ", x);
    std::printf("| ");
    for (int x : std::views::iota(1, 6) | std::views::reverse) std::printf("%d", x);
    std::ranges::sort(v);
    std::printf(" | %d %d\n", v.front(), (int)std::ranges::count_if(v, [](int x) { return x > 4; }));
    return 0;
}
