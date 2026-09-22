// A variable template.
#include <cstdio>

template <typename T>
constexpr T pi = T(3.14159265358979323846L);

template <int N>
constexpr int squares = N * N + squares<N - 1>;
template <>
constexpr int squares<0> = 0;

int main() {
    std::printf("%d %.6f %d\n", (int)pi<int>, pi<double>, squares<5>);
    return 0;
}
