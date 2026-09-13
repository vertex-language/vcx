// §7.6.1.5/3 -- `p->m` is `(*p).m`, so a class reached through a pointer is
// the same access with one more indirection. A function taking a pointer is
// how a class is passed anywhere at all until passing by value works.
struct Point { int x; int y; };

void move(Point* p, int dx, int dy) {
    p->x = p->x + dx;
    p->y = p->y + dy;
}

int manhattan(Point* p) {
    int x = p->x;
    int y = p->y;
    if (x < 0) x = -x;
    if (y < 0) y = -y;
    return x + y;
}

int main() {
    Point p;
    p.x = 1;
    p.y = 2;
    move(&p, 10, -20);

    Point* q = &p;
    q->x = q->x + 1;

    return manhattan(&p) * 10 + (q == &p);
}
