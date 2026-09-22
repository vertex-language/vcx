// std::format with widths, alignment, bases and argument indices.
#include <cstdio>
#include <format>
#include <string>

int main() {
    std::string a = std::format("{} + {} = {}", 2, 3, 2 + 3);
    std::string b = std::format("[{:>6}] [{:<6}] [{:^6}]", "r", "l", "c");
    std::string c = std::format("{0:#x} {1:08b} {2:+d} {1}", 255, 5, 42);
    std::printf("%s\n%s\n%s\n", a.c_str(), b.c_str(), c.c_str());
    return 0;
}
