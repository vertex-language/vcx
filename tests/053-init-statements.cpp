// if and switch with an init-statement.
#include <cstdio>

int lookup(int k) { return k * k - 10; }

int main() {
    if (int v = lookup(4); v > 0) std::printf("positive %d\n", v);
    if (int v = lookup(2); v > 0) std::printf("never\n");
    else std::printf("not positive %d\n", v);
    switch (int v = lookup(3); v) {
    case -1: std::printf("minus one\n"); break;
    default: std::printf("other %d\n", v);
    }
    return 0;
}
