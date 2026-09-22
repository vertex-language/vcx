// short and unsigned short, and the wrap of the unsigned one.
#include <cstdio>

int main() {
    short s = -32768;
    unsigned short u = 65535;
    u = u + 1;
    std::printf("%d %u %zu\n", s, u, sizeof(short));
    return 0;
}
