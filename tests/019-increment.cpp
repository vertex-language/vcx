// Prefix and postfix increment and decrement.
#include <cstdio>

int main() {
    int i = 5;
    int a = i++;
    int b = ++i;
    int c = i--;
    int d = --i;
    std::printf("%d %d %d %d %d\n", a, b, c, d, i);
    return 0;
}
