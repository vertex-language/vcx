// Does a block-scope static keep its value between calls, and initialize once?
//
// §8.8/4 [stmt.dcl] -- a variable with static storage duration declared
// in a block is initialized the first time control passes through its
// declaration, and only then; it is zero-initialized before that
// (§6.7.5.2). A constant initializer is the object's image in the object
// file; a dynamic one runs behind a guard.

int calls = 0;

int expensive() {
    ++calls;
    return 100;
}

int counter() {
    static int c = 10;      // constant initializer
    return ++c;
}

int once() {
    static int v = expensive();   // dynamic initializer: runs once
    return v;
}

struct Acc {
    int total;
    Acc(int start) : total(start) {}
    void add(int n) { total += n; }
};

int accumulate(int n) {
    static Acc acc(1000);   // class type, constructed once
    acc.add(n);
    return acc.total;
}

struct Ids {
    static int next() {
        static int n;       // no initializer: zero
        return n++;
    }
};

int sameName() {
    static int c = 5;       // another c, in another function
    return c++;
}

int main() {
    int r = 0;

    counter(); counter();
    if (counter() == 13) r += 1;

    once(); once();
    if (once() == 100 && calls == 1) r += 1;

    accumulate(1); accumulate(2);
    if (accumulate(3) == 1006) r += 1;

    Ids::next(); Ids::next();
    if (Ids::next() == 2) r += 1;

    sameName();
    if (sameName() == 6 && counter() == 14) r += 1;

    return r; // five checks
}
