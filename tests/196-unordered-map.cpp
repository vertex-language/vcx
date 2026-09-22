// std::unordered_map, printed in sorted key order so the output is fixed.
#include <algorithm>
#include <cstdio>
#include <unordered_map>
#include <vector>

int main() {
    std::unordered_map<int, int> squares;
    for (int i = 0; i < 10; ++i) squares[i * 7 % 10] = i * i;
    std::vector<int> keys;
    for (const auto& kv : squares) keys.push_back(kv.first);
    std::sort(keys.begin(), keys.end());
    for (int k : keys) std::printf("%d:%d ", k, squares[k]);
    std::printf("\n");
    return 0;
}
