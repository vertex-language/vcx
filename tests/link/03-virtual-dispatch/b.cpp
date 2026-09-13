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

int Shape::area() const { return 0; }
Square::Square(int s) : side(s) { sides = 4; }
int Square::area() const { return side * side; }
Labelled::Labelled() : extra(50) { sides = 3; }

int via_shape(Shape *s) { return s->area(); }
int via_named(Named *n) { return n->name(); }

static Square sq_storage(3);
static Labelled l_storage;
Shape *make_square(int s) { return &sq_storage; }
Labelled *make_labelled() { return &l_storage; }
