// using enum, and a bitmask enum with its operators.
#include <cstdio>

enum class Perm : unsigned { None = 0, Read = 1, Write = 2, Exec = 4 };

constexpr Perm operator|(Perm a, Perm b) { return Perm(unsigned(a) | unsigned(b)); }
constexpr Perm operator&(Perm a, Perm b) { return Perm(unsigned(a) & unsigned(b)); }
constexpr bool any(Perm p) { return p != Perm::None; }

const char* describe(Perm p) {
    using enum Perm;
    switch (p) {
    case None: return "none";
    case Read: return "read";
    case Write: return "write";
    default: return "several";
    }
}

int main() {
    Perm rw = Perm::Read | Perm::Write;
    std::printf("%u %d %d %s %s\n", unsigned(rw), any(rw & Perm::Write), any(rw & Perm::Exec), describe(Perm::Read),
                describe(rw));
    return 0;
}
