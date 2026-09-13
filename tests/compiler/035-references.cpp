// Can a reference be bound, read through, and written through?
//
// §9.3.4.3 -- a reference is another name for an object, so every use of it
// is a use of the object it was bound to. In the object file that is an
// address: binding takes one, and every read or write goes through it.

void bump(int &n) { n += 10; }

int &pick(int &a, int &b, bool first) { return first ? a : b; }

int main() {
    int x = 1;
    int y = 2;
    int &rx = x;
    rx = 5;            // writes x
    bump(y);           // y is 12
    pick(x, y, false) = 20;   // y is 20
    const int &cr = x + y;    // bound to a temporary holding 25
    return rx + y + cr;       // 5 + 20 + 25
}
