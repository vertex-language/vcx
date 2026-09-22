// A union with a non-trivial member: construction and destruction by hand.
#include <cstdio>
#include <new>
#include <string>

union Slot {
    Slot() : n(0) {}
    ~Slot() {}
    int n;
    std::string s;
};

int main() {
    Slot slot;
    slot.n = 5;
    std::printf("%d\n", slot.n);
    new (&slot.s) std::string("now a string");
    slot.s += "!";
    std::printf("%s %zu\n", slot.s.c_str(), slot.s.size());
    slot.s.~basic_string();
    slot.n = 9;
    std::printf("%d\n", slot.n);
    return 0;
}
