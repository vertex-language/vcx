// A lambda called where it is written, to initialize a const.
#include <cstdio>

int main(int argc, char**) {
    const int mode = [&] {
        if (argc > 5) return 3;
        int m = 0;
        for (int i = 0; i < 4; ++i) m += i;
        return m;
    }();
    const char* name = [](int m) { return m == 6 ? "six" : "other"; }(mode);
    std::printf("%d %s\n", mode, name);
    return 0;
}
