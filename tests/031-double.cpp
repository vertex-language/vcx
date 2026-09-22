// double arithmetic and division.
#include <cstdio>

int main() {
    double a = 1.0 / 3.0;
    double b = 1e308 * 10.0 / 1e308;
    double c = 0.1 + 0.2;
    std::printf("%.17g %g %.17g\n", a, b, c);
    return 0;
}
