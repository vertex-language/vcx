// NaN compares unequal to everything; infinity is ordered.
#include <cstdio>
#include <limits>

int main() {
    double inf = std::numeric_limits<double>::infinity();
    double nan = std::numeric_limits<double>::quiet_NaN();
    std::printf("%d %d %d %d\n", nan == nan, nan != nan, inf > 1e308, -inf < -1e308);
    std::printf("%g %g\n", inf, -inf);
    return 0;
}
