// Do constrained overloads and the implicit object parameter choose?
//
// §13.5.4 -- two member functions of one signature told apart by their
// requires-clauses: for It<int> the constrained one is a candidate and
// wins, for It<char> it is not a candidate at all. §12.2.2.1 -- a
// member call has an implied object argument, and `get() const` against
// `get()` is decided by whether the object is const: the same two
// overloads that overload resolution over the explicit arguments alone
// found tied.

template <class T> concept Big = sizeof(T) > 2;

template <class T> struct It {
    int v;
    int step() const noexcept { return 1; }
    int step() const noexcept requires Big<T> { return 2; }
    int get() const { return v; }
    int get() { return v * 10; }
};

template <class T> int f(T) { return 1; }
template <class T> requires Big<T> int f(T) { return 2; }

int main() {
    It<int> a{3};
    It<char> b{4};
    const It<int>& ca = a;
    return a.step() * 100 + b.step() * 10 + f('c') + f(1) * 1000 + ca.get() + a.get() + b.get();
    // 200 + 10 + 1 + 2000 + 3 + 30 + 40 = 2284 -> exit 2284 & 0xff = 236
}
