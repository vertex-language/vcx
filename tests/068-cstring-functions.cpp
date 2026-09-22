// <cstring>: strlen, strcmp, strcpy, memcpy, memset.
#include <cstdio>
#include <cstring>

int main() {
    char buf[32];
    std::strcpy(buf, "hello");
    std::strcat(buf, ", world");
    int d = std::strcmp("abc", "abd");
    char z[8];
    std::memset(z, 'x', 7);
    z[7] = 0;
    char c[8];
    std::memcpy(c, z, 8);
    std::printf("%s %zu %d %s\n", buf, std::strlen(buf), d < 0, c);
    return 0;
}
