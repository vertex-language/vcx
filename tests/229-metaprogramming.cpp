// Compile-time computation with templates: a type list and a recursive template.
#include <cstdio>
#include <type_traits>

template <typename... Ts>
struct List {};

template <typename L>
struct Length;
template <typename... Ts>
struct Length<List<Ts...>> : std::integral_constant<int, sizeof...(Ts)> {};

template <typename T, typename L>
struct IndexOf;
template <typename T, typename... Rest>
struct IndexOf<T, List<T, Rest...>> : std::integral_constant<int, 0> {};
template <typename T, typename U, typename... Rest>
struct IndexOf<T, List<U, Rest...>> : std::integral_constant<int, 1 + IndexOf<T, List<Rest...>>::value> {};

template <unsigned N>
struct Binary : std::integral_constant<unsigned, Binary<N / 10>::value * 2 + N % 10> {};
template <>
struct Binary<0> : std::integral_constant<unsigned, 0> {};

using Types = List<char, short, int, long, double>;

int main() {
    std::printf("%d %d %d %u\n", Length<Types>::value, IndexOf<int, Types>::value, IndexOf<double, Types>::value,
                Binary<101101>::value);
    return 0;
}
