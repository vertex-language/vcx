// std::set: unique, ordered, with lower_bound.
#include <cstdio>
#include <set>

int main() {
    std::set<int> s{9, 1, 5, 1, 9, 3, 7};
    s.erase(5);
    s.insert(4);
    for (int v : s) std::printf("%d ", v);
    std::printf("| %d %zu\n", *s.lower_bound(6), s.size());
    return 0;
}
