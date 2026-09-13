// §9.5.1 -- a non-void function must return on every path, and these all do.
int if_else(int a) {
    if (a > 0) return 1;
    else return -1;
}

int if_no_else(int a) {
    if (a > 0) return 1;
    return -1;
}

int switch_with_default(int a) {
    switch (a) {
    case 0: return 10;
    case 1: return 20;
    default: return 30;
    }
}

int loops(int n) {
    while (n > 0) { --n; if (n == 3) return n; }
    for (int i = 0; i < n; ++i) { if (i) return i; }
    do { --n; } while (n > 0);
    return 0;
}

int nested(int a, int b) {
    if (a) {
        if (b) return 1;
        else return 2;
    } else {
        while (b) return 3;
        return 4;
    }
}

[[noreturn]] void never_returns();
int after_noreturn(int a) {
    if (a) return 1;
    never_returns();
    return 0;
}

void void_needs_nothing() { }
void void_may_return() { return; }

// §8.7.6 [stmt.goto] -- a goto is an unconditional transfer, and a label is
// reachable from every goto to it. Both halves matter to the flow analysis:
// without the jump edge the code after a label looked unreachable whenever
// the statement before it returned, which is the shape goto is written in.
int jumps_forward(int a) {
    if (a) goto end;
    return 1;
end:
    return 0;
}

int jumps_backward(int a) {
top:
    if (a) {
        --a;
        goto top;
    }
    return a;
}

int out_of_a_loop(int a) {
    for (;;) {
        if (a) goto done;
        --a;
    }
done:
    return 0;
}

// A label may also be reached by falling into it, which is the edge the
// goto does not provide.
int falls_into_a_label(int a) {
    if (a) goto here;
here:
    return 0;
}
