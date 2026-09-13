// The layout every other layout is built out of: a member's offset is its
// type's alignment rounded up from wherever the previous one ended, and a
// class's size is that rounded up to its own alignment.
//
// Nothing here states a number. The numbers are what `cl` says, read out of
// `/d1reportSingleClassLayout` and compared against what vcx computed for
// the same class -- which is the only way to settle a layout question, and
// the reason this directory exists.

struct Empty {};

struct OneChar { char c; };
struct OneShort { short s; };
struct OneInt { int i; };
struct OneLongLong { long long l; };
struct OneFloat { float f; };
struct OneDouble { double d; };
struct OnePointer { void* p; };

// Padding between members, and the tail padding that rounds the whole thing
// up to its alignment.
struct CharThenInt { char c; int i; };
struct IntThenChar { int i; char c; };
struct CharIntChar { char a; int i; char b; };
struct ShortThenLongLong { short s; long long l; };
struct Interleaved { char a; double d; char b; int i; char c; };

// Ordering is the program's, not the compiler's: the same members in a
// different order are a different size, which is the first thing anyone
// discovers about layout and the first thing a compiler must not decide for
// itself.
struct Wasteful { char a; int i; char b; short s; };
struct Tight { int i; short s; char a; char b; };

// Arrays and nested classes contribute their own size and alignment.
struct WithArray { char c; int a[4]; };
struct WithNested { char c; CharThenInt n; char d; };
struct ArrayOfNested { CharThenInt n[3]; };

// A pointer member is the target's pointer, not the host's, which is the
// whole reason the model is asked rather than the machine.
struct Pointers { char c; void* p; int i; };
struct PointerToMember { char c; int OneInt::* p; };
