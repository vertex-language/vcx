// Do new[] and delete[] allocate, construct, destroy and free an array?
//
// §7.6.2.8 [expr.new] -- `new T[n]` calls operator new[] for n elements
// (and, when T has a destructor, room for the count the platform keeps
// before them), then constructs each element first to last; `new T[n]()`
// value-initializes. §7.6.2.9 [expr.delete] -- `delete[] p` destroys the
// elements last first and calls operator delete[]; on a null pointer it
// does nothing. §6.7.6 -- an over-aligned element type takes the aligned
// forms of both, and the elements come back aligned.

int live = 0;
long order = 0;

struct Counted {
    int v;
    Counted() : v(3) { ++live; }
    ~Counted() { --live; order = order * 10 + v; }
};

struct Plain { int v; };

struct Aligned {
    alignas(32) int v;
    Aligned() { ++live; }
    ~Aligned() { --live; }
};

struct Virt {
    Virt() { ++live; }
    virtual ~Virt() { --live; }
    virtual int f() { return 1; }
};

int main() {
    int r = 0;

    // Scalars: the elements are storage, and delete[] frees it.
    int* p = new int[4];
    for (int i = 0; i < 4; ++i) p[i] = i * 2;
    if (p[0] + p[1] + p[2] + p[3] == 12) r += 1;
    delete[] p;

    // Value-initialized scalars are zero.
    int* z = new int[3]();
    if (z[0] == 0 && z[1] == 0 && z[2] == 0) r += 1;
    delete[] z;

    // A class with a constructor and no destructor: constructed, freed.
    Plain* pl = new Plain[2]{};
    if (pl[0].v == 0 && pl[1].v == 0) r += 1;
    delete[] pl;

    // A class with a destructor: a run-time count, constructed in order,
    // destroyed in reverse.
    int n = 3;
    Counted* c = new Counted[n];
    c[0].v = 5; c[1].v = 6; c[2].v = 7;
    if (live == 3) r += 1;
    delete[] c;
    if (live == 0 && order == 765) r += 1;

    // Over-aligned: the aligned allocation functions, and aligned elements.
    Aligned* a = new Aligned[2];
    if (live == 2 && ((unsigned long long)a) % 32 == 0 && ((unsigned long long)&a[1]) % 32 == 0) r += 1;
    delete[] a;
    if (live == 0) r += 1;

    // Polymorphic elements: each gets its table, each is destroyed.
    Virt* v = new Virt[2];
    if (live == 2 && v[0].f() + v[1].f() == 2) r += 1;
    delete[] v;
    if (live == 0) r += 1;

    // Deleting a null array pointer does nothing.
    Counted* none = nullptr;
    delete[] none;
    if (live == 0) r += 1;

    return r; // ten checks
}
