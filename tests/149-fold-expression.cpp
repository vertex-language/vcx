// Fold expressions over a parameter pack.
#include <cstdio>

template <typename... Ts>
auto product(Ts... v) { return (v * ... * 1); }

template <typename... Ts>
bool all(Ts... v) { return (... && v); }

template <typename... Ts>
void print(Ts... v) {
    ((std::printf("%d,", v)), ...);
    std::printf("\n");
}

int main() {
    std::printf("%d %d %d\n", product(2, 3, 7), all(true, 1, 5), all(true, 0));
    print(4, 5, 6);
    return 0;
}
