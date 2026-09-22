// auto non-type template parameters and class-type ones.
#include <cstdio>

template <auto V>
constexpr auto value = V;

struct Color {
    int r, g, b;
};

template <Color C>
int luminance() { return (C.r * 3 + C.g * 6 + C.b) / 10; }

template <int... Ns>
constexpr int sum = (Ns + ... + 0);

int main() {
    std::printf("%d %c %d %d\n", value<42>, value<'z'>, luminance<Color{100, 200, 50}>(), sum<1, 2, 3, 4>);
    return 0;
}
