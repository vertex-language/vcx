// Does a virtual function table get the name cl gives it?
//
// `??_7Shape@@6B@` is the class's own; a class with a second polymorphic
// base has a second table named for that base. The unit that defines a
// virtual function defines the table, which is how the names come to be in
// the object at all.

// A class with two polymorphic bases has two tables and cl names both for
// their base -- `??_7Labelled@@6BShape@@@` and `??_7Labelled@@6BNamed@@@`,
// no plain `6B@` at all -- and the second holds an adjustor thunk for each
// override, `?name@Labelled@@W7EBAPEBDXZ`, since a caller holding the Named
// subobject is eight bytes past the Labelled the override expects.
//
// Every class declares its constructor. Left implicit, cl would define one
// as a function -- a polymorphic class's default constructor installs the
// table pointer, so it is not trivial and gets emitted -- where vcx does the
// installation inline at each object's declaration. Which is right for
// linking a mixed program is a question for tests/compiler, not for names.

struct Shape {
    Shape() {}
    virtual int area() const { return 0; }
};

struct Circle : Shape {
    int r;
    Circle() : r(1) {}
    int area() const override { return 3 * r * r; }
};

struct Named {
    Named() {}
    virtual const char *name() const { return "named"; }
};

struct Labelled : Shape, Named {
    Labelled() {}
    int area() const override { return 1; }
    const char *name() const override { return "labelled"; }
};

// The tables exist in the object because objects of the classes do: cl
// emits a vftable where a constructor of the class runs, as a COMDAT the
// linker folds, and nowhere else. A class nothing constructs has no table
// in this unit and the corpus expects none.
int main() {
    Shape s;
    Circle c;
    Labelled l;
    return s.area() + c.area() + l.area() + (l.name() != nullptr);
}
