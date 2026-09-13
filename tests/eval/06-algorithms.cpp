// The evaluator asked to run something longer than one idea. Each of these
// is a whole program: it allocates, loops, branches, calls, and returns an
// answer that is checked -- and all of it happens before a single
// instruction is emitted.

// Sieve of Eratosthenes.
constexpr int primes_below(int n) {
    bool composite[100] = {};
    int count = 0;
    for (int i = 2; i < n; ++i) {
        if (composite[i]) continue;
        ++count;
        for (int j = i * i; j < n; j += i) composite[j] = true;
    }
    return count;
}
static_assert(primes_below(10) == 4);
static_assert(primes_below(100) == 25);

// The Collatz path length, which is a loop whose trip count nobody can
// compute ahead of time.
constexpr int collatz_steps(int n) {
    int steps = 0;
    while (n != 1) {
        n = (n % 2 == 0) ? n / 2 : 3 * n + 1;
        ++steps;
    }
    return steps;
}
static_assert(collatz_steps(1) == 0);
static_assert(collatz_steps(6) == 8);
static_assert(collatz_steps(27) == 111);

// Binary search over an array built in the same evaluation.
constexpr int binary_search(int target) {
    int a[16] = {};
    for (int i = 0; i < 16; ++i) a[i] = i * 3;
    int lo = 0, hi = 15;
    while (lo <= hi) {
        int mid = lo + (hi - lo) / 2;
        if (a[mid] == target) return mid;
        if (a[mid] < target) lo = mid + 1;
        else hi = mid - 1;
    }
    return -1;
}
static_assert(binary_search(0) == 0);
static_assert(binary_search(21) == 7);
static_assert(binary_search(45) == 15);
static_assert(binary_search(22) == -1);

// An in-place sort: writes through indices, a nested loop, and a swap.
constexpr int sort_and_pick(int which) {
    int a[8] = {5, 3, 8, 1, 9, 2, 7, 4};
    for (int i = 0; i < 8; ++i) {
        for (int j = 0; j < 7 - i; ++j) {
            if (a[j] > a[j + 1]) {
                int t = a[j];
                a[j] = a[j + 1];
                a[j + 1] = t;
            }
        }
    }
    return a[which];
}
static_assert(sort_and_pick(0) == 1);
static_assert(sort_and_pick(3) == 4);
static_assert(sort_and_pick(7) == 9);

// String handling, which is array handling with a sentinel.
constexpr int length(const char* s) {
    int n = 0;
    while (s[n] != '\0') ++n;
    return n;
}
static_assert(length("") == 0);
static_assert(length("hello") == 5);
static_assert(length("a longer string") == 15);

constexpr bool equal(const char* a, const char* b) {
    int i = 0;
    while (a[i] != '\0' && b[i] != '\0') {
        if (a[i] != b[i]) return false;
        ++i;
    }
    return a[i] == b[i];
}
static_assert(equal("abc", "abc"));
static_assert(!equal("abc", "abd"));
static_assert(!equal("abc", "ab"));

// Integer square root, by the method that only terminates because the
// evaluator gets the arithmetic right.
constexpr int isqrt(int n) {
    if (n < 2) return n;
    int lo = 1, hi = n;
    while (lo < hi) {
        int mid = lo + (hi - lo + 1) / 2;
        if (mid <= n / mid) lo = mid;
        else hi = mid - 1;
    }
    return lo;
}
static_assert(isqrt(0) == 0);
static_assert(isqrt(1) == 1);
static_assert(isqrt(24) == 4);
static_assert(isqrt(25) == 5);
static_assert(isqrt(1000000) == 1000);
