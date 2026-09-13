// §11.8 [class.access] -- who may name a member, and the three ways in.
class Box {
public:
    int open;
    int reach_private() { return shut; }
protected:
    int guarded = 1;
private:
    int shut = 2;

    // §11.8.4 [class.friend] -- friendship is granted by the class, to a
    // function by its signature and to a class by its name.
    friend int peek(const Box&);
    friend class Inspector;
    friend struct Auditor;
};

// A friend is characteristically *outside* every class, which is the case
// friendship exists for.
int peek(const Box& b) { return b.shut; }

struct Inspector {
    int look(const Box& b) { return b.shut + b.guarded; }
};

struct Auditor {
    int count(const Box& b) { return b.shut; }
};

// §11.8.3 [class.access.base] -- a derived class reaches protected members
// and not private ones.
struct Heir : Box {
    int reach_protected() { return guarded; }
};

int use(Box& b) { return b.open + b.reach_private() + peek(b); }
