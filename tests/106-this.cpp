// this, and member functions that return *this to chain.
#include <cstdio>

class Builder {
public:
    Builder& add(int v) {
        total_ += v;
        return *this;
    }
    Builder& twice() {
        this->total_ *= 2;
        return *this;
    }
    int total() const { return total_; }

private:
    int total_ = 0;
};

int main() {
    Builder b;
    std::printf("%d\n", b.add(3).add(4).twice().add(1).total());
    return 0;
}
