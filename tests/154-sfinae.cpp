// enable_if removes an overload from consideration.
#include <cstdio>
#include <type_traits>

template <typename T, std::enable_if_t<std::is_integral_v<T>, int> = 0>
const char* kind(T) { return "integral"; }

template <typename T, std::enable_if_t<!std::is_integral_v<T>, int> = 0>
const char* kind(T) { return "not integral"; }

int main() {
    std::printf("%s %s %s\n", kind(1), kind('c'), kind(2.0));
    return 0;
}
