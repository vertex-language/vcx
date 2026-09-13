// Do new and delete reach the allocator and the constructor?
//
// §7.6.2.8 -- `new T(args)` calls the allocation function, then constructs
// in the storage it returned; §7.6.2.9 -- `delete p` destroys and then
// deallocates. The allocation functions are the library's, which for this
// corpus means the C runtime's `operator new` under its mangled name, so
// this is the second import from outside the program and the first one
// whose name is not C's.

int live;

struct Node {
    int value;
    Node *next;
    Node(int v, Node *n) : value(v), next(n) { live++; }
    ~Node() { live--; }
};

int main() {
    Node *list = nullptr;
    for (int i = 1; i <= 4; i++) {
        list = new Node(i, list);
    }
    int sum = 0;
    while (list) {
        Node *n = list;
        sum += n->value;
        list = n->next;
        delete n;
    }
    int *p = new int(5);
    int v = *p;
    delete p;
    return sum * 10 + live + v;   // 100 + 0 + 5
}
