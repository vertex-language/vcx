// An empty base takes no space in its derived class.
#include <cstdio>

struct Empty {};
struct Tag {
    void mark() {}
};

struct Holder : Empty {
    int v;
};

struct WithMember {
    [[no_unique_address]] Empty e;
    int v;
};

int main() {
    std::printf("%zu %zu %zu %zu\n", sizeof(Empty), sizeof(Holder), sizeof(WithMember), sizeof(Tag));
    return 0;
}
