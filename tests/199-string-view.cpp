// std::string_view slices without copying.
#include <cstdio>
#include <string_view>

int count_words(std::string_view s) {
    int n = 0;
    bool in = false;
    for (char c : s) {
        if (c == ' ') in = false;
        else if (!in) {
            in = true;
            ++n;
        }
    }
    return n;
}

int main() {
    std::string_view s = "the quick brown fox";
    std::string_view mid = s.substr(4, 5);
    std::printf("%d %.*s %d %zu\n", count_words(s), (int)mid.size(), mid.data(), s.starts_with("the"),
                s.find("fox"));
    return 0;
}
