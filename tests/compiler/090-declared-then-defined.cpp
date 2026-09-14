// Does a function's body see the parameter names its definition gave?
//
// §9.3.4.6 [dcl.fct]/8 -- parameter names are not part of a function's
// type, so a declaration without them and a definition with them are one
// function, and the body names the definition's. The definition used to
// be merged into the declaration with the declaration's unnamed
// parameters, and the body's names were undeclared.

int add(int, int);
int scale(int value, int by);
static int twice(int);

class Box {
    int shut = 20;
    friend int peek(const Box&);
};

int add(int a, int b) { return a + b; }
int scale(int x, int factor) { return x * factor; }
static int twice(int n) { return n * 2; }
int peek(const Box& box) { return box.shut; }

int main() {
    Box b;
    return add(1, 2) + scale(3, 4) + twice(5) + peek(b);   // 3 + 12 + 10 + 20 = 45
}
