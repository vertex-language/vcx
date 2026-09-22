// A static local keeps its value between calls.
#include <cstdio>

int counter() {
    static int n = 100;
    return n++;
}

int main() {
    counter();
    counter();
    std::printf("%d\n", counter());
    return 0;
}
