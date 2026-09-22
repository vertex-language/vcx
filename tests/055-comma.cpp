// The comma operator evaluates left to right and yields the right.
#include <cstdio>

int main() {
    int a = 0, b = 0;
    int c = (a = 3, b = a * 2, a + b);
    for (int i = 0, j = 10; i < j; ++i, --j) c += 1;
    std::printf("%d %d %d\n", a, b, c);
    return 0;
}
