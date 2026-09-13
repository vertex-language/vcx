// §13.7.9 [temp.concept] and the type traits -- the questions a program asks
// the compiler about its own type system, answered here rather than by a
// library, because no library can answer them.
//
// A trait or a constraint that defaults to *satisfied* is the one kind of
// wrong answer that produces no diagnostic anywhere: a concept meant to
// reject a type quietly accepts it, the wrong overload is chosen, and the
// program compiles. So every assertion below is written to fail if the
// answer were a default, by asserting both a true case and a false one.

// §13.7.9/5 -- a concept-id is a prvalue of type bool.
template <typename T> concept Always = true;
template <typename T> concept Never = false;
static_assert(Always<int>);
static_assert(!Never<int>);

// A constraint that reads the argument, and therefore discriminates.
template <typename T> concept Wide = sizeof(T) >= 8;
static_assert(Wide<long long>);
static_assert(Wide<double>);
static_assert(!Wide<char>);
static_assert(!Wide<int>);

template <typename T> concept Tiny = sizeof(T) == 1;
static_assert(Tiny<char>);
static_assert(Tiny<bool>);
static_assert(!Tiny<int>);

// §13.5.2 [temp.constr.op] -- conjunction and disjunction over concepts,
// which is how constraints are built out of each other.
template <typename T> concept WideOrTiny = Wide<T> || Tiny<T>;
template <typename T> concept WideAndTiny = Wide<T> && Tiny<T>;
static_assert(WideOrTiny<char>);
static_assert(WideOrTiny<double>);
static_assert(!WideOrTiny<int>);
static_assert(!WideAndTiny<char>);
static_assert(!WideAndTiny<double>);

template <typename T> concept NotWide = !Wide<T>;
static_assert(NotWide<int>);
static_assert(!NotWide<double>);

// A concept over more than one parameter.
template <typename T, typename U> concept SameSize = sizeof(T) == sizeof(U);
static_assert(SameSize<int, float>);
static_assert(!SameSize<char, int>);

// The traits. §7.7 makes each of these a constant expression, and the
// compiler is the only thing that can evaluate one.
struct Base {};
struct Derived : Base {};
struct Unrelated {};
union Union { int i; float f; };
enum Colour { Red };

static_assert(__is_same(int, int));
static_assert(!__is_same(int, long));
static_assert(!__is_same(int, unsigned int));
static_assert(__is_same(Base, Base));
static_assert(!__is_same(Base, Derived));

static_assert(__is_integral(int));
static_assert(__is_integral(char));
static_assert(!__is_integral(double));
static_assert(!__is_integral(Base));

static_assert(__is_floating_point(double));
static_assert(__is_floating_point(float));
static_assert(!__is_floating_point(int));

static_assert(__is_arithmetic(int));
static_assert(__is_arithmetic(double));
static_assert(!__is_arithmetic(Base));

static_assert(__is_pointer(int*));
static_assert(__is_pointer(Base*));
static_assert(!__is_pointer(int));

static_assert(__is_reference(int&));
static_assert(__is_lvalue_reference(int&));
static_assert(!__is_lvalue_reference(int&&));
static_assert(__is_rvalue_reference(int&&));
static_assert(!__is_reference(int));

static_assert(__is_const(const int));
static_assert(!__is_const(int));

static_assert(__is_array(int[4]));
static_assert(!__is_array(int*));

static_assert(__is_void(void));
static_assert(!__is_void(int));

static_assert(__is_class(Base));
static_assert(!__is_class(Union));
static_assert(!__is_class(int));
static_assert(__is_union(Union));
static_assert(!__is_union(Base));

static_assert(__is_enum(Colour));
static_assert(!__is_enum(Base));

static_assert(__is_base_of(Base, Derived));
static_assert(__is_base_of(Base, Base));
static_assert(!__is_base_of(Derived, Base));
static_assert(!__is_base_of(Base, Unrelated));

// A concept written over a trait, which is where the two meet in practice.
template <typename T> concept Numeric = __is_arithmetic(T);
static_assert(Numeric<int>);
static_assert(Numeric<double>);
static_assert(!Numeric<Base>);

template <typename T> concept DerivesFromBase = __is_base_of(Base, T);
static_assert(DerivesFromBase<Derived>);
static_assert(!DerivesFromBase<Unrelated>);
