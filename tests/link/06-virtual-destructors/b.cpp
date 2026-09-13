// The other half of 06: see a.cpp.

extern int destroyed;

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

Derived::~Derived() { destroyed += 10; }

Base* make_base(int id) { return new Base(id); }
Base* make_derived(int id) { return new Derived(id); }
void destroy(Base* p) { delete p; }
