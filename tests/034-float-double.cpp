// float widens to double exactly, and double narrows to float with rounding.
#include <cstdio>

int main() {
    float f = 0.1f;
    double d = f;
    float back = (float)0.1;
    std::printf("%.17g %.9g\n", d, back);
    return 0;
}
