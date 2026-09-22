// &&, || and ! short-circuit and yield bool.
#include <cstdio>

int calls = 0;
bool touch(bool v) { ++calls; return v; }

int main() {
    bool a = touch(false) && touch(true);
    bool b = touch(true) || touch(false);
    bool c = !touch(false);
    std::printf("%d %d %d calls=%d\n", a, b, c, calls);
    return 0;
}
