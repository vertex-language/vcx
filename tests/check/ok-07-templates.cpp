template <typename T>
struct Box {
    T value;
    T get() const { return value; }
    void set(T v) { value = v; }
};

template <typename T>
T identity(T v) { return v; }

template <typename T, typename U>
struct Pair {
    T first;
    U second;
};

template <int N>
struct Fixed {
    // §13.2/6 -- a non-type template-parameter is a value of its declared
    // type wherever it appears in the body, and typechecks as one. It has
    // no *value* until the arguments arrive.
    static constexpr int from_parameter = N;
    static constexpr int doubled = N * 2;
    static constexpr int size = 4;
    int buffer[4];
};

template <typename T> struct Trait { using type = T; static constexpr int always = 1; };
template <typename T> using Alias = Box<T>;

Box<int> bi;
Box<double> bd;
Pair<int, double> pid;
Alias<char> ac;
// §6.5.5 [class.qual] -- a static data member reached through the class,
// including through a template-id, is a constant expression.
static_assert(Fixed<4>::size == 4);
static_assert(Fixed<9>::size == 4);
static_assert(Trait<int>::always == 1);
// §13.4.3 [temp.arg.nontype] -- and one that does depend on the parameter
// is the argument, substituted at instantiation.
static_assert(Fixed<4>::from_parameter == 4);
static_assert(Fixed<9>::doubled == 18);
