// §7.6.2.2/3 -- the operand of unary & must be an lvalue.
int* f() {
    return &1;
}
