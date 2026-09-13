// §11.4.2 -- a member function has an implicit object parameter, which is
// what `this` names and what makes `p.sum()` a call with one argument.
struct Point {
    int x;
    int y;

    int sum() { return x + y; }
    int scaled(int by) { return (x + y) * by; }
    void move(int dx, int dy) { x = x + dx; y = y + dy; }
    int viaThis() { return this->x - this->y; }
};

int main() {
    Point p;
    p.x = 3;
    p.y = 4;

    int a = p.sum();
    p.move(1, 1);
    int b = p.scaled(2);
    int c = p.viaThis();

    Point* q = &p;
    int d = q->sum();

    return a + b + c + d;
}
