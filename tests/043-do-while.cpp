// A do-while loop runs its body at least once.
#include <cstdio>

int main() {
    int i = 10, runs = 0;
    do {
        ++runs;
    } while (i < 5);
    int digits = 0, v = 12345;
    do { ++digits; v /= 10; } while (v);
    std::printf("%d %d\n", runs, digits);
    return 0;
}
