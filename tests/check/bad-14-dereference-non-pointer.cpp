// §7.6.2.2 [expr.unary.op]/1 -- the operand of unary * must be a pointer.
int f(int n) {
    return *n;
}
