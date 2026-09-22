// Integer addition.
#include <cstdio>

int add(int a, int b) { return a + b; }

int main() {
    std::printf("%d %d %d\n", add(2, 3), add(-7, 3), add(0, 0));
    return 0;
}
