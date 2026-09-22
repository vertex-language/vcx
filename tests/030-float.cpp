// float arithmetic, printed with enough digits to see rounding.
#include <cstdio>

int main() {
    float a = 1.0f / 3.0f;
    float b = a * 3.0f;
    float c = 0.1f + 0.2f;
    std::printf("%.9g %.9g %.9g\n", a, b, c);
    return 0;
}
