// A C-style variadic function with va_list.
#include <cstdarg>
#include <cstdio>

int sum(int count, ...) {
    va_list ap;
    va_start(ap, count);
    int s = 0;
    for (int i = 0; i < count; ++i) s += va_arg(ap, int);
    va_end(ap);
    return s;
}

double mean(int count, ...) {
    va_list ap;
    va_start(ap, count);
    double s = 0;
    for (int i = 0; i < count; ++i) s += va_arg(ap, double);
    va_end(ap);
    return s / count;
}

int main() {
    std::printf("%d %g\n", sum(5, 1, 2, 3, 4, 5), mean(3, 1.5, 2.5, 5.0));
    return 0;
}
