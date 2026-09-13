// Does a class-typed global get all of its bytes?
//
// A global's storage is its type's size, not its register's: a struct of
// three long longs is twenty-four bytes in the data section, and treating
// it as the eight-byte pointer that carries it in a register overwrote
// whatever the linker placed after it.

struct Triple { long long a, b, c; };
Triple t;
int guard;

void fill() { t.a = 1; t.b = 2; t.c = 3; }

int main() {
    guard = 40;
    fill();
    return (int)(t.a + t.b + t.c) + guard;   // 46
}
