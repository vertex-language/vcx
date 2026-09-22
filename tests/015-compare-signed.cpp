// The six comparisons on signed values.
#include <cstdio>

void cmp(int a, int b) {
    std::printf("%d%d%d%d%d%d\n", a < b, a <= b, a > b, a >= b, a == b, a != b);
}

int main() {
    cmp(1, 2);
    cmp(2, 2);
    cmp(-3, 2);
    return 0;
}
