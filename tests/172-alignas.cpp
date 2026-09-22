// alignas on a type and on a variable.
#include <cstdint>
#include <cstdio>

struct alignas(32) Aligned {
    int v;
};

int main() {
    alignas(64) char buf[10];
    Aligned a[2];
    std::printf("%zu %zu %d %d\n", alignof(Aligned), sizeof(Aligned),
                (int)(reinterpret_cast<std::uintptr_t>(buf) % 64), (int)((char*)&a[1] - (char*)&a[0]));
    return 0;
}
