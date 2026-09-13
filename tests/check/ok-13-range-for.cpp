// §8.6.5 [stmt.ranged] -- the ranged loop, and the deduction that makes it
// the loop people actually write.
//
// The loop variable is deduced from `*__begin`, which is the one declaration
// in the language whose `auto` has no initializer to read. An array says
// what its element is, so these check; a class range would need its begin
// and end found and called, and that call is not made yet -- which is why
// every range here is an array.

int sum_by_value() {
    int a[5] = {1, 2, 3, 4, 5};
    int s = 0;
    for (int v : a) s += v;
    return s;
}

int sum_deduced() {
    int a[5] = {1, 2, 3, 4, 5};
    int s = 0;
    for (auto v : a) s += v;
    return s;
}

int sum_by_const_ref() {
    int a[5] = {1, 2, 3, 4, 5};
    int s = 0;
    for (const auto& v : a) s += v;
    return s;
}

void write_through_ref() {
    int a[4] = {};
    for (auto& v : a) v = 7;
    for (auto&& v : a) v = 8;
}

int from_braced_list() {
    int s = 0;
    for (auto v : {1, 2, 3}) s += v;
    return s;
}

// C++20's init-statement, and the control-flow transfers inside a loop that
// only became a loop after §8.6.5/2 expanded it.
int with_init_statement() {
    int a[3] = {1, 2, 3};
    for (int base = 10; auto v : a) {
        if (v == 2) continue;
        if (v == 3) break;
        base += v;
    }
    return 0;
}

// A ranged loop may run zero times, so it is not by itself a return.
int returns_after_the_loop(int n) {
    int a[3] = {1, 2, 3};
    for (auto v : a) {
        if (v == n) return v;
    }
    return -1;
}

// Nested, and with a wider element than the accumulator.
long long nested() {
    long long a[2] = {1, 2};
    long long b[2] = {10, 20};
    long long total = 0;
    for (auto x : a) {
        for (auto y : b) total += x * y;
    }
    return total;
}
