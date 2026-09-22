// Throwing unwinds the stack, destroying every live object on the way.
#include <cstdio>

struct Guard {
    int id;
    Guard(int i) : id(i) {}
    ~Guard() { std::printf("release %d\n", id); }
};

void inner() {
    Guard g(3);
    throw 1;
}

void middle() {
    Guard g(2);
    inner();
    std::printf("never\n");
}

int main() {
    Guard g(1);
    try {
        middle();
    } catch (int) {
        std::printf("caught\n");
    }
    return 0;
}
