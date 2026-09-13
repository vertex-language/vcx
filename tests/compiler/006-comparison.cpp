// §7.6.9 -- the relational operators, and the bool they produce, which
// §7.3.7 promotes to int the moment it is added to one.
int main() {
    int a = 3;
    int b = 7;
    int n = 0;
    n = n + (a < b);
    n = n + (b < a) * 2;
    n = n + (a <= a) * 4;
    n = n + (a == b) * 8;
    n = n + (a != b) * 16;
    n = n + (b > a) * 32;
    n = n + (a >= b) * 64;
    return n;
}
