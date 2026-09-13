// Does a specialization get the name cl gives it?
//
// A function template's specialization is `?$name@args@`, the name and its
// arguments as one back-reference unit with a back-reference context of
// its own; a class template's is the same shape in scope position, so a
// member of `Box<int>` is `?get@?$Box@H@@`. The arguments are written as
// types are elsewhere, except that a cv-qualified one carries `$$C`.
// Every specialization is a COMDAT, emitted where it is used.

template <typename T> T twice(T x) { return x + x; }
template <typename T> struct Box {
    T v;
    Box(T x) : v(x) {}
    T get() const { return v; }
    void set(const T &x) { v = x; }
};
template <typename T> Box<T> boxed(T x) { return Box<T>(x); }
template <typename A, typename B> A first(A a, B) { return a; }

struct Point { int x, y; };

int main() {
    Box<int> bi(1);
    Box<double> bd(2.5);
    Box<Point> bp(Point{1, 2});
    bi.set(3);
    bd.set(1.5);
    return twice(3) + (int)twice(2.5) + bi.get() + (int)bd.get() + bp.get().x
        + boxed(4).get() + first(1, 'c') + (int)first(2.0, bi);
}
