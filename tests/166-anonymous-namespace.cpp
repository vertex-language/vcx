// An anonymous namespace and static functions give internal linkage.
#include <cstdio>

namespace {
int hidden = 11;
int twice(int v) { return v * 2; }
}  // namespace

static int thrice(int v) { return v * 3; }

int main() {
    std::printf("%d\n", twice(hidden) + thrice(1));
    return 0;
}
