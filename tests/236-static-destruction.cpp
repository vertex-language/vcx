// Static objects are destroyed after main, in reverse order of construction, with atexit interleaved.
#include <cstdio>
#include <cstdlib>

struct Loud {
    const char* name;
    Loud(const char* n) : name(n) { std::printf("init %s\n", name); }
    ~Loud() { std::printf("fini %s\n", name); }
};

Loud first("first");
Loud second("second");

void handler() { std::printf("atexit handler\n"); }

Loud& lazy() {
    static Loud l("lazy");
    return l;
}

int main() {
    std::atexit(handler);
    lazy();
    std::printf("main\n");
    return 0;
}
