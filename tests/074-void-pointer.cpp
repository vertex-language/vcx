// void* holds any object pointer and converts back.
#include <cstdio>

int read_int(const void* p) { return *static_cast<const int*>(p); }

int main() {
    int v = 1234;
    void* p = &v;
    double d = 2.5;
    const void* q = &d;
    std::printf("%d %g\n", read_int(p), *static_cast<const double*>(q));
    return 0;
}
