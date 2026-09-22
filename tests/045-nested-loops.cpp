// Loops inside loops.
#include <cstdio>

int main() {
    int count = 0;
    for (int a = 1; a < 20; ++a)
        for (int b = a; b < 20; ++b)
            for (int c = b; c < 30; ++c)
                if (a * a + b * b == c * c) ++count;
    std::printf("%d\n", count);
    return 0;
}
