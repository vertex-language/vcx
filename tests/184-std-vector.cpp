// std::vector: push_back, growth, erase, and iteration.
#include <cstdio>
#include <vector>

int main() {
    std::vector<int> v;
    for (int i = 0; i < 20; ++i) v.push_back(i * i);
    v.erase(v.begin() + 2, v.begin() + 5);
    v.insert(v.begin(), -1);
    long sum = 0;
    for (int x : v) sum += x;
    std::printf("%zu %d %d %ld\n", v.size(), v[0], v[3], sum);
    return 0;
}
