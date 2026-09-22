// index_sequence to walk a tuple, and std::apply to call with its elements.
#include <cstdio>
#include <tuple>
#include <utility>

template <typename Tuple, std::size_t... I>
long sum_impl(const Tuple& t, std::index_sequence<I...>) {
    return (0L + ... + std::get<I>(t));
}

template <typename... Ts>
long sum(const std::tuple<Ts...>& t) {
    return sum_impl(t, std::index_sequence_for<Ts...>{});
}

int volume(int a, int b, int c) { return a * b * c; }

int main() {
    auto t = std::make_tuple(1, 2L, 3, 4L);
    std::printf("%ld %d\n", sum(t), std::apply(volume, std::make_tuple(2, 3, 7)));
    return 0;
}
