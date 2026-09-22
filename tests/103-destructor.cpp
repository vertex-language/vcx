// Destructors run at the end of scope, in reverse order of construction.
#include <cstdio>

struct Noisy {
    int id;
    explicit Noisy(int i) : id(i) { std::printf("ctor %d\n", id); }
    ~Noisy() { std::printf("dtor %d\n", id); }
};

int main() {
    Noisy a(1);
    {
        Noisy b(2);
        Noisy c(3);
    }
    Noisy d(4);
    return 0;
}
