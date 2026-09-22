// std::map keeps its keys ordered.
#include <cstdio>
#include <map>
#include <string>

int main() {
    std::map<std::string, int> count;
    const char* words[] = {"pear", "apple", "fig", "apple", "pear", "apple"};
    for (const char* w : words) ++count[w];
    for (const auto& [k, v] : count) std::printf("%s=%d ", k.c_str(), v);
    std::printf("%zu %d\n", count.size(), count.contains("kiwi"));
    return 0;
}
