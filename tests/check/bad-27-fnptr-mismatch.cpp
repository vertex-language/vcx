// §7.3.13 [conv.ptr] -- no implicit conversion between function pointers of different signatures.
int add(int a, int b) {
    return a + b;
}

void (*bad_fn)(int) = add;
