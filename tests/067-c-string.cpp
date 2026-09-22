// A C string: its terminator, and a length loop.
#include <cstdio>

int length(const char* s) {
    int n = 0;
    while (s[n]) ++n;
    return n;
}

int main() {
    const char* s = "vertex";
    char buf[] = "abc";
    std::printf("%d %zu %d %c\n", length(s), sizeof buf, buf[3] == 0, s[2]);
    return 0;
}
