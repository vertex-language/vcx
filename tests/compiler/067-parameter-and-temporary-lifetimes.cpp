// Are by-value parameters and temporaries destroyed, and when?
//
// §6.7.7/4 [class.temporary] -- a temporary is destroyed at the end of
// the full-expression that made it, last made first; §6.7.7/6 -- one
// bound to a reference in a declaration lives as long as the reference.
// §6.7.7/3 -- a by-value parameter is destroyed when the call returns
// (which side does it is the ABI's call: the callee under Microsoft).
// §9.4/17 -- `T t = make();` builds the result in t, and no temporary
// is made at all.
//
// The log is a sequence of digits, so the order is checked, not only the
// count: 1 is a construction, 2 a copy, 3 a destruction, 4 a mark.

long log = 0;
void note(int d) { log = log * 10 + d; }

struct R {
    int v;
    R(int x) : v(x) { note(1); }
    R(const R& o) : v(o.v) { note(2); }
    ~R() { note(3); }
};

R make(int x) { return R(x); }
int take(R r) { note(4); return r.v; }
int look(const R& r) { note(4); return r.v; }
int sum(R a, R b) { note(4); return a.v + b.v; }

int main() {
    int r = 0;

    // A copy for the parameter, destroyed when the call returns, before
    // the statement after it runs: 1 2 4 3, then the local's 3.
    log = 0;
    {
        R a(1);
        take(a);
        note(4);
    }
    if (log == 124343) r += 1;

    // A temporary in an expression statement: made, used, destroyed
    // before the next statement: 1 4 3 4.
    log = 0;
    look(make(2));
    note(4);
    if (log == 1434) r += 1;

    // Elision: no copy and no temporary, one object all along: 1 4 3.
    log = 0;
    {
        R b = make(3);
        note(4);
    }
    if (log == 143) r += 1;

    // A temporary bound to a reference lives with the reference: 1 4 4 3.
    log = 0;
    {
        const R& ref = make(4);
        note(4);
        look(ref);
    }
    if (log == 1443) r += 1;

    // Two parameters: both copied before the call, both destroyed after.
    log = 0;
    {
        R x(5);
        R y(6);
        int s = sum(x, y);
        note(4);
        if (s == 11) r += 1;
    }
    if (log == 1122433433) r += 1;

    // A temporary in a condition dies before the branch runs: 1 3 4.
    log = 0;
    if (make(7).v == 7) {
        note(4);
    }
    if (log == 134) r += 1;

    // The value of a temporary survives its destruction into the return.
    log = 0;
    int v = make(8).v;
    if (v == 8 && log == 13) r += 1;

    return r; // eight checks
}
