// §7.3.9/2 -- converting to a narrower type keeps the value modulo 2^n, and
// the sign of what comes back depends on the *destination's* signedness.
int main() {
    int big = 300;
    char c = (char)big;
    unsigned char uc = (unsigned char)big;
    short s = (short)70000;
    unsigned short us = (unsigned short)70000;

    int n = 0;
    n = n + (c == 44);
    n = n + (uc == 44) * 2;
    n = n + (s == 4464) * 4;
    n = n + (us == 4464) * 8;
    return n;
}
