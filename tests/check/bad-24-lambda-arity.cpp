// §7.5.5.1/3 -- the closure's operator() takes the lambda's parameters, so
// calling one with the wrong number of arguments is an ordinary bad call.
int f() {
    auto takes_two = [](int a, int b) { return a + b; };
    return takes_two(1);
}
