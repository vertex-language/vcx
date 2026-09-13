// §7.6.19 -- assignment yields the value assigned, so it can be chained and
// used where a value is wanted.
int main() {
    int a = 1;
    int b = 2;
    a = b = 5;
    int c = (a = a + 1);
    return a + b + c;
}
