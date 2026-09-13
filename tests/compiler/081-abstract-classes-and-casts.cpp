// Do abstract classes, downcasts and class-valued conditionals behave?
//
// §11.7.4 [class.abstract] -- a class with a pure virtual function has
// no objects of its own, but a table, and a derived class overriding
// the function is called through a pointer or reference to the base.
// §7.6.1.9/2 [expr.static.cast] -- a pointer to a base converts back to
// the derived class, undoing the subobject offset a secondary base has.
// §7.6.16/6 [expr.cond] -- a conditional whose operands are class
// prvalues builds whichever is chosen in the result object; one whose
// operands are lvalues is an lvalue. §11.9.3/7 with §9.4.4 -- a
// reference member is bound by its mem-initializer.

struct Shape { virtual int area() const = 0; virtual ~Shape() {} };
struct Named { int id; Named(int i) : id(i) {} virtual int name() const { return id; } virtual ~Named() {} };
struct Sq : Shape, Named {
    int s;
    Sq(int k) : Named(k * 10), s(k) {}
    int area() const override { return s * s; }
    int name() const override { return id + 1; }
};

int viaShape(const Shape& sh) { return sh.area(); }
int viaNamed(const Named* n) { return n->name(); }

struct T { int v; T(int x) : v(x) {} };
T pick(bool b) { return b ? T(1) : T(2); }

struct Ref { int& r; Ref(int& x) : r(x) {} void bump() { ++r; } };
struct Cache { mutable int hits = 0; int v; Cache(int x) : v(x) {} int get() const { ++hits; return v; } int get() { return v * 2; } };

int main() {
    int r = 0;

    // Through both bases, and back down from the secondary one.
    Sq q(4);
    Shape* s = &q;
    Named* n = &q;
    Sq* back = static_cast<Sq*>(n);
    if (viaShape(q) == 16 && viaNamed(&q) == 41 && s->area() == 16 && n->name() == 41 && back->s == 4 && back == &q) r += 1;

    // An abstract base deleted through its pointer runs the derived destructor.
    Shape* heap = new Sq(2);
    if (heap->area() == 4) r += 1;
    delete heap;

    // Class prvalues in a conditional, and class lvalues.
    T a(5), b(6);
    T& ref = true ? a : b;
    ref.v = 50;
    const T& cref = false ? a : b;
    if (a.v == 50 && cref.v == 6 && pick(false).v == 2 && pick(true).v == 1) r += 1;

    // A reference member refers to the entity.
    int count = 1;
    Ref rf(count);
    rf.bump();
    rf.bump();
    if (count == 3) r += 1;

    // const and non-const overloads by the object's constness; mutable.
    Cache c(5);
    const Cache& cc = c;
    if (c.get() == 10 && cc.get() == 5 && cc.get() == 5 && cc.hits == 2) r += 1;

    return r; // five checks
}
