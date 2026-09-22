// Building a string in a char buffer, and reversing it in place.
#include <cstdio>

void reverse(char* s) {
    char* e = s;
    while (*e) ++e;
    while (s < --e) {
        char t = *s;
        *s++ = *e;
        *e = t;
    }
}

int main() {
    char buf[16];
    int n = 0;
    for (char c = 'a'; c <= 'h'; ++c) buf[n++] = c;
    buf[n] = 0;
    reverse(buf);
    std::printf("%s\n", buf);
    return 0;
}
