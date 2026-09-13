// The evaluator's answers about types, which are the answers the rest of
// the compiler reads back: an array bound, a struct's layout, an enumerator.
// A wrong one here is a wrong object file rather than a wrong number.

// §7.6.2.5 [expr.sizeof] -- sizeof is an integer constant expression, and
// its value is the target's, not the host's.
static_assert(sizeof(char) == 1);
static_assert(sizeof(signed char) == 1);
static_assert(sizeof(unsigned char) == 1);
static_assert(sizeof(short) == 2);
static_assert(sizeof(int) == 4);
static_assert(sizeof(long long) == 8);
static_assert(sizeof(float) == 4);
static_assert(sizeof(double) == 8);
static_assert(sizeof(bool) == 1);
static_assert(sizeof(void*) == 8);
static_assert(sizeof(int*) == 8);
static_assert(sizeof(char16_t) == 2);
static_assert(sizeof(char32_t) == 4);

static_assert(sizeof(int[4]) == 16);
static_assert(sizeof(int[2][3]) == 24);

// §6.8.2/4 -- the relations between the integer types, which hold on every
// conforming implementation.
static_assert(sizeof(char) <= sizeof(short));
static_assert(sizeof(short) <= sizeof(int));
static_assert(sizeof(int) <= sizeof(long));
static_assert(sizeof(long) <= sizeof(long long));

// §7.6.2.6 [expr.alignof].
static_assert(alignof(char) == 1);
static_assert(alignof(short) == 2);
static_assert(alignof(int) == 4);
static_assert(alignof(double) == 8);

// §11.4/17 -- a class's size includes its padding, and its alignment is
// that of its most-aligned member.
struct Empty {};
struct OneInt { int a; };
struct TwoInts { int a; int b; };
struct Padded { char c; int i; };
struct Nested { TwoInts t; char c; };

static_assert(sizeof(Empty) == 1);
static_assert(sizeof(OneInt) == 4);
static_assert(sizeof(TwoInts) == 8);
static_assert(sizeof(Padded) == 8);
static_assert(alignof(Padded) == 4);
static_assert(sizeof(Nested) == 12);

// §9.7.1 [dcl.enum] -- an enumerator's value is the previous one plus one
// unless it says otherwise.
enum Plain { A, B, C = 10, D, E = -1, F };
static_assert(A == 0);
static_assert(B == 1);
static_assert(C == 10);
static_assert(D == 11);
static_assert(E == -1);
static_assert(F == 0);

enum class Scoped : int { X = 5, Y };
static_assert(static_cast<int>(Scoped::X) == 5);
static_assert(static_cast<int>(Scoped::Y) == 6);

// §7.3.10 [conv.fpint] and §7.3.9 [conv.integral] -- a conversion in a
// constant expression is evaluated, and truncation is toward zero.
static_assert(static_cast<int>(3.9) == 3);
static_assert(static_cast<int>(-3.9) == -3);
static_assert(static_cast<double>(7) / 2 == 3.5);
static_assert(static_cast<char>(321) == 65);
static_assert(static_cast<bool>(2));
static_assert(!static_cast<bool>(0));

// §7.7/1 -- __builtin_is_constant_evaluated is true here by construction.
static_assert(__builtin_is_constant_evaluated());
