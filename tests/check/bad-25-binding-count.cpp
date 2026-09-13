// §9.6/5 -- a structured binding's name count must match the number of non-static data members.
struct Point { int x; int y; };
int f() {
    Point p{1, 2};
    auto [a, b, c] = p;
    return a;
}
