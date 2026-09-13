// Unsigned arithmetic wraps rather than trapping, §6.8.2/2, and the
// comparison it picks is the unsigned one -- which is the whole reason
// signedness is tracked separately from width.
int main() {
    unsigned int a = 1;
    unsigned int b = 2;
    unsigned int wrapped = a - b;

    int n = 0;
    n = n + (wrapped > a);
    n = n + (a / b == 0) * 2;
    n = n + ((unsigned int)(-1) / 2 > 1000) * 4;
    n = n + (b % 3 == 2) * 8;
    return n;
}
