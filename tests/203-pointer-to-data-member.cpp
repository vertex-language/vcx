// A pointer to a data member selects a field of any object of the class.
#include <cstdio>

struct Stats {
    int hp, mp, xp;
};

int total(const Stats* s, int n, int Stats::*field) {
    int t = 0;
    for (int i = 0; i < n; ++i) t += s[i].*field;
    return t;
}

int main() {
    Stats party[] = {{10, 5, 100}, {20, 0, 50}, {15, 8, 75}};
    int Stats::*f = &Stats::mp;
    Stats* p = &party[2];
    p->*f += 100;
    std::printf("%d %d %d\n", total(party, 3, &Stats::hp), total(party, 3, f), total(party, 3, &Stats::xp));
    return 0;
}
