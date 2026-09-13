// §7.5.5/1 [expr.prim.lambda] -- the body is a function body and is checked
// where it is written. An error in one is reported once, at the lambda.
int f() {
    auto uses_nothing = []() { return no_such_name; };
    return uses_nothing();
}
