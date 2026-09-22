// Overloads chosen by which constraints are satisfied.
#include <cstdio>

template <typename T>
const char* name(T) { return "any"; }

template <typename T>
    requires(sizeof(T) == 1)
const char* name(T) { return "byte"; }

template <typename T>
    requires(sizeof(T) == 8 && T(1) / 2 == 0)
const char* name(T) { return "wide integer"; }

int main() {
    std::printf("%s %s %s %s\n", name('a'), name(1), name(1LL), name(1.0));
    return 0;
}
