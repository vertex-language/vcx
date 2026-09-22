// The conditional operator evaluates only the chosen arm.
#include <cstdio>

int big(int v) { return v > 10 ? v * 2 : v - 1; }

int main() {
    std::printf("%d %d\n", big(20), big(5));
    int x = 0;
    int y = true ? 1 : ++x;
    std::printf("%d %d\n", y, x);
    return 0;
}
