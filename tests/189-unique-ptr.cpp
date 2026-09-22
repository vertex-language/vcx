// std::unique_ptr owns and releases; ownership moves.
#include <cstdio>
#include <memory>

struct Res {
    int id;
    Res(int i) : id(i) { std::printf("acquire %d\n", id); }
    ~Res() { std::printf("release %d\n", id); }
};

std::unique_ptr<Res> make(int id) { return std::make_unique<Res>(id); }

int main() {
    auto a = make(1);
    std::unique_ptr<Res> b = std::move(a);
    std::printf("%d %d\n", a == nullptr, b->id);
    b.reset(new Res(2));
    auto arr = std::make_unique<int[]>(3);
    arr[2] = 9;
    std::printf("%d\n", arr[2]);
    return 0;
}
