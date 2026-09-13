// §11.4.4 [special] -- the members a class gets when it does not write them,
// and the ones it writes instead.
struct Implicit {
    int a;
    double b;
};

struct Written {
    Written();
    Written(int);
    Written(const Written&);
    Written(Written&&);
    Written& operator=(const Written&);
    Written& operator=(Written&&);
    ~Written();
};

struct Defaulted {
    Defaulted() = default;
    Defaulted(const Defaulted&) = default;
    Defaulted& operator=(const Defaulted&) = default;
    ~Defaulted() = default;
};

struct Deleted {
    Deleted() = default;
    Deleted(const Deleted&) = delete;
    Deleted& operator=(const Deleted&) = delete;
};

struct Initializing {
    int a;
    int b;
    Initializing(int x, int y) : a(x), b(y) {}
    Initializing() : Initializing(0, 0) {}
};

struct Converting {
    explicit Converting(int);
    operator int() const;
    explicit operator bool() const;
};

int use() {
    Implicit i{1, 2.0};
    Implicit copy = i;
    Defaulted d;
    Defaulted dc = d;
    Initializing init(1, 2);
    (void)copy; (void)dc;
    return init.a + init.b;
}
