// Do non-type template parameters compile and execute correctly?
//
// §13.2 [temp.param] -- non-type template parameters (integral, pointer,
// enum), template arguments as compile-time constants, expressions
// involving NTTPs, and dependent array bounds.

template <int N>
struct IntConstant {
    static int get() { return N; }
};

template <int A, int B>
struct Adder {
    static int sum() { return A + B; }
};

template <typename T, int N>
struct FixedArray {
    T data[N];
    int size() const { return N; }
    T get(int i) const { return data[i]; }
    void set(int i, T val) { data[i] = val; }
};

template <int N>
int scaleBy(int x) {
    return x * N;
}

int main() {
    int v1 = IntConstant<42>::get();
    int v2 = IntConstant<-7>::get();
    int sum = Adder<15, 27>::sum(); // 42

    FixedArray<int, 4> arr;
    arr.set(0, 10);
    arr.set(1, 20);
    arr.set(2, 30);
    arr.set(3, 40);

    int s = arr.size(); // 4
    int elem = arr.get(2); // 30

    int scaled = scaleBy<5>(6); // 30

    int r = 0;
    if (v1 == 42 && v2 == -7 && sum == 42) {
        r += 10;
    }
    if (s == 4 && elem == 30) {
        r += 20;
    }
    if (scaled == 30) {
        r += 12;
    }
    return r; // Expected: 42
}
