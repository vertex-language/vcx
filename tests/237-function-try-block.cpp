// A function-try-block on a constructor sees a member's exception, which is rethrown.
#include <cstdio>

struct Part {
    Part(int v) {
        if (v < 0) throw v;
    }
};

struct Whole {
    Part p;
    Whole(int v) try : p(v) {
        std::printf("built\n");
    } catch (int e) {
        std::printf("constructor saw %d\n", e);
    }
};

int checked(int v) try {
    if (v == 0) throw 1.5;
    return 10 / v;
} catch (double) {
    return -1;
}

int main() {
    Whole ok(1);
    try {
        Whole bad(-4);
    } catch (int e) {
        std::printf("caller caught %d\n", e);
    }
    std::printf("%d %d\n", checked(5), checked(0));
    return 0;
}
