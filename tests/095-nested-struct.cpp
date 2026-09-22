// A struct holding a struct and an array.
#include <cstdio>

struct Range {
    int lo, hi;
};

struct Record {
    const char* name;
    Range range;
    int samples[3];
};

int main() {
    Record r = {"temp", {-5, 40}, {1, 2, 3}};
    r.range.hi -= r.samples[2];
    std::printf("%s %d %d %d\n", r.name, r.range.lo, r.range.hi, r.samples[0] + r.samples[1]);
    return 0;
}
