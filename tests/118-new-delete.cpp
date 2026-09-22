// new and delete for single objects and arrays.
#include <cstdio>

struct Node {
    int v;
    Node(int x) : v(x) { std::printf("new %d\n", v); }
    ~Node() { std::printf("delete %d\n", v); }
};

int main() {
    Node* n = new Node(7);
    int* arr = new int[5]{1, 2, 3, 4, 5};
    int s = 0;
    for (int i = 0; i < 5; ++i) s += arr[i];
    delete n;
    delete[] arr;
    Node* many = new Node[2]{Node(1), Node(2)};
    delete[] many;
    std::printf("%d\n", s);
    return 0;
}
