// A move constructor steals from its source.
#include <cstdio>
#include <utility>

struct Buffer {
    int* data;
    int size;
    explicit Buffer(int n) : data(new int[n]), size(n) {
        for (int i = 0; i < n; ++i) data[i] = i;
    }
    Buffer(Buffer&& o) noexcept : data(o.data), size(o.size) {
        o.data = nullptr;
        o.size = 0;
    }
    ~Buffer() { delete[] data; }
};

int main() {
    Buffer a(5);
    Buffer b(std::move(a));
    std::printf("%d %d %d %d\n", a.size, a.data == nullptr, b.size, b.data[4]);
    return 0;
}
