// Do the two compilers agree on a template's specializations?
//
// Both sides see the same template and each instantiates what it uses, so
// the same specialization can be defined in both objects -- `Box<int>` and
// its members here, and again in b -- and the linker keeps one of each,
// which only works if both compilers gave it the same name and the same
// layout. One side's `twice<int>` is called from the other by name alone.

template <typename T> T twice(T x) { return x + x; }

template <typename T> struct Box {
    T v;
    Box(T x) : v(x) {}
    T get() const { return v; }
    void set(T x) { v = x; }
};

int use_box(Box<int> &b);       // defined in b: reads and bumps the box
double twice_double(double x);  // defined in b: calls twice<double>
int twice_int_from_b(int x);    // defined in b: calls twice<int>, which a also instantiates

int main() {
    Box<int> b(5);
    int r = use_box(b);          // 5, b becomes 6
    return r + b.get() + twice(10) + (int)twice_double(1.5) + twice_int_from_b(4) - 30;   // 5 + 6 + 20 + 3 + 8 - 30 = 12
}
