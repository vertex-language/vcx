// The other unit's helper: the same name and signature, and a different
// function.

static int helper(int x) { return x * 2; }

namespace detail {
static int offset() { return 100; }
}

int fromB(int x) { return helper(x); }
