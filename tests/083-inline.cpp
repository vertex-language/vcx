// An inline function, defined in the unit and called several times.
#include <cstdio>

inline int cube(int v) { return v * v * v; }

int main() {
    int sum = 0;
    for (int i = 1; i <= 5; ++i) sum += cube(i);
    std::printf("%d\n", sum);
    return 0;
}
