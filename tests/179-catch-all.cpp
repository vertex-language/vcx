// catch (...) and an exception thrown from a constructor.
#include <cstdio>

struct Member {
    Member() { std::printf("member built\n"); }
    ~Member() { std::printf("member destroyed\n"); }
};

struct Fails {
    Member m;
    Fails() { throw 3.5; }
    ~Fails() { std::printf("never\n"); }
};

int main() {
    try {
        Fails f;
    } catch (...) {
        std::printf("caught something\n");
    }
    return 0;
}
