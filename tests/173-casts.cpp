// static_cast, const_cast, reinterpret_cast, and a C-style cast.
#include <cstdint>
#include <cstdio>

int main() {
    double d = 9.75;
    int i = static_cast<int>(d);
    const int ci = 5;
    const int* cp = &ci;
    int local = 7;
    const int* lp = &local;
    *const_cast<int*>(lp) = 8;
    std::uintptr_t addr = reinterpret_cast<std::uintptr_t>(&local);
    int* back = reinterpret_cast<int*>(addr);
    std::printf("%d %d %d %d\n", i, *cp, local, *back == (int)(long)8.9);
    return 0;
}
