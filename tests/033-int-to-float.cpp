// Integer to floating conversion, including values a float cannot hold exactly.
#include <cstdio>

int main() {
    int a = 16777217;
    long long b = 9007199254740993LL;
    unsigned c = 4294967295u;
    std::printf("%.1f %.1f %.1f\n", (double)(float)a, (double)b, (double)c);
    return 0;
}
