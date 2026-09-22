// Integer subtraction, including a negative result.
#include <cstdio>

int sub(int a, int b) { return a - b; }

int main() {
    std::printf("%d %d %d\n", sub(10, 3), sub(3, 10), sub(-5, -5));
    return 0;
}
