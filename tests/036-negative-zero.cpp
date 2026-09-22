// Negative zero equals zero but divides to negative infinity.
#include <cstdio>

int main() {
    double z = -0.0;
    std::printf("%d %g %g\n", z == 0.0, 1.0 / z, 1.0 / 0.0);
    return 0;
}
