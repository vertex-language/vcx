// A derived override calling the base version explicitly.
#include <cstdio>

struct Logger {
    virtual void log(int v) { std::printf("base %d\n", v); }
    virtual ~Logger() = default;
};

struct Prefixed : Logger {
    void log(int v) override {
        std::printf("prefix ");
        Logger::log(v * 10);
    }
};

int main() {
    Prefixed p;
    Logger& l = p;
    l.log(4);
    return 0;
}
