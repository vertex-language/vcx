// Do operator(), operator[] and operator-> compile and execute correctly?
//
// §12.4.4 [over.call] -- a class with operator() is called like a
// function; §12.4.5 [over.sub] -- operator[] must be a member and may
// return a reference to assign through; §12.4.6 [over.ref] -- x->m with
// x a class is (x.operator->())->m, and if that is a class again its own
// operator-> is applied, until a pointer results.

struct Adder {
    int base;
    int operator()(int x) const { return base + x; }
    int operator()(int x, int y) const { return base + x + y; }
};

struct Table {
    int d[4];
    int& operator[](int i) { return d[i]; }
    const int& operator[](int i) const { return d[i]; }
};

struct Node {
    int v;
    Node* next;
};

struct Cursor {
    Node* at;
    Node* operator->() const { return at; }
    Node& operator*() const { return *at; }
};

// operator-> returning a class: applied again (§12.4.6/1).
struct Handle {
    Cursor c;
    Cursor operator->() const { return c; }
};

int sumTable(const Table& t) { return t[0] + t[1] + t[2] + t[3]; }

int main() {
    int r = 0;

    Adder add{10};
    if (add(5) == 15 && add(1, 2) == 13) r += 1;

    Table t{{1, 2, 3, 4}};
    t[2] = 30;
    t[0] += 100;
    if (t[2] == 30 && sumTable(t) == 137) r += 1;

    Node second{20, nullptr};
    Node first{10, &second};
    Cursor c{&first};
    if (c->v == 10 && c->next->v == 20 && (*c).v == 10) r += 1;

    c->v = 11;
    if (first.v == 11) r += 1;

    Handle h{Cursor{&second}};
    if (h->v == 20) r += 1;

    return r; // five checks
}
