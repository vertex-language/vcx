// Is a member initialized from its default member initializer wherever
// the class is constructed without naming it?
//
// §11.4/6 [class.mem] -- a non-static data member may carry a default
// member initializer, `= expr` or `{...}`. §11.9.3/9 [class.base.init] --
// a constructor initializes a member it does not name from that; §11.4.5.2
// -- the implicit default constructor does the same for all of them, and
// default-initializes a base or member of class type; §9.4.2/5 -- an
// aggregate's element with no item in the list takes its default member
// initializer. Every route to an object is here: a local, a base, a
// member, an array element, new, a temporary, an aggregate with too few
// items, and a constructor that names some members and not others.

int log = 0;
int next() { return ++log; }

struct Plain { int a = 7; int b{9}; };

struct Own {
    int m = 5;
    int n = m + 1;          // reads a member initialized before it
    int k;
    Own() : k(100) {}       // k named; m and n from their initializers
    Own(int x) : n(x) {}    // n named; m from its initializer, k left
};

struct Holds { Plain p; int z = 3; };     // p default-constructed: 7, 9
struct Derives : Plain { int q = 11; };   // the base's initializers too
struct Row { Plain cells[2]; };

struct Counted {
    int id = next();        // an initializer with a side effect, run once
};

struct Aggr { int x; int y = 42; };

int main() {
    int r = 0;

    Plain t;
    if (t.a == 7 && t.b == 9) r += 1;

    Own s;
    Own u(20);
    if (s.m == 5 && s.n == 6 && s.k == 100 && u.m == 5 && u.n == 20) r += 1;

    Holds h;
    Derives d;
    if (h.p.a == 7 && h.p.b == 9 && h.z == 3 && d.a == 7 && d.q == 11) r += 1;

    Row w;
    if (w.cells[0].a == 7 && w.cells[1].b == 9) r += 1;

    Plain* p = new Plain;
    Plain* q = new Plain();
    if (p->a == 7 && q->b == 9) r += 1;
    delete p;
    delete q;

    Aggr g{1};
    if (g.x == 1 && g.y == 42) r += 1;

    log = 0;
    Counted c1;
    Counted c2;
    if (c1.id == 1 && c2.id == 2 && log == 2) r += 1;

    if (Plain().a + Plain{}.b == 16) r += 1;

    return r; // eight checks
}
