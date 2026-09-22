// A captureless lambda converts to a function pointer.
#include <cstdio>

int call(int (*f)(int), int v) { return f(v); }

int main() {
    std::printf("%d\n", call([](int v) { return v * 3 + 1; }, 7));
    return 0;
}
