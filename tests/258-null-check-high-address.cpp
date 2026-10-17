// A pointer compared with nullptr, 0 or another pointer compares the whole
// address: one whose low 32 bits are zero (malloc's big blocks land on
// 4 GiB boundaries) is not null, and differs from one 4 GiB away.
#include <cstdint>
#include <cstdio>

int* at(std::uintptr_t a) { return reinterpret_cast<int*>(a); }

int main() {
    int* p = at(std::uintptr_t(3) << 32);
    int* q = at(std::uintptr_t(4) << 32);
    void (*f)() = reinterpret_cast<void (*)()>(at(std::uintptr_t(5) << 32));
    int eq = p == nullptr, ne = p != nullptr, zero = p == 0, flip = nullptr == p;
    std::printf("%d %d %d %d %d %d %d\n", eq, ne, zero, flip, p == q, p < q, f != 0);
    if (p == nullptr) return 1;
    if (!(p != 0)) return 2;
    return 0;
}
