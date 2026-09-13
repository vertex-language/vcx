// Does a virtual call through a secondary base reach the override, and does
// the override see the whole object?
//
// A class with two polymorphic bases has two tables. A call through a
// pointer to the second base finds the second table, and the override it
// reaches has to get back to the complete object to read its own members.
// The Microsoft ABI does that in the callee -- the override is compiled
// expecting the second base's address -- and every direct call has to know
// that too, so `l.name()` and `n->name()` must agree on what they pass.

struct Shape {
    int sides;
    Shape() : sides(3) {}
    virtual int area() const { return 0; }
};

struct Named {
    int tag;
    Named() : tag(7) {}
    virtual int name() const { return 1; }
};

struct Labelled : Shape, Named {
    int extra;
    Labelled() : extra(100) {}
    int area() const override { return sides * 2; }
    int name() const override { return extra + tag + sides; }
};

int via_named(Named *n) { return n->name(); }
int via_shape(Shape *s) { return s->area(); }

int main() {
    Labelled l;
    int direct = l.name();          // 110
    int through = via_named(&l);    // 110
    int primary = via_shape(&l);    // 6
    Named plain;
    return direct + through + primary + via_named(&plain) - 200;  // 27
}
