// Do the braces a program leaves out mean what they would have?
//
// §9.4.2 [dcl.init.aggr]/16 -- where an initializer list element lands on
// a subaggregate and is not itself a braced list or a value of that
// subaggregate's type, the subaggregate takes as many of the following
// elements as it has. So `{1, 2, 3, 4}` initializes a struct of two pairs,
// and `{kind, ptr}` a struct whose base holds kind.

struct Pair { int a; int b; };
struct Two { Pair first; Pair second; };
struct Base { int kind; };
struct Derived : Base { const int* data; int extra; };
struct WithArray { int values[3]; int tail; };
struct Matrix { int m[2][2]; };

static const int shared = 7;

int main() {
    Two t = {1, 2, 3, 4};
    Derived d = {5, &shared, 9};
    Derived braced = {{6}, &shared, 1};
    WithArray w = {10, 20, 30, 40};
    WithArray partial = {1, 2};
    Matrix m = {1, 2, 3, 4};
    Pair pairs[2] = {7, 8, 9, 10};
    Two mixed = {{1, 1}, 2, 2};
    return t.first.a + t.first.b + t.second.a + t.second.b     // 10
         + d.kind + *d.data + d.extra                          // 21
         + braced.kind + braced.extra                          // 7
         + w.values[0] + w.values[2] + w.tail                  // 80
         + partial.values[1] + partial.values[2] + partial.tail // 2
         + m.m[1][0] + m.m[0][1]                               // 5
         + pairs[1].a - pairs[0].b                             // 1
         + mixed.first.b + mixed.second.a;                     // 3 -> 129
}
