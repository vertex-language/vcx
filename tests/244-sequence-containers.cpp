// std::list, std::deque and std::forward_list.
#include <cstdio>
#include <deque>
#include <forward_list>
#include <list>

int main() {
    std::list<int> l{3, 1, 4, 1, 5};
    l.push_front(9);
    l.remove(1);
    l.sort();
    for (int v : l) std::printf("%d", v);
    std::deque<int> d;
    for (int i = 0; i < 5; ++i) {
        d.push_back(i);
        d.push_front(-i);
    }
    d.pop_back();
    std::forward_list<int> f{7, 8, 9};
    f.push_front(6);
    f.reverse();
    std::printf(" | %zu %d %d | %d\n", d.size(), d.front(), d.back(), f.front());
    return 0;
}
