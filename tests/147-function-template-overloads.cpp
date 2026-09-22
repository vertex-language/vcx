// Overload resolution between templates and non-templates.
#include <cstdio>

template <typename T>
const char* pick(T) { return "template"; }
template <typename T>
const char* pick(T*) { return "pointer template"; }
const char* pick(int) { return "int"; }

int main() {
    int i = 0;
    std::printf("%s | %s | %s | %s\n", pick(1), pick(1.0), pick(&i), pick<int>(1));
    return 0;
}
