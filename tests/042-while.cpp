// A while loop.
#include <cstdio>

int main() {
    int n = 27, steps = 0;
    while (n != 1) {
        n = n % 2 ? 3 * n + 1 : n / 2;
        ++steps;
    }
    std::printf("%d\n", steps);
    return 0;
}
