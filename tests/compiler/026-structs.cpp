// §11.4 -- a class's members, reached by name. The offsets are the ones
// tests/abi already checks against cl; this checks that reading and writing
// through them agrees too.
struct Point { int x; int y; };
struct Mixed { char c; int i; double d; };

int main() {
    Point p;
    p.x = 3;
    p.y = 4;

    Mixed m;
    m.c = 7;
    m.i = 100;
    m.d = 2.5;

    int n = p.x * p.y;
    n = n + m.c + m.i;
    n = n + (int)m.d;
    return n;
}
