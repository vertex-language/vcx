// §7.6.2 -- the prefix operators, including the two that are not arithmetic.
int main() {
    int a = 5;
    int b = -a;
    int c = +a;
    int d = ~a;
    int e = !a;
    int f = !0;
    return c - b + (d + 6) + e * 2 + f * 4;
}
