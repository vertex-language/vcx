// <algorithm>: sort, reverse, find, count, min and max.
#include <algorithm>
#include <cstdio>
#include <vector>

int main() {
    std::vector<int> v{8, 3, 5, 1, 9, 2, 5};
    std::sort(v.begin(), v.end());
    for (int x : v) std::printf("%d", x);
    std::reverse(v.begin(), v.end());
    auto it = std::find(v.begin(), v.end(), 3);
    std::printf(" %d %td %td %d %d\n", v[0], it - v.begin(), std::count(v.begin(), v.end(), 5),
                *std::min_element(v.begin(), v.end()), std::max(4, 11));
    return 0;
}
