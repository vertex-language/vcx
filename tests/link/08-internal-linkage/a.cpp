// Is a static function this unit's alone?
//
// §6.6 [basic.link]/3.1 -- a function declared static at namespace scope
// has internal linkage, so b.cpp's helper of the same signature is a
// different function. Given external linkage, the two collide at the
// link, or one silently answers for the other.

static int helper(int x) { return x + 1; }

namespace detail {
static int offset() { return 2; }
}

int fromB(int x);

int main() {
    return helper(10) + detail::offset() + fromB(10);   // 11 + 2 + 20 = 33
}
