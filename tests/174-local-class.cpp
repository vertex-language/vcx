// A class defined inside a function.
#include <cstdio>

int run() {
    struct Acc {
        int total = 0;
        void add(int v) { total += v * v; }
    };
    Acc a;
    for (int i = 1; i <= 4; ++i) a.add(i);
    return a.total;
}

int main() {
    std::printf("%d\n", run());
    return 0;
}
