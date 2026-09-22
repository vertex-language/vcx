// <cmath>: the functions whose results are exact or correctly rounded.
#include <cmath>
#include <cstdio>

int main() {
    std::printf("%g %g %g %g %g\n", std::sqrt(2.0) * std::sqrt(2.0) - 2.0 < 1e-15 ? 1.0 : 0.0,
                std::floor(-2.5), std::ceil(-2.5), std::round(2.5), std::trunc(-2.7));
    std::printf("%g %g %g %d\n", std::fabs(-3.25), std::fmod(10.5, 3.0), std::hypot(3.0, 4.0),
                std::isnan(std::nan("")));
    return 0;
}
