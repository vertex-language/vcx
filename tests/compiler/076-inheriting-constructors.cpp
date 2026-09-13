// Does `using Base::Base;` give a derived class its base's constructors?
//
// §9.9/3 [namespace.udecl] -- a using-declaration naming a base's
// constructors makes them the derived class's own to call. §11.9.4
// [class.inhctor.init] -- such a constructor constructs the base
// subobject with the arguments, and the rest of the object as the
// implicit default constructor would: other bases default-initialized,
// members from their default member initializers. The copy constructor
// is not inherited; the derived class's own is used for a copy.

struct Base {
    int v;
    Base(int x) : v(x) {}
    Base(int x, int y) : v(x * y) {}
};

struct Derived : Base {
    using Base::Base;
    int extra = 5;          // initialized alongside the inherited construction
    int twice() { return v * 2; }
};

struct Mixin { int m = 3; };

struct Both : Base, Mixin {
    using Base::Base;
};

struct Overloaded : Base {
    using Base::Base;
    Overloaded(char c) : Base(c - 'a') {}   // its own beside the inherited
};

int main() {
    int r = 0;

    Derived d(21);
    if (d.v == 21 && d.twice() == 42 && d.extra == 5) r += 1;

    Derived e(6, 7);
    if (e.v == 42 && e.extra == 5) r += 1;

    Both b(9);
    if (b.v == 9 && b.m == 3) r += 1;

    Overloaded o1(4);
    Overloaded o2('d');
    if (o1.v == 4 && o2.v == 3) r += 1;

    Derived copy = d;
    if (copy.v == 21 && copy.extra == 5) r += 1;

    Derived* heap = new Derived(8);
    if (heap->v == 8 && heap->extra == 5) r += 1;
    delete heap;

    return r; // six checks
}
