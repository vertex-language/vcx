// inline variables and static constexpr data members.
#include <cstdio>

inline int shared_counter = 100;

struct Limits {
    static constexpr int max = 64;
    static constexpr double ratio = 1.5;
    static inline int instances = 0;
    static constexpr const char* name = "limits";
    Limits() { ++instances; }
};

int main() {
    Limits a, b;
    const int* p = &Limits::max;
    ++shared_counter;
    std::printf("%d %g %d %s %d %d\n", Limits::max, Limits::ratio, Limits::instances, Limits::name, *p,
                shared_counter);
    return 0;
}
