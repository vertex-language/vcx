// snprintf formats into a buffer and reports the length it wanted.
#include <cstdio>

int main() {
    char buf[16];
    int n = std::snprintf(buf, sizeof buf, "%d-%s-%.2f", 42, "abc", 3.14159);
    char small[6];
    int m = std::snprintf(small, sizeof small, "%s", "truncated");
    std::printf("%s %d %s %d\n", buf, n, small, m);
    return 0;
}
