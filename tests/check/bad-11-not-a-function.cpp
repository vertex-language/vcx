// §7.6.1.3 [expr.call]/1 -- the postfix-expression is not a function.
void f() {
    int n = 1;
    n(1);
}
