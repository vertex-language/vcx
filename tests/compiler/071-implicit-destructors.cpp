// Does a class with no destructor of its own destroy its members and bases?
//
// §11.4.7/4 [class.dtor] -- a class that declares no destructor has one
// implicitly defined when it is odr-used; /9 -- after a destructor's body
// the members are destroyed in the reverse order of their declaration,
// each array element likewise, and then the bases in the reverse order
// of construction. The order is logged, not just the count. §11.9.3/7
// with §9.4.5 -- an array member takes a braced list in a
// mem-initializer, one element per item.

long log = 0;
void note(int d) { log = log * 10 + d; }

struct Tag {
    int id;
    Tag(int i) : id(i) {}
    ~Tag() { note(id); }
};

// No destructor declared: the implicit one destroys b, then a.
struct Two {
    Tag a;
    Tag b;
    Two() : a(1), b(2) {}
};

// An array member: elements last first, then the member before it.
struct Row {
    Tag first;
    Tag cells[3];
    Row() : first(4), cells{5, 6, 7} {}
};

// A base with an implicit destructor, and a member after it: the member
// goes first, then the base's members.
struct Derived : Two {
    Tag extra;
    Derived() : extra(3) {}
};

// A user-written destructor still destroys the members after its body.
struct Loud {
    Tag t;
    Loud() : t(9) {}
    ~Loud() { note(8); }
};

// Nothing to destroy: no destructor is needed at all.
struct Plain { int x; };

int main() {
    int r = 0;

    log = 0;
    { Two t; }
    if (log == 21) r += 1;

    log = 0;
    { Row w; }
    if (log == 7654) r += 1;

    log = 0;
    { Derived d; }
    if (log == 321) r += 1;

    log = 0;
    { Loud l; }
    if (log == 89) r += 1;

    log = 0;
    { Plain p{1}; Two* heap = new Two; delete heap; }
    if (log == 21) r += 1;

    return r; // five checks
}
