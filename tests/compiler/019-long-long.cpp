// The types that live in a 64-bit register, and the conversions between the
// two widths -- where a sign extension and a zero extension differ.
long long wide(long long n) { return n * 3; }

int main() {
    long long a = 1000000000;
    long long b = a * 4;
    long long c = wide(b);

    int n = 0;
    n = n + (b == 4000000000LL);
    n = n + (c == 12000000000LL) * 2;
    n = n + ((int)(a / 1000000) == 1000) * 4;
    n = n + ((long long)(-1) < 0) * 8;
    return n;
}
