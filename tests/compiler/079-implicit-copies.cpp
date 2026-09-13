// Do the implicit copy constructor and copy assignment copy member by member?
//
// §11.4.5.3/14 [class.copy.ctor] -- the implicitly-defined copy
// constructor copies the bases and members, each by its own copy
// constructor; §11.4.6/12 [class.copy.assign] -- the implicitly-defined
// copy assignment operator assigns them, each by its own operator=. A
// member that counts its copies says whether that happened, and a
// member that changes what it copies says which of the two ran: a class
// with a user-provided constructor but no operator= is assigned through
// its operator=, never constructed anew.

int copies = 0;
int assigns = 0;

struct Buf {
    int n;
    Buf(int k) : n(k) {}
    Buf(const Buf& o) : n(o.n + 1000) { ++copies; }
    Buf& operator=(const Buf& o) { n = o.n + 100; ++assigns; return *this; }
};

struct Holder {                   // no copy of its own: member-wise
    Buf b;
    int tag;
    Holder(int k) : b(k), tag(k) {}
};

struct Deep {                     // a member with the implicit copy
    Holder h;
    Buf arr[2];
    Deep(int k) : h(k), arr{k + 1, k + 2} {}
};

struct Base { Buf b; Base(int k) : b(k) {} };
struct Derived : Base { int d; Derived(int k) : Base(k), d(k * 2) {} };

struct Plain { int a, b; };
struct Mixed { Plain p; int q; Buf b; Mixed(int k) : p{k, k}, q(k), b(k) {} };

int main() {
    int r = 0;

    // Copy-construction of a Holder runs Buf's copy constructor once.
    copies = assigns = 0;
    Holder h1(5);
    Holder h2 = h1;
    if (h2.b.n == 1005 && h2.tag == 5 && copies == 1 && assigns == 0) r += 1;

    // Assignment runs Buf's operator=, not its copy constructor.
    copies = assigns = 0;
    Holder h3(7);
    h3 = h1;
    if (h3.b.n == 105 && h3.tag == 5 && copies == 0 && assigns == 1) r += 1;

    // Nested: the Holder member's implicit copy, and the array's elements.
    copies = assigns = 0;
    Deep d1(1);
    Deep d2 = d1;
    if (d2.h.b.n == 1001 && d2.arr[0].n == 1002 && d2.arr[1].n == 1003 && copies == 3) r += 1;
    copies = assigns = 0;
    Deep d3(9);
    d3 = d1;
    if (d3.h.b.n == 101 && d3.arr[1].n == 103 && d3.h.tag == 1 && assigns == 3 && copies == 0) r += 1;

    // A base's member is copied through the base.
    copies = assigns = 0;
    Derived e1(3);
    Derived e2 = e1;
    Derived e3(8);
    e3 = e1;
    if (e2.b.n == 1003 && e2.d == 6 && e3.b.n == 103 && e3.d == 6 && copies == 1 && assigns == 1) r += 1;

    // Plain members go by value alongside the one that counts.
    copies = assigns = 0;
    Mixed m1(4);
    Mixed m2 = m1;
    m2.p.a = 40;
    Mixed m3(6);
    m3 = m2;
    if (m2.b.n == 1004 && m3.p.a == 40 && m3.p.b == 4 && m3.q == 4 && m3.b.n == 1104 && copies == 1 && assigns == 1) r += 1;

    // Passing by value and returning by value copy-construct.
    copies = assigns = 0;
    auto take = [](Holder h) { return h.b.n; };
    if (take(h1) == 1005 && copies == 1) r += 1;

    return r; // seven checks
}
