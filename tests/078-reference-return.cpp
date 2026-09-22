// A function returning a reference is an lvalue.
#include <cstdio>

int storage[4];
int& at(int i) { return storage[i]; }

int main() {
    at(1) = 7;
    at(2) = at(1) * 3;
    ++at(3);
    std::printf("%d %d %d %d\n", storage[0], storage[1], storage[2], storage[3]);
    return 0;
}
