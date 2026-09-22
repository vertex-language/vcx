// Overload resolution ranks exact matches, promotions, conversions and user conversions.
#include <cstdio>

struct From {
    From(int) {}
};

const char* f(int) { return "f(int): promotion"; }
const char* f(long) { return "f(long)"; }
const char* g(long) { return "g(long): standard conversion"; }
const char* g(From) { return "g(From)"; }
const char* h(double) { return "h(double)"; }
const char* h(int) { return "h(int)"; }
const char* k(const int&) { return "k(const int&)"; }
const char* k(int&&) { return "k(int&&)"; }

int main() {
    short s = 1;
    int i = 2;
    std::printf("%s | %s | %s | %s\n", f(s), g(i), h(1.0f), h('a'));
    std::printf("%s | %s\n", k(i), k(3));
    return 0;
}
