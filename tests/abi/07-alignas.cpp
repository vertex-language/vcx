// §9.12.2 [dcl.align] -- alignas raises the alignment of what it
// appertains to: a member, whose offset then rounds up to it and whose
// class inherits it; or a class, on its head, whose size rounds up to it.
// Where it is written matters -- before the class-key's name it is the
// class's, in a member's declaration it is the member's -- and both
// spellings are here, with the type form alignas(T) beside the number.
//
// As everywhere in this directory the numbers are cl's, read out of
// /d1reportSingleClassLayout.

struct MemberAligned { alignas(16) int v; };
struct MemberAlignedAfter { char c; alignas(8) short s; };
struct MemberAlignedTwice { alignas(4) char a; alignas(4) char b; };
struct MemberAlignedType { char c; alignas(double) int i; };
struct MemberOverAligned { char c; alignas(32) int i; char d; };

struct alignas(16) ClassAligned { int v; };
struct alignas(32) ClassOverAligned { char c; };
struct alignas(8) ClassAlignedNoLess { long long l; };

// A member of an aligned class carries its alignment into the enclosing
// class, and a derived class carries its base's.
struct HoldsAligned { char c; ClassAligned a; };
struct DerivesAligned : ClassAligned { char c; };
struct HoldsMemberAligned { char c; MemberAligned m; };

// Both at once: the larger wins. The class-head's is the larger one here
// because the other way round is ill-formed -- §9.12.2/6 forbids an
// alignment-specifier weaker than what the entity would need without it,
// and a member's alignas(16) already needs 16. cl accepts that spelling
// without a word; clang refuses it.
struct alignas(16) BothAligned { alignas(8) int v; };

// An array member: the alignment is the element's, the size n of them.
struct ArrayOfAligned { ClassAligned a[2]; };
