// if constexpr discards the branch not taken.
#include <cstdio>
#include <type_traits>

template <typename T>
const char* describe(T v) {
    if constexpr (std::is_integral_v<T>) {
        return v % 2 ? "odd integer" : "even integer";
    } else if constexpr (std::is_floating_point_v<T>) {
        return v < 0 ? "negative real" : "real";
    } else {
        return "other";
    }
}

int main() {
    std::printf("%s | %s | %s\n", describe(3), describe(-1.5), describe("x"));
    return 0;
}
