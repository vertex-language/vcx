// The operators bind the way they are written, and parentheses are the only
// thing that changes it. Nothing here is checked against a number: the
// question is whether both compilers group it the same way.
int main() {
    int a = 2, b = 3, c = 4;
    return 1 + a * b - c / 2 + (a + b) * c - ((a - b) * (c - 1));
}
