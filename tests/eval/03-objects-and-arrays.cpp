// §7.7/5 [expr.const] -- the evaluator has to model storage, not only
// values: a local that is written to, an array that is indexed, a pointer
// that is dereferenced. This is where a constant folder stops and an
// interpreter begins.

constexpr int array_sum() {
    int a[5] = {1, 2, 3, 4, 5};
    int s = 0;
    for (int i = 0; i < 5; ++i) s += a[i];
    return s;
}
static_assert(array_sum() == 15);

constexpr int array_write() {
    int a[4] = {};
    for (int i = 0; i < 4; ++i) a[i] = i * i;
    return a[0] + a[1] + a[2] + a[3];
}
static_assert(array_write() == 14);

constexpr int array_partial_init() {
    int a[4] = {7};
    return a[0] + a[1] + a[2] + a[3];
}
static_assert(array_partial_init() == 7);

constexpr int reverse_in_place() {
    int a[5] = {1, 2, 3, 4, 5};
    for (int i = 0, j = 4; i < j; ++i, --j) {
        int t = a[i];
        a[i] = a[j];
        a[j] = t;
    }
    return a[0] * 10000 + a[1] * 1000 + a[2] * 100 + a[3] * 10 + a[4];
}
static_assert(reverse_in_place() == 54321);

// §7.6.2.2 -- a pointer into an object, and a write through it.
constexpr int through_pointer() {
    int x = 0;
    int* p = &x;
    *p = 7;
    return x;
}
static_assert(through_pointer() == 7);

constexpr int pointer_into_array() {
    int a[3] = {10, 20, 30};
    int* p = a;
    return p[1];
}
static_assert(pointer_into_array() == 20);

// A nested call that mutates its own locals, so the evaluator's frames have
// to be separate.
constexpr int accumulate(int n) {
    int total = 0;
    for (int i = 0; i < n; ++i) {
        int local = i;
        local *= 2;
        total += local;
    }
    return total;
}
static_assert(accumulate(5) == 20);

constexpr int nested_frames(int n) {
    if (n == 0) return 0;
    int here = n;
    int below = nested_frames(n - 1);
    return here + below;
}
static_assert(nested_frames(10) == 55);

// §8.6.5 [stmt.ranged] -- the ranged loop, run at compile time. The general
// expansion calls begin() and end(); over an array, a braced list or a
// string literal the elements are right there, and those are the ranges a
// constexpr routine walks.
constexpr int range_sum() {
    int a[4] = {1, 2, 3, 4};
    int s = 0;
    for (int v : a) s += v;
    return s;
}
static_assert(range_sum() == 10);

constexpr int range_deduced() {
    int a[3] = {5, 6, 7};
    int s = 0;
    for (auto v : a) s += v;
    return s;
}
static_assert(range_deduced() == 18);

// A reference binds to the element, so what the body writes stays written.
constexpr int range_writes_back() {
    int a[3] = {1, 1, 1};
    for (auto& v : a) v = 4;
    return a[0] + a[1] + a[2];
}
static_assert(range_writes_back() == 12);

constexpr int range_break() {
    int a[5] = {1, 2, 3, 4, 5};
    int s = 0;
    for (auto v : a) {
        if (v == 4) break;
        s += v;
    }
    return s;
}
static_assert(range_break() == 6);

constexpr int range_continue() {
    int a[5] = {1, 2, 3, 4, 5};
    int s = 0;
    for (auto v : a) {
        if (v % 2) continue;
        s += v;
    }
    return s;
}
static_assert(range_continue() == 6);

constexpr int range_over_braced_list() {
    int s = 0;
    for (auto v : {10, 20, 30}) s += v;
    return s;
}
static_assert(range_over_braced_list() == 60);

// A string literal is an array of char and its null is part of it, so the
// loop makes four turns over "abc" and three of them are non-zero.
constexpr int range_over_literal() {
    int n = 0;
    for (char c : "abc") {
        if (c) ++n;
    }
    return n;
}
static_assert(range_over_literal() == 3);
