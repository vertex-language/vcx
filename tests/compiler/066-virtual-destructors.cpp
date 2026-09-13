// Does `delete` through a base pointer run the derived destructor?
//
// §7.6.2.9/3 [expr.delete] -- if the static type of the object differs
// from its dynamic type, the static type's destructor must be virtual, and
// the destructor called is the dynamic type's. §11.4.7/9 then runs the
// base destructors after it. A non-virtual destructor through a matching
// static type is the ordinary direct call, and a null pointer deletes
// nothing (§7.6.2.9/7).

int log = 0;

struct Base {
    int id;
    Base(int i) : id(i) {}
    virtual ~Base() { log = log * 10 + 1; }
    virtual int kind() const { return 1; }
};

struct Middle : Base {
    Middle(int i) : Base(i) {}
    ~Middle() override { log = log * 10 + 2; }
    int kind() const override { return 2; }
};

struct Leaf : Middle {
    int* counter;
    Leaf(int i, int* c) : Middle(i), counter(c) {}
    ~Leaf() override { log = log * 10 + 3; *counter += 1; }
    int kind() const override { return 3; }
};

struct Plain {
    int v;
    ~Plain() { log = log * 10 + 7; }
};

int main() {
    int r = 0;
    int leaves = 0;

    // Leaf, then Middle, then Base: 321.
    Base* b = new Leaf(1, &leaves);
    int k = b->kind();
    delete b;
    if (k == 3 && log == 321 && leaves == 1) r += 1;

    // Through the middle of the hierarchy: still the dynamic type's.
    log = 0;
    Middle* m = new Leaf(2, &leaves);
    delete m;
    if (log == 321 && leaves == 2) r += 1;

    // The static type is the dynamic type: 21.
    log = 0;
    Base* mid = new Middle(3);
    delete mid;
    if (log == 21) r += 1;

    // No virtual destructor, no dispatch: 7.
    log = 0;
    Plain* p = new Plain{5};
    delete p;
    if (log == 7) r += 1;

    // Deleting null does nothing.
    log = 0;
    Base* none = nullptr;
    delete none;
    if (log == 0) r += 1;

    return r; // five checks
}
