// Doubles, and the conversions between them and the integers -- §7.3.11
// truncates toward zero, and §7.3.7 widens on the way in.
double half(double x) { return x / 2.0; }

int main() {
    double a = 10.0;
    double b = half(a);
    double c = a * b + 1.5;
    int truncated = (int)c;
    double fromInt = 7;

    int n = 0;
    n = n + (b == 5.0);
    n = n + (truncated == 51) * 2;
    n = n + (fromInt == 7.0) * 4;
    n = n + ((int)(-3.9) == -3) * 8;
    n = n + (a > b) * 16;
    return n;
}
