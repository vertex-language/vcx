// auto and decltype(auto) deduce types.
#include <cstdio>
#include <type_traits>

int g = 5;
int& ref() { return g; }
auto by_value() { return ref(); }
decltype(auto) by_ref() { return ref(); }

int main() {
    auto a = 1.5f;
    auto b = 'c';
    by_ref() = 9;
    std::printf("%d %d %d %d\n", std::is_same_v<decltype(a), float>, std::is_same_v<decltype(b), char>,
                std::is_same_v<decltype(by_value()), int>, g);
    return 0;
}
