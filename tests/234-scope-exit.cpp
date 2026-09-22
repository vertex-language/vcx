// Leaving a scope by break, continue, goto and return destroys its objects.
#include <cstdio>

struct Guard {
    int id;
    Guard(int i) : id(i) {}
    ~Guard() { std::printf("~%d ", id); }
};

int run() {
    for (int i = 0; i < 4; ++i) {
        Guard g(i);
        if (i == 1) continue;
        if (i == 2) break;
    }
    std::printf("| ");
    {
        Guard a(10);
        goto out;
    }
out:
    std::printf("| ");
    Guard r(20);
    return 7;
}

int main() {
    int v = run();
    std::printf("\n%d\n", v);
    return 0;
}
