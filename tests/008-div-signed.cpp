// Signed division truncates toward zero.
#include <cstdio>

int quo(int a, int b) { return a / b; }

int main() {
    std::printf("%d %d %d %d\n", quo(7, 2), quo(-7, 2), quo(7, -2), quo(-7, -2));
    return 0;
}
