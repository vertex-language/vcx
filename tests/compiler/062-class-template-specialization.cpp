// Do explicit and partial class template specializations compile and execute correctly?
//
// §13.7.6 [temp.spec] -- primary templates, full specializations,
// partial specializations, member variables, and member functions
// in specializations.

template <typename T>
struct TypeTraits {
    static int kind() { return 1; }
    static int size() { return (int)sizeof(T); }
};

template <>
struct TypeTraits<int> {
    static int kind() { return 2; }
    static int size() { return 4; }
};

template <>
struct TypeTraits<double> {
    static int kind() { return 3; }
    static int size() { return 8; }
};

template <typename T>
struct TypeTraits<T*> {
    static int kind() { return 4; }
    static int size() { return (int)sizeof(T*); }
};

template <typename T>
struct Container {
    T val;
    T get() const { return val; }
};

template <>
struct Container<bool> {
    int bits;
    bool get() const { return bits != 0; }
    void flip() { bits = (bits != 0 ? 0 : 1); }
};

int main() {
    int k_char = TypeTraits<char>::kind();
    int k_int = TypeTraits<int>::kind();
    int k_dbl = TypeTraits<double>::kind();
    int k_ptr = TypeTraits<int*>::kind();
    int k_cptr = TypeTraits<char*>::kind();

    Container<int> ci{100};
    Container<bool> cb{1};
    cb.flip();

    int r = 0;
    if (k_char == 1 && k_int == 2 && k_dbl == 3 && k_ptr == 4 && k_cptr == 4) {
        r += 10;
    }
    if (ci.get() == 100 && !cb.get()) {
        r += 20;
    }
    return r; // Expected: 30
}
