// std::shared_ptr counts its owners; weak_ptr does not.
#include <cstdio>
#include <memory>

struct Node {
    ~Node() { std::printf("node gone\n"); }
};

int main() {
    std::weak_ptr<Node> w;
    {
        auto a = std::make_shared<Node>();
        auto b = a;
        w = a;
        std::printf("%ld %d\n", a.use_count(), w.expired());
    }
    std::printf("%d\n", w.expired());
    return 0;
}
