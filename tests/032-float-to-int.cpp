// Float to integer conversion truncates toward zero.
#include <cstdio>

int main() {
    double v[] = {3.99, -3.99, 0.5, -0.5, 2147483647.0};
    for (double d : v) std::printf("%d ", (int)d);
    std::printf("%u\n", (unsigned)4000000000.0);
    return 0;
}
