// Overloads chosen by argument type.
#include <cstdio>

const char* kind(int) { return "int"; }
const char* kind(double) { return "double"; }
const char* kind(const char*) { return "string"; }
const char* kind(char) { return "char"; }

int main() {
    std::printf("%s %s %s %s\n", kind(1), kind(1.0), kind("x"), kind('x'));
    return 0;
}
