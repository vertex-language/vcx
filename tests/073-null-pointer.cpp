// nullptr, null checks, and a pointer's truth value.
#include <cstdio>

const char* describe(const int* p) { return p ? "set" : "null"; }

int main() {
    int v = 3;
    int* p = nullptr;
    std::printf("%s ", describe(p));
    p = &v;
    std::printf("%s %d\n", describe(p), p != nullptr);
    return 0;
}
