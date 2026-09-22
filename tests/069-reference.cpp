// A reference is another name for an object.
#include <cstdio>

void swap(int& a, int& b) {
    int t = a;
    a = b;
    b = t;
}

int main() {
    int x = 1, y = 2;
    int& r = x;
    r = 10;
    swap(x, y);
    std::printf("%d %d\n", x, y);
    return 0;
}
