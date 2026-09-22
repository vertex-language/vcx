// A full specialization of a class template.
#include <cstdio>

template <typename T>
struct Name {
    static const char* get() { return "unknown"; }
};
template <>
struct Name<int> {
    static const char* get() { return "int"; }
};
template <>
struct Name<double> {
    static const char* get() { return "double"; }
};

int main() {
    std::printf("%s %s %s\n", Name<int>::get(), Name<double>::get(), Name<char>::get());
    return 0;
}
