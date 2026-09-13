// Does a user-provided copy constructor run when an object is copied?
//
// §11.4.5.3 -- a class that declares its own copy constructor is copied by
// calling it, and every place the language copies is a call site: `T t =
// u`, `T t(u)`, and the copy a function makes of a by-value argument. A
// prvalue is not copied at all since C++17 (§9.4/17): `T t = make()` builds
// t in place, and the copy constructor does not run, which the count here
// tells apart from the case where it does.

int copies;

struct Tracked {
    int v;
    Tracked(int x) : v(x) {}
    Tracked(const Tracked &o) : v(o.v + 100) { copies++; }
};

Tracked make(int x) { return Tracked(x); }
int read(Tracked t) { return t.v; }

int main() {
    Tracked a(1);
    Tracked b = a;          // copy: v = 101, copies = 1
    Tracked c(b);           // copy: v = 201, copies = 2
    Tracked d = make(5);    // no copy: v = 5
    int r = read(d);        // copy for the parameter: 105, copies = 3
    return b.v + c.v + d.v + r + copies - 400;   // 101 + 201 + 5 + 105 + 3 - 400 = 15
}
