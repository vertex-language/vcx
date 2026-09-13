template <typename T> concept Anything = true;
template <typename T> concept Sized = sizeof(T) > 0;
template <typename T> concept Both = Anything<T> && Sized<T>;

template <Anything T> void constrained_param(T);
template <typename T> requires Sized<T> void requires_clause(T);
template <typename T> void trailing(T) requires Both<T>;
void abbreviated(Anything auto);

template <typename T> concept HasPlus = requires(T a, T b) {
    a + b;
    { a + b } -> Anything;
};

struct Addable { };
int operator+(Addable, Addable);

// §13.7.9/5 -- a concept-id is a prvalue of type bool, so a static_assert
// is the natural way to check one, and deciding it means binding the
// parameters to the arguments and evaluating the constraint.
static_assert(Anything<int>);
static_assert(Sized<int>);
static_assert(Both<int>);
static_assert(Sized<Addable>);

// A constraint that discriminates has to answer differently for different
// arguments, which is the only thing that distinguishes a working concept
// from one that is satisfied by everything.
template <typename T> concept Wide = sizeof(T) >= 8;
static_assert(Wide<long long>);
static_assert(!Wide<char>);

// §7.5.7 -- and a requires-expression is decided by forming each
// requirement with T bound and asking whether it is well-formed. A struct
// with no operator+ is what tells a working HasPlus from one satisfied by
// everything; tests/eval/11 is the corpus for that.
struct NoPlus { };
static_assert(HasPlus<int>);
static_assert(HasPlus<Addable>);
static_assert(!HasPlus<NoPlus>);
