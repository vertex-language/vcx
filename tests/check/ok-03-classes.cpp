struct Point {
    int x;
    int y;
    int sum() const { return x + y; }
    void move(int dx, int dy) { x += dx; y += dy; }
};

class Counter {
public:
    void bump() { ++n; }
    int value() const { return n; }
private:
    int n = 0;
};

int use() {
    Point p{1, 2};
    p.move(1, 1);
    Counter c;
    c.bump();
    return p.sum() + c.value();
}
