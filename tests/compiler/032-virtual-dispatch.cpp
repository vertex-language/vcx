// §11.7.3 -- a call through a base pointer reaches the most-derived
// override. The table it goes through is the one tests/abi diffs against
// cl; this is the call that reads it.
struct Shape {
    int side;
    virtual int area() { return 0; }
    virtual int perimeter() { return 0; }
};

struct Square : Shape {
    int area() override { return side * side; }
    int perimeter() override { return 4 * side; }
};

struct Line : Shape {
    int perimeter() override { return side; }
};

int describe(Shape* s) { return s->area() * 100 + s->perimeter(); }

int main() {
    Square sq;
    sq.side = 3;
    Line ln;
    ln.side = 7;
    Shape plain;
    plain.side = 1;

    return describe(&sq) + describe(&ln) + describe(&plain);
}
