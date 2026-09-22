// Global variables: initialized, zero-initialized, and written.
#include <cstdio>

int initialized = 7;
int zeroed;
const int table[4] = {1, 2, 4, 8};

void bump() { zeroed += initialized; }

int main() {
    bump();
    bump();
    std::printf("%d %d %d\n", initialized, zeroed, table[3]);
    return 0;
}
