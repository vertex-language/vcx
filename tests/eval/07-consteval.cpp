// §8.5.2 [stmt.if] and §9.2.6 [dcl.constexpr] -- the two places a program
// can ask *when* it is running, and be answered here rather than later.

// §8.5.2/3: `if consteval` is true exactly when the statement is reached
// during constant evaluation. Every assertion in this file reaches it that
// way, so every one of them takes the first branch.
constexpr int which_branch() {
    if consteval {
        return 1;
    } else {
        return 0;
    }
}
static_assert(which_branch() == 1);

constexpr int negated() {
    if !consteval {
        return 0;
    } else {
        return 1;
    }
}
static_assert(negated() == 1);

// The form without an else, whose fall-through is the run-time path.
constexpr int no_else() {
    if consteval {
        return 42;
    }
    return 0;
}
static_assert(no_else() == 42);

// Nested, and inside a loop, so the branch is taken on every turn rather
// than folded once.
constexpr int inside_a_loop(int n) {
    int total = 0;
    for (int i = 0; i < n; ++i) {
        if consteval {
            total += i;
        } else {
            total -= i;
        }
    }
    return total;
}
static_assert(inside_a_loop(5) == 10);

// §7.7/1 [expr.const] -- the library-facing spelling of the same question.
static_assert(__builtin_is_constant_evaluated());

constexpr int agrees() {
    if consteval {
        return __builtin_is_constant_evaluated() ? 1 : 2;
    }
    return 3;
}
static_assert(agrees() == 1);

// §9.2.6/4 -- a consteval function may only be called where the answer is
// needed at compile time, which is every call in this file.
consteval int immediately(int x) { return x * 2; }
static_assert(immediately(21) == 42);

constexpr int calls_immediate() {
    if consteval {
        return immediately(10);
    }
    return 0;
}
static_assert(calls_immediate() == 20);
