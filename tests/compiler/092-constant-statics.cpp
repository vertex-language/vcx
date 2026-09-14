// Is an object initialized from constants data, wherever it is declared?
//
// §6.9.3.2 [basic.start.static] and §8.8 [stmt.dcl]/3 -- a static object
// whose initializer is a constant expression is initialized before any
// code runs, a function-local one included, so there is no guard to test
// and no constructor to register. Declaration order does not matter
// either: an initializer may take the address of a global declared after
// it. A size or an alignment in such an initializer is the layout's --
// which the constant table once recorded as zero.

struct Entry { const char* name; const int* value; };

extern int later;
Entry entry = {"later", &later};
int later = 32;

struct Sized { long long a; int b; };
struct Table { unsigned long size; unsigned long align; };
static const Table layout = {sizeof(Sized), alignof(Sized)};
static_assert(sizeof(Sized) == 16);

inline constexpr unsigned flag = 1u << 3;
unsigned mask = 7 | flag;

int digit(int i) {
    static const char digits[] = "0123456789";
    static const int* first = &later;
    return (digits[i] - '0') + *first;
}

int main() {
    return *entry.value + int(mask) + digit(3) - 32    // 32 + 15 + 35 - 32 = 50
         + int(layout.size) - int(layout.align);       // 16 - 8 -> 58
}
