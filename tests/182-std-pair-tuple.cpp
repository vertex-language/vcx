// std::pair and std::tuple, with get and structured bindings.
#include <cstdio>
#include <tuple>
#include <utility>

std::tuple<int, double, char> make() { return {7, 2.5, 'q'}; }

int main() {
    std::pair<int, int> p{3, 4};
    auto [i, d, c] = make();
    auto t = std::make_tuple(1, 2, 3);
    std::printf("%d %d %d %g %c %d %zu\n", p.first, p.second, i, d, c, std::get<2>(t),
                std::tuple_size_v<decltype(t)>);
    return 0;
}
