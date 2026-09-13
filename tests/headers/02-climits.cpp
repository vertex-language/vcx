// Does <climits> compile, and are its numbers the target's?

#include <climits>

static_assert(CHAR_BIT == 8);
static_assert(INT_MAX == 2147483647);
static_assert(LONG_MAX == (sizeof(long) == 8 ? 9223372036854775807L : 2147483647L));
static_assert(sizeof(long long) == 8);

int main() { return (INT_MAX / 1000000000) - 2; }
