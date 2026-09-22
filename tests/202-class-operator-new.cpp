// A class's own operator new and operator delete, for objects and arrays.
#include <cstdio>
#include <cstdlib>

struct Tracked {
    int v = 0;
    static int live_bytes;
    static void* operator new(std::size_t n) {
        live_bytes += (int)n;
        std::printf("new %zu\n", n);
        return std::malloc(n);
    }
    static void operator delete(void* p, std::size_t n) {
        live_bytes -= (int)n;
        std::printf("delete %zu\n", n);
        std::free(p);
    }
};
int Tracked::live_bytes = 0;

int main() {
    Tracked* t = new Tracked;
    t->v = 5;
    std::printf("live %d\n", Tracked::live_bytes);
    delete t;
    std::printf("live %d\n", Tracked::live_bytes);
    return 0;
}
