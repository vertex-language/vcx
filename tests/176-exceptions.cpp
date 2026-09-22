// throw and catch an int, and a class by reference.
#include <cstdio>
#include <initializer_list>

struct Error {
    int code;
};

int risky(int v) {
    if (v < 0) throw Error{-v};
    if (v == 0) throw 42;
    return v * 2;
}

int main() {
    for (int v : {3, -7, 0}) {
        try {
            std::printf("ok %d\n", risky(v));
        } catch (const Error& e) {
            std::printf("error %d\n", e.code);
        } catch (int n) {
            std::printf("int %d\n", n);
        }
    }
    return 0;
}
