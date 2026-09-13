// 057-function-pointers.cpp
// Tests function pointers:
// - declaration and initialization with &fn and fn
// - indirect call via fn(args) and (*fn)(args)
// - passing function pointer as argument
// - array of function pointers
// - type alias (using / typedef) for function pointers
// - struct member function pointer
// - static member function pointer
// - function pointer comparison (equality, null)

int add(int a, int b) {
    return a + b;
}

int sub(int a, int b) {
    return a - b;
}

int mul(int a, int b) {
    return a * b;
}

int double_val(int x) {
    return x * 2;
}

using BinaryOp = int (*)(int, int);
typedef int (*UnaryOp)(int);

struct Calculator {
    BinaryOp op;
    UnaryOp post;

    static int cube(int x) {
        return x * x * x;
    }

    int compute(int a, int b) {
        int r = op(a, b);
        if (post) {
            r = post(r);
        }
        return r;
    }
};

int apply(BinaryOp fn, int x, int y) {
    return fn(x, y);
}

int main() {
    BinaryOp op = &add;
    int r1 = op(10, 20); // 30

    op = sub;
    int r2 = (*op)(50, 15); // 35

    int r3 = apply(mul, 6, 7); // 42

    BinaryOp ops[3];
    ops[0] = add;
    ops[1] = sub;
    ops[2] = mul;

    int r4 = ops[0](1, 2) + ops[1](10, 4) + ops[2](3, 3); // 3 + 6 + 9 = 18

    Calculator calc;
    calc.op = add;
    calc.post = double_val;
    int r5 = calc.compute(4, 6); // (4 + 6) * 2 = 20

    UnaryOp static_fn = &Calculator::cube;
    int r6 = static_fn(3); // 27

    // Comparison tests
    int r7 = 0;
    BinaryOp p1 = add;
    BinaryOp p2 = &add;
    BinaryOp p_null = nullptr;
    if (p1 == p2) r7 += 1;
    if (p1 != sub) r7 += 2;
    if (p_null == nullptr) r7 += 4;
    // r7 should be 1 + 2 + 4 = 7

    // Total: 30 + 35 + 42 + 18 + 20 + 27 + 7 = 179
    return r1 + r2 + r3 + r4 + r5 + r6 + r7;
}
