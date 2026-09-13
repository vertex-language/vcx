// Is a statement with an attribute still the statement?
//
// §9.12.1 [dcl.attr.grammar] -- an attribute-specifier-seq at the start of
// a statement appertains to that statement and changes nothing about what
// it is. `[[maybe_unused]] int n = 3;` declares n; `[[likely]] return x;`
// returns. The standard library writes locals this way (libc++'s allocate
// names its size `[[__maybe_unused__]] size_t __size`), and the attributed
// declaration used to vanish, leaving every later use of the name
// undeclared.

int f(int x) {
    [[maybe_unused]] int n = 3;
    switch (x) {
    case 1:
        n += 1;
        [[fallthrough]];
    case 2:
        return n;
    }
    if (x > 10) [[unlikely]]
        return -1;
    [[likely]] return n + x;
}

int main() {
    return f(1) + f(20) + f(2) + f(5);   // 4 - 1 + 3 + 8 = 14
}
