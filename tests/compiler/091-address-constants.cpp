// Is the address of a static object a constant?
//
// §7.7 [expr.const]/13 -- the address of an object with static storage
// duration, of one of its members or elements, or of a function is an
// address constant expression, so a namespace-scope object initialized
// from them is data the linker completes, not code run before main.

struct Table {
    int (*pick)(int);
    const int* first;
    const int* third;
    const int* member;
};

struct Holder { int a; int b; };

static int twice(int x) { return x * 2; }
int values[4] = {5, 6, 7, 8};
Holder holder = {40, 41};

Table table = {twice, values, &values[2], &holder.b};
int (*const functions[2])(int) = {twice, &twice};
const int* const decayed = values;

int main() {
    return table.pick(*table.first) + *table.third + *table.member  // 10 + 7 + 41
         + functions[1](1) + decayed[3] - 58;                      // 2 + 8 - 58 = 10
}
