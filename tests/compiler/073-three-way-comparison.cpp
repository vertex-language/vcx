// Does <=> compare, and do the comparisons written in terms of it follow?
//
// §7.6.8 [expr.spaceship] -- on integers and pointers `a <=> b` is a
// std::strong_ordering, on floating point a std::partial_ordering, and
// the orderings are the classes <compare> declares, compared with a
// literal 0 through their hidden friends. §12.2.2.3/4 [over.match.oper]
// -- `a < b` with no operator< is `(a <=> b) < 0`, or `0 < (b <=> a)`
// reversed. §11.10.2-3 [class.compare.default] -- a comparison declared
// `= default` compares the bases and then the members in order, an array
// element by element, and a defaulted operator<=> returning auto has the
// weakest category among them.

#include <compare>

struct Pt {
    int x, y;
    auto operator<=>(const Pt&) const = default;
    bool operator==(const Pt&) const = default;
};

struct Ver {
    int major, minor;
    std::strong_ordering operator<=>(const Ver& o) const {
        if (auto c = major <=> o.major; c != 0) return c;
        return minor <=> o.minor;
    }
    bool operator==(const Ver& o) const { return major == o.major && minor == o.minor; }
};

// A member with its own <=>, and an array: both walked in order.
struct Line {
    Pt from;
    Pt to;
    int tags[2];
    auto operator<=>(const Line&) const = default;
    bool operator==(const Line&) const = default;
};

// A double member weakens the category to partial.
struct Mixed {
    int i;
    double d;
    auto operator<=>(const Mixed&) const = default;
};

// A base compared before the members.
struct Base { int b; auto operator<=>(const Base&) const = default; bool operator==(const Base&) const = default; };
struct Derived : Base { int d; auto operator<=>(const Derived&) const = default; bool operator==(const Derived&) const = default; };

int main() {
    int r = 0;

    // Built-in: the ordering classes and their comparisons with 0.
    if ((1 <=> 2) < 0 && (2 <=> 1) > 0 && (3 <=> 3) == 0 && !((1 <=> 2) >= 0)) r += 1;
    if ((1.5 <=> 2.5) < 0 && (2.5 <=> 2.5) == 0) r += 1;
    int arr[2];
    if ((&arr[0] <=> &arr[1]) < 0 && (&arr[1] <=> &arr[0]) > 0) r += 1;

    // Defaulted <=> and ==, and every relational rewritten through them.
    Pt a{1, 2}, b{1, 3}, c{2, 0};
    if (a < b && b > a && a <= a && a >= a && !(a > b) && a != b && a == a && c > b) r += 1;

    // A user-written <=>: the same rewrites.
    Ver v1{1, 9}, v2{2, 0};
    if (v1 < v2 && !(v2 < v1) && v1 <= v1 && v2 >= v1 && v1 != v2) r += 1;

    // Nested defaulted members and an array member, in declaration order.
    Line l1{{0, 0}, {1, 1}, {5, 5}};
    Line l2{{0, 0}, {1, 1}, {5, 6}};
    Line l3{{0, 0}, {2, 0}, {0, 0}};
    if (l1 < l2 && l1 < l3 && l2 < l3 && l1 == l1 && !(l1 == l2)) r += 1;

    // partial_ordering from a double member; NaN is unordered on every side.
    Mixed m1{1, 1.0}, m2{1, 2.0};
    double zero = 0.0;
    double nan = zero / zero;
    Mixed m3{1, nan};
    if (m1 < m2 && !(m1 < m3) && !(m1 > m3) && !(m1 == m3) && (m1 <=> m3) != 0) r += 1;

    // The base first.
    Derived d1{{1}, 9}, d2{{2}, 0};
    if (d1 < d2 && d2 > d1 && d1 == d1 && d1 != d2) r += 1;

    // The ordering is a value: passed along and read later.
    std::strong_ordering o = a <=> c;
    if (o < 0 && o != 0 && !(o > 0)) r += 1;

    return r; // nine checks
}
