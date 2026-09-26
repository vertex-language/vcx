// A namespace-scope floating object initialized from an integer constant
// holds that value as a float: `static double scale = 1;` is 1.0, not the
// integer 1 written into an f64.
#include <cstdio>
static double scale = 1;
double half = 1 / 2.0;
float three = 3;
double big = 1LL << 40;
double sum = 1 + 2;
static double dyn = scale * 2;
int main() { std::printf("%g %g %g %g %g %g\n", scale, half, (double)three, big, sum, dyn); return 0; }
