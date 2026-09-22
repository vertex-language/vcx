// A class with private data and public member functions.
#include <cstdio>

class Counter {
public:
    void add(int n) { value_ += n; }
    int value() const { return value_; }

private:
    int value_ = 0;
};

int main() {
    Counter c;
    c.add(5);
    c.add(7);
    std::printf("%d\n", c.value());
    return 0;
}
