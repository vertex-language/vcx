// Do float and double compute, convert and compare the way the machine does?
//
// §5.13.4 [lex.fcon] -- a literal is double, float with f, long double
// with l. §7.4 [expr.arith.conv] -- two floats stay float; a float and a
// double go to double; an integer with either goes to it. §7.3.11
// [conv.fpint] -- float to integer truncates toward zero, integer to
// float rounds. §7.6.9 -- the comparisons, NaN comparing false to all.

float half(float x) { return x / 2; }
double avg(double a, double b) { return (a + b) / 2; }
int trunc(double d) { return (int)d; }

int main() {
    int r = 0;

    // Single precision stays single.
    float f = 2.5f;
    float g = f * 4;                 // 10
    float h = half(f) + 0.25f;       // 1.5
    if (g == 10.0f && h == 1.5f && sizeof(f * 2) == sizeof(float)) r += 1;

    // Mixed: double wins; an int converts.
    double d = 3.7;
    double m = f + d;                // 6.2 as double
    if (m > 6.1 && m < 6.3 && sizeof(f + d) == sizeof(double)) r += 1;

    // Truncation toward zero, both signs.
    if (trunc(3.7) == 3 && trunc(-3.7) == -3 && (int)(f * 3) == 7 && (long long)(d * 1e9) == 3700000000LL) r += 1;

    // Integer to floating point, unsigned included.
    unsigned u = 7u;
    long long big = 1LL << 40;
    if (u / 2.0 == 3.5 && (double)big == 1099511627776.0 && (float)3 / 2 == 1.5f) r += 1;

    // Negation, division and a comparison chain.
    double neg = -d;
    if (neg < 0 && -neg == d && avg(1.0, 2.0) == 1.5 && 1.0 / 4 == 0.25) r += 1;

    // NaN is unordered: every comparison but != is false.
    double zero = 0.0;
    double nan = zero / zero;
    if (!(nan == nan) && !(nan < 1) && !(nan > 1) && nan != nan) r += 1;

    // Compound assignment and increments on floating types.
    float acc = 1.0f;
    acc += 0.5f;
    acc *= 4;
    double dd = 1.0;
    ++dd;
    dd--;
    if (acc == 6.0f && dd == 1.0) r += 1;

    return r; // seven checks
}
