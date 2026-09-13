// Does a class smaller than a register come back in one, at its own width?
//
// The Microsoft convention returns a plain aggregate of one, two, four
// or eight bytes in RAX. What this asks is the part 02 did not: that a
// four-byte one comes back as four bytes. The receiver's storage is four
// bytes wide, and a compiler that stores all of RAX into it writes over
// whatever it laid next to it -- which is how this was found, with an
// iterator's end() overwritten by its begin(). The neighbours are checked
// here on purpose.

struct One { unsigned char c; };
struct Two { short s; };
struct Four { int i; };
struct Pair { short a, b; };

One make_one(int c);
Two make_two(int s);
Four make_four(int i);
Pair make_pair(int a, int b);
int sum_one(One o);
int sum_two(Two t);
int sum_four(Four f);
int sum_pair(Pair p);

int main() {
    int guard1 = 11;
    Four f = make_four(4);
    int guard2 = 22;
    Two t = make_two(2);
    int guard3 = 33;
    One o = make_one(1);
    int guard4 = 44;
    Pair p = make_pair(5, 6);
    int guard5 = 55;

    int r = 0;
    if (f.i == 4 && t.s == 2 && o.c == 1 && p.a == 5 && p.b == 6) r += 1;
    if (guard1 == 11 && guard2 == 22 && guard3 == 33 && guard4 == 44 && guard5 == 55) r += 1;
    if (sum_one(o) + sum_two(t) + sum_four(f) + sum_pair(p) == 1 + 2 + 4 + 11) r += 1;
    return r; // three checks
}
