// §7.3.3 [conv.func], §9.3.4.6 [dcl.fct]
// Function pointers and decayed conversions.

int compute(int x, int y) {
    return x + y;
}

void test_fn_ptr() {
    int (*p1)(int, int) = compute;
    int (*p2)(int, int) = &compute;
    int r1 = p1(1, 2);
    int r2 = (*p2)(3, 4);

    using FnType = int (*)(int, int);
    FnType p3 = p1;
    bool eq = (p1 == p2);
    bool null_check = (p3 != nullptr);
}
