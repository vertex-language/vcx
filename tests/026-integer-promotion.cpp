// Small integer types promote to int before arithmetic.
#include <cstdio>

int main() {
    unsigned char a = 200, b = 100;
    int sum = a + b;
    short s = 30000;
    int twice = s * 2;
    std::printf("%d %d\n", sum, twice);
    return 0;
}
