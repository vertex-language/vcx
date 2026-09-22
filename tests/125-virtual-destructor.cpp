// Deleting through a base pointer runs the derived destructor.
#include <cstdio>

struct Base {
    virtual ~Base() { std::printf("~Base\n"); }
};

struct Derived : Base {
    int* data = new int[4];
    ~Derived() override {
        delete[] data;
        std::printf("~Derived\n");
    }
};

int main() {
    Base* b = new Derived;
    delete b;
    return 0;
}
