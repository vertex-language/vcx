// A pointer: address-of, dereference, and writing through it.
#include <cstdio>

int main() {
    int x = 5;
    int* p = &x;
    *p = *p + 10;
    std::printf("%d %d\n", x, p == &x);
    return 0;
}
