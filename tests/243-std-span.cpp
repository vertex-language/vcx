// std::span views an array, a vector, or part of either, without owning it.
#include <cstdio>
#include <span>
#include <vector>

int sum(std::span<const int> s) {
    int t = 0;
    for (int v : s) t += v;
    return t;
}

void zero_tail(std::span<int> s, std::size_t n) {
    for (int& v : s.last(n)) v = 0;
}

int main() {
    int arr[] = {1, 2, 3, 4, 5, 6};
    std::vector<int> vec{10, 20, 30};
    zero_tail(arr, 2);
    std::span<int, 6> fixed(arr);
    std::printf("%d %d %d %zu %zu\n", sum(arr), sum(vec), sum(fixed.subspan(1, 3)), fixed.size(),
                sizeof(std::span<int, 6>) < sizeof(std::span<int>));
    return 0;
}
