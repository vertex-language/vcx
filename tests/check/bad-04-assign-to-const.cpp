// §7.6.19 [expr.ass]/2 -- the left operand must be a modifiable lvalue.
void f() {
    const int frozen = 1;
    frozen = 2;
}
