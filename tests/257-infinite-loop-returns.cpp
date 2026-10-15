// A non-void function whose only returns are inside an infinite loop:
// `while (true)`, `while (1)`, `for (;;)` and `do ... while (true)` never
// fall off the end, so no return is missing after them.
#include <cstdio>

static int firstDivisor(int n) {
    int d = 2;
    while (true) {
        if (n % d == 0) return d;
        d++;
    }
}

static int collatzSteps(long n) {
    int steps = 0;
    while (1) {
        if (n == 1) return steps;
        n = n % 2 ? 3 * n + 1 : n / 2;
        steps++;
    }
}

static int retryUntil(int target) {
    int tries = 0;
    for (;;) {
        tries++;
        if (tries * tries >= target) return tries;
    }
}

static int digits(unsigned v) {
    int n = 0;
    do {
        n++;
        v /= 10;
        if (v == 0) return n;
    } while (true);
}

// A loop that can end by its condition still needs the return after it.
static int countDown(int n) {
    while (0) {
        n++;
    }
    return n - 1;
}

int main() {
    printf("%d %d %d %d\n", firstDivisor(91), firstDivisor(97), collatzSteps(27), retryUntil(50));
    printf("%d %d %d\n", digits(0), digits(12345), countDown(3));
    return 0;
}
