// Does a virtual call cross the boundary?
//
// The class hierarchy is declared on both sides; each side defines some of
// the overrides and constructs some of the objects. A call through a base
// pointer on one side reaches a table the other side emitted, and a slot
// in it that the other side filled, including through a secondary base --
// where the Microsoft convention has the override expect the base
// subobject's address, and both compilers had better agree on that.

struct Shape {
    int sides;
    Shape() : sides(0) {}
    virtual int area() const;
    virtual int perimeter() const { return sides; }
};

struct Named {
    int tag;
    Named() : tag(7) {}
    virtual int name() const { return 1; }
};

struct Square : Shape {
    int side;
    Square(int s);
    int area() const override;
};

struct Labelled : Shape, Named {
    int extra;
    Labelled();
    int area() const override { return 100; }
    int name() const override { return extra + tag + sides; }
};

int via_shape(Shape *s);
int via_named(Named *n);
Shape *make_square(int s);
Labelled *make_labelled();

int main() {
    Shape *sq = make_square(3);
    Labelled *l = make_labelled();
    Labelled local;
    return via_shape(sq) + sq->perimeter() + via_named(l) + via_shape(l) + via_named(&local) - 200;
}
