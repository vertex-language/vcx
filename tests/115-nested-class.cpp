// A class nested inside another, and an enum inside a class.
#include <cstdio>

class List {
public:
    struct Node {
        int value;
        Node* next;
    };
    enum Order { Forward, Backward };

    int sum(Node* n) const { return n ? n->value + sum(n->next) : 0; }
};

int main() {
    List::Node c{3, nullptr}, b{2, &c}, a{1, &b};
    List l;
    std::printf("%d %d\n", l.sum(&a), List::Backward);
    return 0;
}
