// std::string: concatenation, find, substr, comparison.
#include <cstdio>
#include <string>

int main() {
    std::string s = "hello";
    s += ", ";
    s += std::string("world");
    std::string word = s.substr(7, 5);
    auto at = s.find("lo");
    std::printf("%s %zu %s %zu %d\n", s.c_str(), s.size(), word.c_str(), at, word < s);
    std::string num = std::to_string(12345);
    std::printf("%s %d\n", num.c_str(), std::stoi("-99") * 2);
    return 0;
}
