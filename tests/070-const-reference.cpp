// A const reference binds to a temporary and extends its life.
#include <cstdio>

int square(const int& v) { return v * v; }

int main() {
    const int& t = 6 * 7;
    std::printf("%d %d\n", t, square(t + 1));
    return 0;
}
