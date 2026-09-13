// §11.4.9 [class.bit] -- bit-fields, whose allocation rule is the one every
// Windows layout question in vcc came out differently from the
// documentation on, and §11.5 [class.union], where every member is at zero.

// Fields that fit one storage unit share it.
struct ThreeInOne { unsigned a : 3; unsigned b : 5; unsigned c : 1; };
struct FillsAWord { unsigned a : 16; unsigned b : 16; };
struct OneBit { unsigned a : 1; };
struct WholeWord { unsigned a : 32; };

// A field that does not fit starts a new unit rather than straddling.
struct Straddles { unsigned a : 24; unsigned b : 16; };
struct JustOver { unsigned a : 31; unsigned b : 2; };

// A zero-width field ends the current unit, §11.4.9/2, which is the only
// thing it is for.
struct ZeroWidth { unsigned a : 3; unsigned : 0; unsigned b : 3; };

// A narrower declared type is a narrower storage unit.
struct CharBits { unsigned char a : 3; unsigned char b : 3; };
struct ShortBits { unsigned short a : 5; unsigned short b : 5; };
struct LongLongBits { unsigned long long a : 40; unsigned long long b : 20; };

// Whether a unit is shared across a change of declared type is exactly what
// the two ABIs disagree about, so it is asked rather than assumed.
struct MixedWidths { unsigned char a : 3; unsigned int b : 3; };
struct WideThenNarrow { unsigned int a : 3; unsigned char b : 3; };

// Bit-fields among ordinary members.
struct BitsThenInt { unsigned a : 3; int i; };
struct IntThenBits { int i; unsigned a : 3; };
struct CharBitsInt { char c; unsigned a : 3; int i; };
struct Surrounded { int before; unsigned a : 4; unsigned b : 4; int after; };

// §11.5 [class.union] -- every member at offset zero, and the union as big
// as its largest member rounded to its strictest alignment.
union OneMember { int i; };
union TwoSameSize { int i; float f; };
union DifferentSizes { char c; int i; double d; };
union WithArray { char c[9]; int i; };
struct Inner { char c; int i; };
union WithStruct { Inner s; long long l; };

// A union inside a struct, and a struct inside a union.
struct HoldsAUnion { char c; DifferentSizes u; char d; };
union HoldsAStruct { char c; BitsThenInt s; };
