// Does `delete` through a base pointer reach the other side's destructor?
//
// §7.6.2.9/3 -- with a virtual destructor the static type may be a base of
// the dynamic type, and the destructor called is the dynamic type's. Under
// the Microsoft convention the table slot holds the deleting destructor
// (`??_E`), which destroys and, when its flag says so, frees; the caller
// that deletes never learns the object's size. One side constructs, the
// other deletes, and the count of destructors run says whether the slot
// each compiler filled is the one the other called through.

int destroyed = 0;

struct Base {
    int id;
    Base(int i) : id(i) {}
    virtual ~Base();
    virtual int kind() const { return 1; }
};

struct Derived : Base {
    int extra;
    Derived(int i) : Base(i), extra(i * 10) {}
    ~Derived() override;
    int kind() const override { return 2; }
};

Base::~Base() { destroyed += 1; }

// b.cpp
Base* make_base(int id);
Base* make_derived(int id);
void destroy(Base* p);

int main() {
    int r = 0;

    // Built here, deleted there.
    Base* d = new Derived(3);
    destroy(d);
    if (destroyed == 11) r += 1;

    // Built there, deleted here.
    destroyed = 0;
    Base* p = make_derived(4);
    int k = p->kind();
    delete p;
    if (k == 2 && destroyed == 11) r += 1;

    destroyed = 0;
    Base* q = make_base(5);
    delete q;
    if (destroyed == 1) r += 1;

    return r; // three checks
}
